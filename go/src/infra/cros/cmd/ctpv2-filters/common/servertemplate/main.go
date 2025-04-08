// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the post-process for finding tests based on tags.
package servertemplate

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

const (
	DefaultLogPath        = "/tmp/filters"
	TestPlatformFireStore = "test-platform-store"
)

// Version is the version info of this command. It is filled in during emerge.
var Version = "<unknown>"
var defaultPort = 0

type args struct {
	// Common input params.
	logPath string

	// Server mode params
	port int
}

// startServer is the entry point for running post-process (TestFinderService) in server mode.
func startServer(flagSet *flag.FlagSet, executor func(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error), name string) error {
	a := args{}
	commonParams := common.CommonFilterParams{}
	t := time.Now()
	flagSet.StringVar(&a.logPath, "log", DefaultLogPath, fmt.Sprintf("Base path to record logs. Default value is %s", DefaultLogPath))
	flagSet.IntVar(&a.port, "port", defaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", defaultPort))
	flagSet.StringVar(&commonParams.FirestoreDatabaseName, "firestore", TestPlatformFireStore, fmt.Sprintf("Firestore database name to pull from. Default value is %s", TestPlatformFireStore))
	flagSet.StringVar(&commonParams.Environment, "env", "prod", "Environment of the run. Default value is prod")

	flagSet.Parse(os.Args[2:])

	logFile, err := common.CreateLogFile(filepath.Join(a.logPath, name, t.Format("20060102-150405")))
	if err != nil {
		return fmt.Errorf("failed to create log file: %s", err)
	}
	defer logFile.Close()

	logger := common.NewLogger(logFile)
	log.SetOutput(logger.Writer())

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("failed to create a net listener: %s", err)
	}
	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata(name, l.Addr().String(), logger)
	if err != nil {
		return fmt.Errorf("failed to write metadata port: %s", err)
	}
	server, closer := NewServer(logger, a.logPath, name, &commonParams, executor)
	defer closer()

	err = server.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %s", err)
	}
	logger.Println("Starting generic filter service on port ", a.port)

	return nil
}

// Server starts the generic filter server.
func Server(executor func(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error), name string) error {
	fs := flag.NewFlagSet("Run generic-filter server", flag.ExitOnError)
	return startServer(fs, executor, name)
}

// ServerWithFlagSet starts the generic filter server
// with a custom flagSet extending the default flags.
func ServerWithFlagSet(flagSet *flag.FlagSet, executor func(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error), name string) error {
	return startServer(flagSet, executor, name)
}
