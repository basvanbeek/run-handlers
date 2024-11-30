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

// Package grpcvalidator holds a protoc-gen-validate gRPC server interceptor middleware.
package grpcvalidator

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validator interface {
	Validate() error
}

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor to validate
// the incoming request payload prior to handing over to the business logic.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if v, ok := req.(validator); ok {
			if err := v.Validate(); err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a grpc.StreamServerInterceptor to validate
// the incoming request payloads prior to handing over to the business logic.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &wrapper{stream})
	}
}

type wrapper struct {
	grpc.ServerStream
}

func (w *wrapper) RecvMsg(m interface{}) error {
	if err := w.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if v, ok := m.(validator); ok {
		if err := v.Validate(); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
	}
	return nil
}
