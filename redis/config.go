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

// Package redis provides a run.Config implementation to configure a redis connection.
package redis

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
)

// package flags.
const (
	defaultAddress = "localhost:6379"
	defaultDB      = 0

	Hosts    = "redis-hosts"
	DB       = "redis-db"
	UserName = "redis-username"
	Password = "redis-password"
)

// Config implements run.Config to allow configuration of a redis connection pool.
type Config struct {
	Prefix string

	Hosts    []string
	DB       int
	UserName string
	Password string

	rdb redis.UniversalClient
}

func (c *Config) prefix(s string) string {
	if c.Prefix != "" {
		return c.Prefix + "-" + s
	}
	return s
}

// Name implements run.Unit.
func (c *Config) Name() string {
	return c.prefix("redis-pool")
}

func (c *Config) Initialize() {
	if c.Hosts == nil {
		c.Hosts = []string{defaultAddress}
	}
}

// FlagSet implements run.Config.
func (c *Config) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("Redis options")

	if hoststr := os.Getenv("REDIS_HOSTS"); hoststr != "" {
		c.Hosts = strings.Split(hoststr, ",")
	}
	if dbstr := os.Getenv("REDIS_DB"); dbstr != "" {
		if db, err := strconv.ParseInt(dbstr, 10, 64); err == nil {
			c.DB = int(db)
		}
	}
	if user := os.Getenv("REDIS_USERNAME"); user != "" {
		c.UserName = user
	}
	if pass := os.Getenv("REDIS_PASSWORD"); pass != "" {
		c.Password = pass
	}

	flags.StringArrayVar(&c.Hosts, c.prefix(Hosts),
		c.Hosts, "Redis server hosts")

	flags.IntVar(&c.DB, c.prefix(DB),
		c.DB, "Redis database number")

	flags.StringVar(&c.UserName, c.prefix(UserName),
		c.UserName, "Redis username")

	flags.SensitiveStringVar(&c.Password, c.prefix(Password),
		c.Password, "Redis password")

	return flags
}

// Validate implements run.Config.
func (c *Config) Validate() error {
	var mErr error

	for _, addr := range c.Hosts {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			mErr = multierror.Append(mErr,
				flag.NewValidationError(c.prefix(Hosts),
					fmt.Errorf("invalid host: %w", err)))
		}
	}

	return mErr
}

// PreRun implements run.PreRunner.
func (c *Config) PreRun() error {
	c.rdb = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    c.Hosts,
		Username: c.UserName,
		Password: c.Password,
	})

	return nil
}

// Pool returns the redis connection pool.
func (c *Config) Pool() redis.UniversalClient { return c.rdb }

var (
	_ run.Initializer = (*Config)(nil)
	_ run.Config      = (*Config)(nil)
	_ run.PreRunner   = (*Config)(nil)
)
