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

// Package dbpool provides a database connection pool for use with the run package.
package dbpool

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
)

// package flags.
const (
	defaultMaxOpenConnections = 50
	defaultMaxIdleConnections = 0
	defaultMaxConnLifetime    = 5 * time.Second
	defaultMaxConnIdleTime    = 1 * time.Second

	DSN                = "dsn"
	ReadOnlyDSN        = "dsn-read-only"
	MaxIdleConnections = "max-idle-connections"
	MaxOpenConnections = "max-open-connections"
	MaxConnLifetime    = "max-connections-lifetime"
	MaxConnIdleTime    = "max-connections-idletime"
)

// Config implements run.Config to allow configuration of a db connection pool.
type Config struct {
	Prefix             string
	DSN                string
	DSNRead            string
	MaxIdleConnections int32
	MaxOpenConnections int32
	MaxConnLifetime    time.Duration
	MaxConnIdleTime    time.Duration

	pool         *sql.DB
	readOnlyPool *sql.DB
}

func (c *Config) prefix(s string) string {
	if c.Prefix != "" {
		return c.Prefix + "-" + s
	}
	return s
}

// Name implements run.Unit.
func (c *Config) Name() string {
	return c.prefix("db-pool")
}

// FlagSet implements run.Config.
func (c *Config) FlagSet() *run.FlagSet {
	if envDSN := os.Getenv("DSN"); envDSN != "" {
		c.DSN = envDSN
	}
	if envReadOnlyDSN := os.Getenv("DSN_READ_ONLY"); envReadOnlyDSN != "" {
		c.DSNRead = envReadOnlyDSN
	}
	if c.DSNRead == "" {
		c.DSNRead = c.DSN
	}

	if c.MaxOpenConnections == 0 {
		c.MaxOpenConnections = defaultMaxOpenConnections
	}
	if c.MaxIdleConnections == 0 {
		c.MaxIdleConnections = defaultMaxIdleConnections
	}
	if c.MaxConnLifetime == 0 {
		c.MaxConnLifetime = defaultMaxConnLifetime
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = defaultMaxConnIdleTime
	}

	flags := run.NewFlagSet("Database options")

	flags.SensitiveStringVar(&c.DSN, c.prefix(DSN),
		c.DSN, "data source name")

	flags.SensitiveStringVar(&c.DSNRead, c.prefix(ReadOnlyDSN),
		c.DSNRead, "read-only data source name")

	flags.Int32Var(&c.MaxIdleConnections, c.prefix(MaxIdleConnections),
		c.MaxIdleConnections, "max. idle connections")

	flags.Int32Var(&c.MaxOpenConnections, c.prefix(MaxOpenConnections),
		c.MaxOpenConnections, "max. open connections")

	flags.DurationVar(&c.MaxConnLifetime, c.prefix(MaxConnLifetime),
		c.MaxConnLifetime, "max. connection lifetime")

	flags.DurationVar(&c.MaxConnIdleTime, c.prefix(MaxConnIdleTime),
		c.MaxConnIdleTime, "max. connection idle time")

	return flags
}

// Validate implements run.Config.
func (c *Config) Validate() error {
	var mErr error

	if c.DSN == "" {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(c.prefix(DSN), flag.ErrRequired))
	}

	if c.DSNRead == "" {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(c.prefix(ReadOnlyDSN), flag.ErrRequired))
	}

	return mErr
}

func (c *Config) createPool(dsn string) (pool *sql.DB, err error) {
	pool, err = sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open failed: %w", err)
	}

	pool.SetConnMaxLifetime(c.MaxConnLifetime)
	pool.SetConnMaxIdleTime(c.MaxConnIdleTime)
	pool.SetMaxOpenConns(int(c.MaxOpenConnections))
	pool.SetMaxIdleConns(int(c.MaxIdleConnections))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = pool.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	return pool, nil
}

// PreRun implements run.PreRunner.
func (c *Config) PreRun() error {
	var (
		mErr error
		wg   sync.WaitGroup
		errc = make(chan error, 2)
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		if c.pool, err = c.createPool(c.DSN); err != nil {
			errc <- err
		}
	}()

	if c.DSN != c.DSNRead {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			if c.readOnlyPool, err = c.createPool(c.DSNRead); err != nil {
				errc <- err
			}
		}()
	} else {
		c.readOnlyPool = c.pool
	}

	wg.Wait()
	close(errc)

	for err := range errc {
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	return mErr
}

// Pool returns the established database connection pool handler.
func (c *Config) Pool() *sql.DB {
	return c.pool
}

// ReadOnlyPool returns the established read-only database connection pool
// handler. If no read-only connection pool is established, the default pool
// will be returned.
func (c *Config) ReadOnlyPool() *sql.DB {
	return c.readOnlyPool
}

var (
	_ run.Config    = (*Config)(nil)
	_ run.PreRunner = (*Config)(nil)
)
