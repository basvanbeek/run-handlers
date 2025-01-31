package session

import (
	"errors"

	"github.com/basvanbeek/run"
	hndredis "github.com/basvanbeek/run-handlers/redis"
	"github.com/basvanbeek/telemetry/scope"
)

var logger = scope.Register("session", "session store")

type Config struct {
	Redis *hndredis.Config
}

func (c *Config) Name() string {
	return "sessions"
}

func (c *Config) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("Session Options")

	return flags
}

func (c *Config) Validate() error {
	var mErr error

	return mErr
}

func (c *Config) PreRun() error {
	if c.Redis == nil {
		return errors.New("missing redis run handler")
	}
	return nil
}

var (
	_ run.Config    = (*Config)(nil)
	_ run.PreRunner = (*Config)(nil)
)
