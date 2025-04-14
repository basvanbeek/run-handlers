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

// Package session provides a run.Config implementation to configure a session store.
package session

import (
	"encoding/base32"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
	"github.com/basvanbeek/telemetry/scope"

	"github.com/basvanbeek/run-handlers/redis"
)

var logger = scope.Register("session", "session store")

const (
	flagSessionMaxAge         = "session-max-age"
	flagSessionMaxIdle        = "session-max-idle"
	flagSessionSecretKey      = "session-secret-key"
	flagSessionInsecureCookie = "session-insecure-cookie"
	flagSessionPrefix         = "session-prefix"
	flagSessionMaxLength      = "session-max-length"

	defaultSessionMaxIdle = 36 * time.Hour
	defaultSessionPrefix  = "session"
	defaultSessionLength  = 4096
)

type Handler interface {
	sessions.Store
	GetBySessionID(name, sessionID string) (*sessions.Session, error)
}

type Config struct {
	Redis          *redis.Config
	SecretKeys     string
	MaxAge         int
	MaxIdle        time.Duration
	InsecureCookie bool
	NotPartitioned bool
	Prefix         string
	MaxLength      int

	secretKeys [][]byte
	store      Handler
}

func (c *Config) Initialize() {
	if s := os.Getenv("SESSION_SECRET_KEYS"); s != "" {
		c.SecretKeys = s
	} else {
		c.SecretKeys = base32.StdEncoding.WithPadding(base32.NoPadding).
			EncodeToString(securecookie.GenerateRandomKey(64))
	}
	if m := os.Getenv("SESSION_MAX_AGE"); m != "" {
		if im, err := strconv.ParseInt(m, 10, 64); err == nil {
			c.MaxAge = int(im)
		}
	}
	c.MaxIdle = defaultSessionMaxIdle
	if i := os.Getenv("SESSION_MAX_IDLE"); i != "" {
		if id, err := time.ParseDuration(i); err == nil {
			c.MaxIdle = id
		}
	}
	c.Prefix = defaultSessionPrefix
	if p := os.Getenv("SESSION_PREFIX"); p != "" {
		c.Prefix = p
	}
	c.MaxLength = defaultSessionLength
	if l := os.Getenv("SESSION_MAX_LENGTH"); l != "" {
		if il, err := strconv.ParseInt(l, 10, 64); err == nil {
			c.MaxLength = int(il)
		}
	}
}

func (c *Config) Name() string {
	return "sessions"
}

func (c *Config) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("Session Options")

	flags.IntVar(&c.MaxAge, flagSessionMaxAge, c.MaxAge,
		"Session cookie max age in seconds. (0 for session-only cookies)")

	flags.DurationVar(&c.MaxIdle, flagSessionMaxIdle, c.MaxIdle,
		"Session max request idle time before session is invalidated")

	flags.SensitiveStringVar(&c.SecretKeys, flagSessionSecretKey, c.SecretKeys,
		"Secret keys used to sign session cookies (comma separated)")

	flags.BoolVar(&c.InsecureCookie, flagSessionInsecureCookie, c.InsecureCookie,
		"Use insecure cookies (no HTTPS) for development purposes")

	flags.BoolVar(&c.NotPartitioned, "session-not-partitioned", c.NotPartitioned,
		"Disable partitioning of session cookies")

	flags.StringVar(&c.Prefix, flagSessionPrefix, c.Prefix,
		"Session key prefix")

	flags.IntVar(&c.MaxLength, flagSessionMaxLength, c.MaxLength,
		"Maximum length of session data")

	return flags
}

func (c *Config) Validate() error {
	var mErr error

	if c.MaxIdle < 1*time.Minute {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(flagSessionMaxIdle,
				errors.New("max idle time must be at least 1 minute")))
	}

	if c.SecretKeys == "" {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(flagSessionSecretKey,
				errors.New("secret keys can't be empty")))
	}

	sk := strings.Split(c.SecretKeys, ",")
	for _, k := range sk {
		k = strings.Trim(k, "\r\n\t ")
		if k != "" {
			c.secretKeys = append(c.secretKeys, []byte(k))
		}
	}
	if len(c.secretKeys) == 0 {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(flagSessionSecretKey,
				errors.New("secret keys can't be empty")))
	}
	return mErr
}

func (c *Config) PreRun() (err error) {
	if c.Redis == nil {
		return errors.New("missing redis run handler")

	}
	opts := []Option{
		WithKeyPairs(c.secretKeys...),
		WithMaxLength(c.MaxLength),
		WithKeyPrefix(c.Prefix),
		WithSerializer(GobSerializer{}),
		WithSessionOptions(&sessions.Options{
			Path:        "/",
			MaxAge:      c.MaxAge,
			Secure:      !c.InsecureCookie,
			HttpOnly:    true,
			Partitioned: !c.NotPartitioned,
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
