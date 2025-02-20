// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	smpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/suite_manager"

	"go.chromium.org/infra/cros/cmd/suite_manager/server"
)

func innerRun() int {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		slog.Error(err.Error())
		return 1
	}

	suiteManagerServer := server.InitServer()

	opts := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opts...)

	smpb.RegisterSuiteManagerServiceServer(grpcServer, suiteManagerServer)
	reflection.Register(grpcServer)
	err = grpcServer.Serve(listener)
	if err != nil {
		slog.Error(err.Error())
		return 1
	}

	return 0
}

func main() {
	os.Exit(innerRun())
}
