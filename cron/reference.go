package cron

import (
	"context"
	"time"
)

// Job holds a function which can be scheduled to run through the cron.Service.
type Job func(ctx context.Context) error

// Reference holds the pointer to a scheduled Job. It can be used to cancel
// a job when no longer needed.
type Reference struct {
	svc       *Service
	name      string
	lastRun   time.Time
	nextRun   time.Time
	stopAfter time.Time
	interval  time.Duration
	runCount  int
	maxRun    int
	job       Job
	ctx       context.Context
	cancel    context.CancelFunc
}

func (r *Reference) run() bool {
	// see if we need to run the job
	if r.maxRun > 0 && r.runCount >= r.maxRun {
		// we already ran as often as needed
		go r.svc.cancelJob(r) // cancel the job in goroutine to avoid deadlock
		return false
	}
	now := time.Now()
	if r.nextRun.After(now) {
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
		r.nextRun = r.lastRun.Add(r.interval)
	}
	go func() {
		if err := r.job(r.ctx); err != nil {
			log.Error("job failed", err, "job", r.name)
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
		ss = append(ss, "next_run", r.nextRun.Format("2006-01-02 15:04:05"))
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
