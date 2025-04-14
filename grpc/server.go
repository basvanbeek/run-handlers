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

package grpc //nolint:golint // see doc.go

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
)

// package flags.
const (
	ServerListenAddress  = "grpc-listen-address"
	MaxGRPCStreamMsgSize = "max-grpc-stream-msg-size"
)

// default configuration values.
const (
	defaultGRPCAddress          = ":9080"
	defaultMaxGRPCStreamMsgSize = 20 * 1024 * 1024 // 20MB
)

// Service implements a run.Group compatible gRPC server.
type Service struct {
	Address              string
	MaxGRPCStreamMsgSize int
	Options              []grpc.ServerOption

	i Interceptors
	*grpc.Server
	l net.Listener
	f []func(*grpc.Server)
}

// Name implements run.Unit.
func (s *Service) Name() string {
	return "grpc"
}

// FlagSet implements run.Config.
func (s *Service) FlagSet() *run.FlagSet {
	if s.Address == "" {
		s.Address = defaultGRPCAddress
	}

	if s.MaxGRPCStreamMsgSize == 0 {
		s.MaxGRPCStreamMsgSize = defaultMaxGRPCStreamMsgSize
	}

	flags := run.NewFlagSet("gRPC server options")

	flags.StringVarP(
		&s.Address,
		ServerListenAddress, "l",
		s.Address,
		`gRPC server listen address, e.g. ":9080" or "localhost:9000"`)

	flags.IntVar(
		&s.MaxGRPCStreamMsgSize,
		MaxGRPCStreamMsgSize,
		defaultMaxGRPCStreamMsgSize,
		"Max size in bytes of the message sent or received via the stream. Default is 20MB")

	return flags
}

// Validate implements run.Config.
func (s *Service) Validate() error {
	var mErr error

	if s.Address != "" {
		if _, _, err := net.SplitHostPort(s.Address); err != nil {
			mErr = multierror.Append(mErr,
				flag.NewValidationError(ServerListenAddress, err))
		}
	} else {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(ServerListenAddress, flag.ErrRequired))
	}

	if s.MaxGRPCStreamMsgSize < 4*1024*1024 {
		mErr = multierror.Append(mErr,
			flag.NewValidationError(MaxGRPCStreamMsgSize, flag.ValidationError("must be at least 4MB")))
	}

	return mErr
}

// Serve implements run.Service.
func (s *Service) Serve() error {
	s.Options = append([]grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.MaxGRPCStreamMsgSize),
		grpc.MaxSendMsgSize(s.MaxGRPCStreamMsgSize),
	}, s.Options...)

	s.Server = grpc.NewServer(s.Options...)

	// now that we have the internal grpc.Server object, run all callbacks
	// provided with AttachToServer to register the gRPC services to handle.
	for _, f := range s.f {
		f(s.Server)
	}

	reflection.Register(s.Server)

	// listen and serve time
	var err error
	s.l, err = net.Listen("tcp", s.Address)
	if err != nil {
		return err
	}

	return s.Server.Serve(s.l)
}

// GracefulStop implements run.Service.
func (s *Service) GracefulStop() {
	if s.l != nil {
		s.Stop()
		_ = s.l.Close()
	}
}

// Attach allows one to register gRPC services to this server. Once the actual
// gRPC server is created, the registration function provided to this call will
// be executed. (during the run.Group Serve stage).
func (s *Service) Attach(fn func(*grpc.Server)) {
	s.f = append(s.f, fn)
}

// Interceptors returns the Interceptors handler for this gRPC Service.
func (s *Service) Interceptors() *Interceptors {
	return &s.i
}

// GetGrpcAddress returns the grpc address assigned to the server instance.
// If the address is not configured, an error is returned.
func (s *Service) GetGrpcAddress() (string, error) {
	if s.Address == "" {
		return "", errors.New("s.Address is not set")
	}
	// we need an address we can use in a client. the listener address might not be directly suitable
	host, port, err := net.SplitHostPort(s.Address)
	if err != nil {
		return "", fmt.Errorf("s.GrpcAddress is invalid: %w", err)
	}
	switch strings.ToLower(host) {
	case "0.0.0.0", "", "localhost", "[::1]", "[::1%lo0]":
		host = "localhost"
	}

	return net.JoinHostPort(host, port), nil
}

var (
	_ run.Config  = (*Service)(nil)
	_ run.Service = (*Service)(nil)
)
