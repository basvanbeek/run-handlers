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

package session

import (
	"context"
	"encoding/base32"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"

	hndredis "github.com/basvanbeek/run-handlers/redis"
)

// NewRedisStore returns a new gorilla sessions.Store compatible Handler backed
// by Redis. Handler extends the gorilla sessions.Store interface with a
// GetBySessionID method.
func NewRedisStore(redis *hndredis.Config, opts ...Option) (Handler, error) {
	s := &store{
		redis:         redis,
		defaultMaxAge: 48 * 60 * 60,
		options: &sessions.Options{
			Path:        "/",
			Domain:      "",
			MaxAge:      0,
			Secure:      true,
			HttpOnly:    true,
			Partitioned: true,
			SameSite:    http.SameSiteStrictMode,
		},
		maxLength:  4096,
		keyPrefix:  "session_",
		serializer: JSONSerializer{},
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type store struct {
	redis         *hndredis.Config
	codecs        []securecookie.Codec
	options       *sessions.Options
	defaultMaxAge int
	maxLength     int
	keyPrefix     string
	serializer    Serializer
}

// GetBySessionID returns a session by its session ID and name.
// Name is used to create a new session object with the provided name, so it
// can be stored in the session registry.
// GetBySessionID implements the Handler interface.
func (s *store) GetBySessionID(name, sessionID string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	options := *s.options
	session.Options = &options
	session.ID = sessionID
	session.IsNew = false

	data, err := s.redis.Pool().
		Get(context.Background(), s.keyPrefix+session.ID).Bytes()
	if err != nil {
		return nil, err
	}
	if err = s.serializer.Deserialize(data, session); err != nil {
		return nil, err
	}
	return session, nil
}

// Get implements the gorilla sessions.Store interface.
func (s *store) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New implements the gorilla sessions.Store interface.
func (s *store) New(r *http.Request, name string) (*sessions.Session, error) {
	var (
		err  error
		data []byte
	)
	session := sessions.NewSession(s, name)
	options := *s.options
	session.Options = &options
	session.IsNew = true
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.codecs...)
		if err != nil {
			return session, nil
		}

		data, err = s.redis.Pool().
			Get(r.Context(), s.keyPrefix+session.ID).Bytes()
		if err != nil {
			return session, err
		}
		if err = s.serializer.Deserialize(data, session); err != nil {
			return session, err
		}
		session.IsNew = false
		return session, nil
	}
	return session, err
}

// Save implements the gorilla sessions.Store interface.
func (s *store) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	var encoded string

	if session.Options.MaxAge < 0 {
		// session is marked for deletion
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return s.redis.Pool().Del(r.Context(), s.keyPrefix+session.ID).Err()
	}
	if session.ID == "" {
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32),
			),
			"=",
		)
	}
	data, err := s.serializer.Serialize(session)
	if err != nil {
		return err
	}
	if s.maxLength != 0 && len(data) > s.maxLength {
		return errors.New("session data too long")
	}
	age := session.Options.MaxAge
	if age == 0 {
		age = s.defaultMaxAge
	}
	err = s.redis.Pool().SetEx(r.Context(),
		s.keyPrefix+session.ID, data, time.Duration(age)*time.Second).Err()
	if err != nil {
		return err
	}
	encoded, err = securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}
