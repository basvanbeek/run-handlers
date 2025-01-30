package redis

import (
	"fmt"
	"net"

	"github.com/redis/go-redis/v9"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
)

// package flags.
const (
	defaultAddress = "localhost:6379"
	defaultDB      = 0

	Address  = "redis-address"
	DB       = "redis-db"
	UserName = "redis-username"
	Password = "redis-password"
)

// Config implements run.Config to allow configuration of a redis connection pool.
type Config struct {
	Prefix string

	Address  []string
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

// FlagSet implements run.Config.
func (c *Config) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("Redis options")

	flags.StringArrayVar(&c.Address, c.prefix(Address),
		[]string{defaultAddress}, "Redis server addresses")

	flags.IntVar(&c.DB, c.prefix(DB),
		defaultDB, "Redis database number")

	flags.StringVar(&c.UserName, c.prefix(UserName),
		"", "Redis username")

	flags.SensitiveStringVar(&c.Password, c.prefix(Password),
		"", "Redis password")

	return flags
}

// Validate implements run.Config.
func (c *Config) Validate() error {
	var mErr error

	for _, addr := range c.Address {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			mErr = multierror.Append(mErr,
				flag.NewValidationError(c.prefix(Address),
					fmt.Errorf("invalid address: %w", err)))
		}
	}

	return mErr
}

// PreRun implements run.PreRunner.
func (c *Config) PreRun() error {
	c.rdb = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    c.Address,
		Username: c.UserName,
		Password: c.Password,
	})

	return nil
}

// Pool returns the redis connection pool.
func (c *Config) Pool() redis.UniversalClient { return c.rdb }

var (
	_ run.Config    = (*Config)(nil)
	_ run.PreRunner = (*Config)(nil)
)
