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

package grpc //nolint:golint // see doc.go

import (
	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

// Interceptors holds collections of gRPC server and client interceptors which
// can be added to and allows them to be chained in a gRPC server or client.
type Interceptors struct {
	sh []stats.Handler
	us []grpc.UnaryServerInterceptor
	uc []grpc.UnaryClientInterceptor
	ss []grpc.StreamServerInterceptor
	sc []grpc.StreamClientInterceptor
	so []grpc.ServerOption
}

// AddStatsHandler allows one or more stats.Handlers to be registered.
func (i *Interceptors) AddStatsHandler(handlers ...stats.Handler) {
	i.sh = append(i.sh, handlers...)
}

// AddUnaryServer allows one or more UnaryServerInterceptors to be registered.
func (i *Interceptors) AddUnaryServer(us ...grpc.UnaryServerInterceptor) {
	i.us = append(i.us, us...)
}

// AddUnaryClient allows one or more UnaryClientInterceptors to be registered.
func (i *Interceptors) AddUnaryClient(uc ...grpc.UnaryClientInterceptor) {
	i.uc = append(i.uc, uc...)
}

// AddStreamServer allows one or more StreamServerInterceptor to be registered.
func (i *Interceptors) AddStreamServer(ss ...grpc.StreamServerInterceptor) {
	i.ss = append(i.ss, ss...)
}

// AddStreamClient allows one or more StreamClientInterceptor to be registered.
func (i *Interceptors) AddStreamClient(sc ...grpc.StreamClientInterceptor) {
	i.sc = append(i.sc, sc...)
}

// AddServerOption allows to add custom server options to a gRPC server.
func (i *Interceptors) AddServerOption(so ...grpc.ServerOption) {
	i.so = append(i.so, so...)
}

// GetServerOptions returns an array of grpc.ServerOptions composed of the
// registered chained ServerInterceptors.
func (i *Interceptors) GetServerOptions() []grpc.ServerOption {
	var so []grpc.ServerOption

	if len(i.sh) > 0 {
		so = append(so, grpc.StatsHandler(
			NewStatsHandler(i.sh...),
		))
	}
	if len(i.us) > 0 {
		so = append(so, grpc.UnaryInterceptor(
			grpcmiddleware.ChainUnaryServer(i.us...),
		))
	}
	if len(i.ss) > 0 {
		so = append(so, grpc.StreamInterceptor(
			grpcmiddleware.ChainStreamServer(i.ss...),
		))
	}
	if len(i.so) > 0 {
		so = append(so, i.so...)
	}

	return so
}

// GetDialOptions returns an array of grpc.DialOptions composed of the
// registered chained ClientInterceptors.
func (i *Interceptors) GetDialOptions() []grpc.DialOption {
	var do []grpc.DialOption

	if len(i.sh) > 0 {
		do = append(do, grpc.WithStatsHandler(
			NewStatsHandler(i.sh...),
		))
	}
	if len(i.uc) > 0 {
		do = append(do, grpc.WithUnaryInterceptor(
			grpcmiddleware.ChainUnaryClient(i.uc...),
		))
	}

	if len(i.sc) > 0 {
		do = append(do, grpc.WithStreamInterceptor(
			grpcmiddleware.ChainStreamClient(i.sc...),
		))
	}

	return do
}
