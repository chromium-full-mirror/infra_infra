// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"fmt"
	"net"

	"google.golang.org/grpc"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// Server is a long running server that serves labservice RPCs.
type Server struct {
	// gs is the underlying gRPC server.
	gs *grpc.Server
	// lis is the underlying network listener.
	lis net.Listener
}

// New creates a new labservice server.
//
// The server does not start serving on the given address until Start is called.
func New(addr string, cfg *Config) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("new server: %w", err)
	}
	ic := interceptor{}
	gs := grpc.NewServer(ic.unaryOption(), grpc.StreamInterceptor(streamNamespaceInterceptor))
	labapi.RegisterInventoryServiceServer(gs, newInventoryServer(cfg))
	return &Server{
		gs:  gs,
		lis: lis,
	}, nil
}

// Start runs the server.
//
// This method blocks until the server is stopped.
func (s *Server) Start() error {
	return s.gs.Serve(s.lis)
}

// Stop gracefully stops the server.
func (s *Server) Stop(graceful bool) {
	if graceful {
		s.gs.GracefulStop()
	} else {
		s.gs.Stop()
	}
}

// A Config configures a new server.
type Config struct {
	// PreferredCachingServices is a list of caching services to use.
	//
	// The format is "[http://]server[:port]".
	// These services supersede the ones fetched from UFS.
	PreferredCachingServices []string
	// ServiceAccountPath is the path to a service account JSON file.
	ServiceAccountPath string
	// UFSService is the UFS service host.
	UFSService string
}
