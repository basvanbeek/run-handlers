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

package grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type statusError struct {
	m string
	c codes.Code
}

// StatusError creates a new error embedding a gRPC status code and
// regular error.
func StatusError(c codes.Code, msg string) error {
	return &statusError{c: c, m: msg}
}
func (s statusError) Error() string {
	return s.m
}

func (s statusError) String() string {
	return s.m
}

func (s statusError) GRPCStatus() *status.Status {
	return status.New(s.c, s.m)
}
