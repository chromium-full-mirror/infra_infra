// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

const (
	defaultRootPath          = "/tmp/vm-provision"
	vmLeaserEndPoint         = "vmleaser.api.cr.dev:443"
	defaultAuthTokenFilePath = "/tmp/vm-provision/authToken.txt"
)

var defaultPort = 8010

type args struct {
	// Common input params.
	logPath      string
	port         int
	authFilePath string
}

// startServer is the entry point for running vm-provision in server mode.
func startServer(d []string) int {
	a := args{}
	t := time.Now()
	defaultLogPath := filepath.Join(defaultRootPath, t.Format("20060102-150405"))
	fs := flag.NewFlagSet("Run vm-provision", flag.ExitOnError)
	fs.StringVar(&a.logPath, "log", defaultLogPath, fmt.Sprintf("Path to record finder logs. Default value is %s", defaultLogPath))
	fs.IntVar(&a.port, "port", defaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", defaultPort))
	fs.StringVar(&a.authFilePath, "authFilePath", defaultAuthTokenFilePath, fmt.Sprintf("Path to auth token file. Default is %s", defaultAuthTokenFilePath))
	fs.Parse(d)

	// Configure log file
	logFile, err := common_utils.CreateLogFile(a.logPath)
	if err != nil {
		log.Fatalln("Failed to create log file", err)
		return 2
	}
	defer logFile.Close()
	logger := common_utils.NewLogger(logFile)

	// Read and validate auth token required by vm-provision
	authTokenFilePath := defaultAuthTokenFilePath
	if a.authFilePath != "" {
		authTokenFilePath = a.authFilePath
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		logger.Fatalln("Failed to create a net listener: ", err)
		return 2
	}

	logger.Println("Starting Service on port ", a.port)

	server, closer := NewServer(logger, authTokenFilePath)
	defer closer()

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("vm-provision", l.Addr().String(), logger)
	if err != nil {
		logger.Println("Warning: error when writing to metadata file: ", err)
	}
	err = server.Serve(l)
	if err != nil {
		logger.Fatalln("Failed to initialize server: ", err)
		return 2
	}

	return 0
}

func mainInternal(ctx context.Context) int {
	return startServer(os.Args[1:])
}

func main() {
	os.Exit(mainInternal(context.Background()))
}
