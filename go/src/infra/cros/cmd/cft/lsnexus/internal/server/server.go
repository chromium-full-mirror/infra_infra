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
	"path/filepath"

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
	logger      *log.Logger
	artifactDir string

	// Filled during call to Start.
	cl       bols.BolsServiceClient
	bolsConn *grpc.ClientConn
	// dutTopology     *api.Dut
	board           string
	model           string
	pools           []string
	servodSerial    string
	servodContainer string
	servodPort      int32
}

func NewServer(logger *log.Logger, artifactDir string) *grpc.Server {
	s := &LsNexus{
		logger:      logger,
		artifactDir: artifactDir,
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
	server := NewServer(logger, artifactDir)

	err = server.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	logger.Printf("Started %s on %s", name, l.Addr().String())

	return nil
}

// StartServerForTesting initializes and starts an LSNexus gRPC server specifically for testing.
// It requires a BOLS address and will fail if the address is empty or connection fails.
// Returns the server's address, a function to stop the server (which also closes BOLS conn), and any error.
func StartServerForTesting(
	artifactDir string,
	board string,
	model string,
	pools []string,
	servodSerial string,
	servodContainer string,
	servodPort int32,
	bolsAddr string) (addr string, stopFunc func(), err error) {

	logFile, err := common.CreateLogFile(filepath.Join(artifactDir, "lsnexus_test_server_log.txt"))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create log file: %w", err)
	}
	// stopFunc will be responsible for closing logFile

	logger := common.NewLogger(logFile)

	if bolsAddr == "" {
		logFile.Close() // Clean up log file on early exit
		return "", nil, fmt.Errorf("StartServerForTesting: BOLS address cannot be empty")
	}

	logger.Printf("StartServerForTesting: Attempting to connect to BOLS at %s", bolsAddr)
	bolsClient, bolsGrpcConn, connErr := connectToBOLS(bolsAddr)
	if connErr != nil {
		logFile.Close() // Clean up log file on early exit
		// Error is already descriptive from connectToBOLS
		return "", nil, fmt.Errorf("StartServerForTesting: %w", connErr)
	}
	logger.Printf("StartServerForTesting: Successfully connected to BOLS at %s", bolsAddr)

	lsNexusService := &LsNexus{
		logger:          logger,
		artifactDir:     artifactDir,
		board:           board,
		model:           model,
		pools:           pools,
		servodSerial:    servodSerial,
		servodContainer: servodContainer,
		servodPort:      servodPort,
		cl:              bolsClient,
		bolsConn:        bolsGrpcConn, // Store the connection
	}

	server := grpc.NewServer()
	lsnexus.RegisterLSNexusServiceServer(server, lsNexusService)
	api.RegisterGenericServiceServer(server, lsNexusService)
	reflection.Register(server)

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		bolsGrpcConn.Close() // Clean up BOLS connection
		logFile.Close()      // Clean up log file
		return "", nil, fmt.Errorf("failed to listen: %w", err)
	}

	addr = lis.Addr().String()
	logger.Printf("LSNexus test server (via StartServerForTesting) listening on %s", addr)

	go func() {
		if errSrv := server.Serve(lis); errSrv != nil {
			if errSrv != grpc.ErrServerStopped {
				logger.Printf("LSNexus test server (via StartServerForTesting) failed to serve: %v", errSrv)
			}
		}
	}()

	stopFunc = func() {
		if lsNexusService.bolsConn != nil {
			logger.Printf("LSNexus test server: Closing BOLS client connection to %s", bolsAddr)
			if errClose := lsNexusService.bolsConn.Close(); errClose != nil {
				logger.Printf("LSNexus test server: Error closing BOLS connection: %v", errClose)
			}
		}
		server.Stop()
		logger.Printf("LSNexus test server (via StartServerForTesting) stopped on %s", addr)
		logFile.Close()
	}

	return addr, stopFunc, nil
}
