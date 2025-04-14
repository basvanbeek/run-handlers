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
	"context"

	"google.golang.org/grpc/stats"
)

// StatsHandler provides the ability to chain multiple grpc/stats.Handler into
// a single one to be consumed by a gRPC service.
type StatsHandler struct {
	h []stats.Handler
}

// HandleConn implements stats.Handler.
func (s *StatsHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
	for i := len(s.h) - 1; i >= 0; i-- {
		s.h[i].HandleConn(ctx, cs)
	}
}

// TagConn implements stats.Handler.
func (s *StatsHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	for i := len(s.h) - 1; i >= 0; i-- {
		ctx = s.h[i].TagConn(ctx, cti)
	}
	return ctx
}

// HandleRPC implements stats.Handler.
func (s *StatsHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	for i := len(s.h) - 1; i >= 0; i-- {
		s.h[i].HandleRPC(ctx, rs)
	}
}

// TagRPC implements stats.Handler.
func (s *StatsHandler) TagRPC(ctx context.Context, rti *stats.RPCTagInfo) context.Context {
	for i := len(s.h) - 1; i >= 0; i-- {
		ctx = s.h[i].TagRPC(ctx, rti)
	}
	return ctx
}

// NewStatsHandler chains multiple stats.Handler implementations into one.
func NewStatsHandler(handlers ...stats.Handler) stats.Handler {
	return &StatsHandler{h: handlers}
}
