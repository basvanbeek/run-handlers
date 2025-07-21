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

// Package cron provides a simple job scheduler for running jobs at.
package cron

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/basvanbeek/run"
	"github.com/basvanbeek/telemetry/scope"
)

var (
	log       = scope.Register("cron", "cron service")
	mtx       sync.Mutex
	scheduler *Service
)

const (
	flagSchedulerInterval = "scheduler-interval"

	defaultSchedulerInterval = 1 * time.Minute
)

type Service struct {
	SchedulerInterval time.Duration

	ctx  context.Context
	done bool
	mtx  sync.Mutex
	jobs []*Reference
}

func (s *Service) Initialize() {
	mtx.Lock()
	defer mtx.Unlock()
	if scheduler != nil {
		return
	}
	scheduler = s
}

func (s *Service) FlagSet() *run.FlagSet {
	fs := run.NewFlagSet(s.Name())

	fs.DurationVar(&s.SchedulerInterval, flagSchedulerInterval,
		defaultSchedulerInterval, "interval between scheduler runs")

	return fs
}

func (s *Service) Validate() error {
	if s.SchedulerInterval < time.Second {
		return errors.New("scheduler interval needs to be at least one second")
	}

	return nil
}

func (s *Service) Name() string {
	return "cron"
}

func (s *Service) AddJob(job Job, at time.Time, opts ...Option) (*Reference, error) {
	r := &Reference{
		svc:      s,
		job:      job,
		interval: s.SchedulerInterval,
		mode:     IntervalModeOnTick,
	}
	r.nextRun.Store(&at)

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	if r.interval < s.SchedulerInterval {
		return nil, fmt.Errorf("%w (%s)", ErrIntervalTooShort,
			s.SchedulerInterval.String())
	}

	if r.name == "" {
		r.name = "anonymous"
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.done {
		return nil, errors.New("service has already shut down")
	}
	if s.ctx != nil {
		r.ctx, r.cancel = context.WithCancel(s.ctx)
	}
	log.Info("job added", r.logDetails()...)

	s.jobs = append(s.jobs, r)

	return r, nil
}

func (s *Service) cancelJob(r *Reference) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.done {
		return
	}
	for i := 0; i < len(s.jobs); i++ {
		if s.jobs[i] != r {
			continue
		}
		s.jobs[i] = s.jobs[len(s.jobs)-1]
		s.jobs[len(s.jobs)-1] = nil
		s.jobs = s.jobs[:len(s.jobs)-1]
		r.cancel()
		log.Debug("job canceled", "job", r.name)
		return
	}
}

func (s *Service) ServeContext(ctx context.Context) error {
	s.mtx.Lock()
	s.ctx = ctx
	for i := 0; i < len(s.jobs); i++ {
		s.jobs[i].ctx, s.jobs[i].cancel = context.WithCancel(ctx)
	}
	s.mtx.Unlock()
	for {
		// set timer so we don't get back here within that time period.
		timer, cancel := context.WithTimeout(ctx, s.SchedulerInterval)
		now := time.Now()
		log.Debug("cron start iteration")
		// iterate over registered jobs
		s.mtx.Lock()
		for i := 0; i < len(s.jobs); i++ {
			// trigger jobs to see if they need to run
			if s.jobs[i].run() {
				log.Info("job triggered", s.jobs[i].logDetails()...)
			}
		}
		s.mtx.Unlock()
		log.Debug("cron end iteration", "duration", time.Since(now))

		// wait until application context is canceled or trigger timer is done.
		select {
		case <-ctx.Done():
			log.Info("cron service shutting down")
			// cancel our timer
			cancel()
			// remove all jobs
			s.mtx.Lock()
			s.done = true
			for i := 0; i < len(s.jobs); i++ {
				// cancels job if active
				s.jobs[i].cancel()
			}
			s.jobs = nil
			s.mtx.Unlock()
			// we can now safely exit
			return nil
		case <-timer.Done():
			// trigger when timer is done
			continue
		}
	}
}

func AddJob(job Job, at time.Time, opts ...Option) (*Reference, error) {
	mtx.Lock()
	s := scheduler
	mtx.Unlock()
	if s == nil {
		return nil, errors.New("cron service not initialized")
	}
	return s.AddJob(job, at, opts...)
}

var (
	_ run.Initializer    = (*Service)(nil)
	_ run.Config         = (*Service)(nil)
	_ run.ServiceContext = (*Service)(nil)
)
