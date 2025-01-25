package cron

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/basvanbeek/run"
	"github.com/basvanbeek/telemetry/scope"
)

var log = scope.Register("cront", "cron service")

type Service struct {
	ctx  context.Context
	done bool

	mtx  sync.Mutex
	jobs []*Reference
}

func (s *Service) Name() string {
	return "cron"
}

func (s *Service) AddJob(job Job, at time.Time, opts ...Option) (*Reference, error) {
	r := &Reference{
		svc:     s,
		nextRun: at,
		job:     job,
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
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
	log.Info("job added",
		"job", r.name,
		"next_run", r.nextRun.String(),
		"stop_after", r.stopAfter.String(),
		"max_run", r.maxRun,
	)

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
		if s.jobs[i] == r {
			s.jobs[i] = s.jobs[len(s.jobs)-1]
			s.jobs[len(s.jobs)-1] = nil
			s.jobs = s.jobs[:len(s.jobs)-1]
			r.cancel()
			log.Debug("job cancelled", "job", r.name)
			return
		}
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
		timer, cancel := context.WithTimeout(ctx, 1*time.Minute)
		now := time.Now()
		log.Debug("cron start iteration")
		// iterate over registered jobs
		s.mtx.Lock()
		for i := 0; i < len(s.jobs); i++ {
			// trigger jobs to see if they need to run
			if s.jobs[i].run() {
				log.Info("job triggered",
					"job", s.jobs[i].name,
					"next_run", s.jobs[i].nextRun.String(),
					"stop_after", s.jobs[i].stopAfter.String(),
					"run_count", s.jobs[i].runCount,
					"max_run", s.jobs[i].maxRun,
				)
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

var _ run.ServiceContext = &Service{}
