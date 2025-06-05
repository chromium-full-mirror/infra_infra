// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/bols"
	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type LsNexus struct {
	lsnexus.UnimplementedLSNexusServiceServer
	api.UnimplementedGenericServiceServer

	// During initialization.
	logger *log.Logger

	// Filled during call to Start.
	cl bols.BolsServiceClient
	// dutTopology     *api.Dut
	board           string
	model           string
	pools           []string
	servodSerial    string
	servodContainer string
	servodPort      int32
}

func NewServer(logger *log.Logger) *grpc.Server {
	s := &LsNexus{
		logger: logger,
	}
	server := grpc.NewServer()
	lsnexus.RegisterLSNexusServiceServer(server, s)
	api.RegisterGenericServiceServer(server, s)
	reflection.Register(server)

	return server
}

func StartServer(name, artifactDir string) error {
	var port int
	flagSet := flag.NewFlagSet(name, flag.ContinueOnError)
	flagSet.IntVar(&port, "port", 0, "Specify the port for the server. Default value: 0")
	flagSet.Parse(os.Args[2:])

	logFile, err := common.CreateLogFile(artifactDir)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	logger := common.NewLogger(logFile)
	log.SetOutput(logger.Writer())

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to create a net listener: %w", err)
	}
	err = portdiscovery.WriteServiceMetadata(name, l.Addr().String(), logger)
	if err != nil {
		return fmt.Errorf("failed to write metadata port: %w", err)
	}
	server := NewServer(logger)

	err = server.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	logger.Printf("Started %s on %s", name, l.Addr().String())

	return nil
}
