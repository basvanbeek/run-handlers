package cron_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/basvanbeek/run-handlers/cron"
)

var (
	mtxService = &sync.Mutex{}
	svc        *cron.Service
	cancel     context.CancelFunc
)

func startService(schedulerInterval time.Duration) (*cron.Service, error) {
	mtxService.Lock()
	defer mtxService.Unlock()

	if svc != nil {
		return svc, nil
	}

	s := &cron.Service{SchedulerInterval: schedulerInterval}
	err := s.Validate()
	if err != nil {
		return nil, err
	}
	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		err = s.ServeContext(ctx)
		cancel()
	}()

	return s, nil
}

func stopService() {
	mtxService.Lock()
	defer mtxService.Unlock()

	if svc != nil {
		cancel()
	}
}

func TestService_TestJobs(t *testing.T) {
	s, err := startService(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer stopService()

	now := time.Now()
	var countA, countB, countC, countD int
	if _, err = s.AddJob(
		func(context.Context) error {
			countA++
			return nil
		},
		now,
		cron.WithInterval(time.Second),
		cron.WithName("jobA"),
	); err != nil {
		t.Error("expected Job A to be created", err)
	}

	if _, err = s.AddJob(
		func(context.Context) error {
			countB++
			return nil
		},
		now,
		cron.WithInterval(10*time.Second),
		cron.WithName("jobB"),
	); err != nil {
		t.Error("expected Job B to be created", err)
	}
	if _, err = s.AddJob(
		func(context.Context) error {
			countC++
			time.Sleep(3 * time.Second)
			return nil
		},
		now,
		cron.WithInterval(1*time.Second),
		cron.WithIntervalMode(cron.IntervalModeBetweenRuns),
		cron.WithName("jobC"),
	); err != nil {
		t.Error("expected JobC to be created", err)
	}
	if _, err = s.AddJob(
		func(context.Context) error {
			countD++
			time.Sleep(3 * time.Second)
			return nil
		},
		now,
		cron.WithInterval(1*time.Second),
		cron.WithIntervalMode(cron.IntervalModeOnTick),
		cron.WithName("jobD"),
	); err != nil {
		t.Error("expected JobD to be created", err)
	}

	// let's run for 3 seconds
	time.Sleep(3 * time.Second)

	if countA < 2 || countA > 4 {
		t.Errorf("expected Job A count to be around 3, got %d", countA)
	}
	if countB != 1 {
		t.Errorf("expected Job B count to be 1, got %d", countB)
	}
	if countC != 1 {
		t.Errorf("expected Job C count to be 1, got %d", countA)
	}
	if countD < 2 || countD > 4 {
		t.Errorf("expected Job D count to be around 3, got %d", countB)
	}
}
