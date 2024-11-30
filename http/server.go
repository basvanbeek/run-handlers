// Copyright (c) Bas van Beek 2024.
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

// Package http holds a run.Group compatible HTTP Server.
package http

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
)

// package flags.
const (
	flagListenAddress = "http-listen-address"
	flagSecureHeaders = "secure-headers"
)

const (
	defaultHTTPAddress = ":80"
)

// Service implements a run.Group compatible HTTP Server.
type Service struct {
	Address       string
	SecureHeaders bool

	*http.Server
	l net.Listener
}

// Name implements run.Unit.
func (s *Service) Name() string {
	return "http"
}

// FlagSet implements run.Config.
func (s *Service) FlagSet() *run.FlagSet {
	if s.Address == "" {
		s.Address = defaultHTTPAddress
	}
	if s.Server == nil {
		s.Server = &http.Server{
			ReadHeaderTimeout: 60 * time.Second,
			ReadTimeout:       120 * time.Second,
			WriteTimeout:      120 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
	}
	flags := run.NewFlagSet("HTTP server options")

	flags.StringVarP(
		&s.Address,
		flagListenAddress, "a",
		s.Address,
		`HTTP server listen address, e.g. ":443" or "localhost:80"`)

	flags.BoolVar(
		&s.SecureHeaders,
		flagSecureHeaders,
		false,
		"Enable HTTP header security. Only do this in production as we're enabling HTTP-STS!",
	)

	return flags
}

// Validate implements run.Config.
func (s *Service) Validate() error {
	var mErr error

	if s.Address != "" {
		if _, _, err := net.SplitHostPort(s.Address); err != nil {
			mErr = multierror.Append(mErr,
				flag.NewValidationError(flagListenAddress, err))
		}
	} else {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(flagListenAddress, flag.ErrRequired))
	}

	return mErr
}

// Serve implements run.Service.
func (s *Service) Serve() error {
	// listen and serve time
	var err error
	s.l, err = net.Listen("tcp", s.Address)
	if err != nil {
		return err
	}
	if s.TLSConfig != nil {
		return s.Server.ServeTLS(s.l, "", "")
	}
	if s.SecureHeaders {
		s.Server.Handler = SecurityHandler(s.Server.Handler)
	}
	return s.Server.Serve(s.l)
}

// GracefulStop implements run.Service.
func (s *Service) GracefulStop() {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	if s.Server != nil {
		_ = s.Server.Shutdown(ctx)
	}
	if s.l != nil {
		_ = s.l.Close()
	}
}

var (
	_ run.Config  = (*Service)(nil)
	_ run.Service = (*Service)(nil)
)
