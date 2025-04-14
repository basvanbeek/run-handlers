// Copyright (c) Bas van Beek 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cron

import (
	"context"
	"sync/atomic"
	"time"
)

// maxTime is the maximum time that can be represented by a time.Time.
var maxTime = time.Unix(1<<63-62135596801, 999999999)

// Job holds a function which can be scheduled to run through the cron.Service.
type Job func(ctx context.Context) error

// Reference holds the pointer to a scheduled Job. It can be used to cancel
// a job when no longer needed.
type Reference struct {
	svc       *Service
	name      string
	lastRun   time.Time
	nextRun   atomic.Pointer[time.Time]
	stopAfter time.Time
	interval  time.Duration
	mode      IntervalMode
	runCount  int
	maxRun    int
	job       Job
	ctx       context.Context
	cancel    context.CancelFunc
}

type IntervalMode int

const (
	IntervalModeOnTick IntervalMode = iota
	IntervalModeBetweenRuns
)

func (r *Reference) run() bool {
	// see if we need to run the job
	if r.maxRun > 0 && r.runCount >= r.maxRun {
		// we already ran as often as needed
		go r.svc.cancelJob(r) // cancel the job in goroutine to avoid deadlock
		return false
	}
	now := time.Now()
	if r.nextRun.Load().After(now) {
		// next run is still in the future
		return false
	}
	if !r.stopAfter.IsZero() && now.After(r.stopAfter) {
		// we're not allowed to run anymore
		go r.svc.cancelJob(r) // cancel the job in goroutine to avoid deadlock
		return false
	}
	// check if we already need to exit
	if err := r.ctx.Err(); err != nil {
		// job has been canceled
		return false
	}
	// time to run the job
	r.runCount++
	r.lastRun = now
	if r.interval > 0 {
		if r.mode == IntervalModeOnTick {
			nextRun := r.lastRun.Add(r.interval)
			r.nextRun.Store(&nextRun)
		} else {
			// we need to move nextRun sufficiently beyond the possible run time
			// of this job to avoid running it multiple times concurrently
			r.nextRun.Store(&maxTime)
		}
	}
	go func() {
		if err := r.job(r.ctx); err != nil {
			log.Error("job failed", err, "job", r.name)
		}
		if r.interval > 0 && r.mode == IntervalModeBetweenRuns {
			nextRun := time.Now().Add(r.interval)
			r.nextRun.Store(&nextRun)
		}
	}()
	return true
}

func (r *Reference) Cancel() {
	r.svc.cancelJob(r)
}

func (r *Reference) logDetails() []any {
	ss := []any{
		"job", r.name,
	}
	if r.maxRun <= 0 || r.runCount < r.maxRun {
		ss = append(ss, "next_run", r.nextRun.Load().Format("2006-01-02 15:04:05"))
	}
	if !r.stopAfter.IsZero() {
		ss = append(ss, "stop_after", r.stopAfter.Format("2006-01-02 15:04:05"))
	}
	ss = append(ss, "run_count", r.runCount)
	if r.maxRun > 0 {
		ss = append(ss, "max_run", r.maxRun)
	}
	return ss
}
