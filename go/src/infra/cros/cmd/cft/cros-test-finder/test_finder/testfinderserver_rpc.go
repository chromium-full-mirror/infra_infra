// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package test_finder

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

// TestFinderServiceServer implementation of dut_service.proto
type TestFinderServiceServer struct {
	api.UnimplementedTestFinderServiceServer
	*servertemplate.GenericFilterServiceServer
}

// NewServer creates an execution server.
func NewServer(logger *log.Logger, logPath, name string, executorGenerator servertemplate.ExecutorGeneratorFunc) *grpc.Server {
	s := &TestFinderServiceServer{
		GenericFilterServiceServer: &servertemplate.GenericFilterServiceServer{
			LogPath:            logPath,
			Name:               name,
			ServerLogger:       logger,
			ExecutionGenerator: executorGenerator,
		},
	}

	server := grpc.NewServer(grpc.MaxRecvMsgSize(1024*1024*32), grpc.MaxSendMsgSize(1024*1024*32))

	api.RegisterTestFinderServiceServer(server, s)
	api.RegisterGenericFilterServiceServer(server, s)
	reflection.Register(server)

	logger.Println("cros-test-finder/filter service listening for requests")
	return server
}

// FindTests calls the innerMain (test-finder flow) in main.
func (s *TestFinderServiceServer) FindTests(ctx context.Context, req *api.CrosTestFinderRequest) (*api.CrosTestFinderResponse, error) {
	s.ServerLogger.Println("Received api.CrosTestFinderRequest: ", req)

	rspn, err := FindTests(s.ServerLogger, req, defaultTestMetadataDir)
	if err != nil {
		return nil, errors.Annotate(err, "FindTests: failed to find tests").Err()
	}
	s.ServerLogger.Printf("FindTest RPC Command was successful")
	return rspn, nil
}
