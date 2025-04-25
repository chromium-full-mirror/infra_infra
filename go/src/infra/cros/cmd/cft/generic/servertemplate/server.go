// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servertemplate

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type args struct {
	port int
}

func startServer(name, artifactDir string, flagSet *flag.FlagSet, concreteService GenericService) error {
	a := args{}
	flagSet.IntVar(&a.port, "port", 0, "Specify the port for the server. Default value: 0")
	flagSet.Parse(os.Args[2:])

	logFile, err := common.CreateLogFile(artifactDir)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	logger := common.NewLogger(logFile)
	log.SetOutput(logger.Writer())

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("failed to create a net listener: %w", err)
	}
	err = portdiscovery.WriteServiceMetadata(name, l.Addr().String(), logger)
	if err != nil {
		return fmt.Errorf("failed to write metadata port: %w", err)
	}
	server := NewServer(logger, concreteService)

	err = server.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	logger.Printf("Started %s on %s", name, l.Addr().String())

	return nil
}

func Server(name, artifactDir string, concreteService GenericService) error {
	fs := flag.NewFlagSet(fmt.Sprintf("Run %s server", name), flag.ExitOnError)
	return startServer(name, artifactDir, fs, concreteService)
}

func ServerWithFlagSet(name, artifactDir string, flagSet *flag.FlagSet, concreteService GenericService) error {
	return startServer(name, artifactDir, flagSet, concreteService)
}
