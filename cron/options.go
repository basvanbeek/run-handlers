package cron

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrIntervalTooShort = errors.New("interval needs to be at least 1 minute")
)

type Option func(r *Reference) error

// WithMaxRun sets the maximum number of times the job will be run.
func WithMaxRun(maxRun int) Option {
	return func(r *Reference) error {
		r.maxRun = maxRun
		return nil
	}
}

// WithInterval sets the interval between runs of the job.
func WithInterval(interval time.Duration) Option {
	return func(r *Reference) error {
		if interval < time.Minute {
			return ErrIntervalTooShort
		}
		r.interval = interval
		return nil
	}
}

// WithStopAfter sets the time after which the job will no longer be run.
func WithStopAfter(stopAfter time.Time) Option {
	return func(r *Reference) error {
		if stopAfter.Before(time.Now().Add(1 * time.Hour)) {
			return errors.New("stopAfter needs to be at least one hour into future")
		}
		r.stopAfter = stopAfter
		return nil
	}
}

// WithName sets the name of the job.
func WithName(name string) Option {
	return func(r *Reference) error {
		if strings.Trim(name, " \t\r\n") == "" {
			return errors.New("name cannot be empty")
		}
		r.name = name
		return nil
	}
}
