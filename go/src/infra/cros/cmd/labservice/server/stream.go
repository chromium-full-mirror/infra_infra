// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"

	"google.golang.org/grpc"
)

// serverStream overrides behavior of `grpc.serverStream` by allowing us to
// set and get the context.
type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context implements the Context() method of the serverStream interface.
func (s *serverStream) Context() context.Context {
	return s.ctx
}
