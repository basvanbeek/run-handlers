package session

import (
	"errors"
	"net/http"

	"github.com/basvanbeek/run"
	hndredis "github.com/basvanbeek/run-handlers/redis"
	"github.com/basvanbeek/telemetry/scope"
	"github.com/gorilla/sessions"
)

var logger = scope.Register("session", "session store")

type Handler interface {
	sessions.Store
	GetBySessionID(name, sessionID string) (*sessions.Session, error)
}

type Config struct {
	Redis *hndredis.Config
	store Handler
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

func (c *Config) PreRun() (err error) {
	if c.Redis == nil {
		return errors.New("missing redis run handler")
	}
	opts := []Option{
		WithKeyPairs([]byte("secret-key")),
		WithMaxLength(4096),
		WithKeyPrefix("session"),
		WithSerializer(GobSerializer{}),
		WithSessionOptions(&sessions.Options{
			Path:        "/",
			MaxAge:      60 * 5,
			Secure:      true,
			HttpOnly:    true,
			Partitioned: true,
			SameSite:    http.SameSiteStrictMode,
		}),
	}
	c.store, err = NewRedisStore(c.Redis, opts...)
	return err
}

func (c *Config) Handler() Handler {
	return c.store
}

var (
	_ run.Config    = (*Config)(nil)
	_ run.PreRunner = (*Config)(nil)
)
