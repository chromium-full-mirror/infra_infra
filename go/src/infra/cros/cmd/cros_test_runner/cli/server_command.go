// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Represents the server command grouping
package cli

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"infra/cros/cmd/cros_test_runner/service"
)

// ServerCommand executes as a server
type ServerCommand struct {
	metadata  *service.ServerMetadata
	inputFile string
	flagSet   *flag.FlagSet
}

func NewServerCommand() *ServerCommand {
	sc := &ServerCommand{
		flagSet:  flag.NewFlagSet("server", flag.ContinueOnError),
		metadata: &service.ServerMetadata{},
	}

	sc.flagSet.IntVar(&sc.metadata.Port, "port", 0, fmt.Sprintf("Specify the port for the server. Default value %d.", 0))
	sc.flagSet.StringVar(&sc.metadata.ServiceMetadataExportPath, "export-metadata", ".", fmt.Sprintf("folder path to export cros test runner server metadata into. Default value is %s", "./"))
	sc.flagSet.StringVar(&sc.inputFile, "input", "", "Specify the request jsonproto input file. Must provide local artifact directory path.")
	sc.flagSet.StringVar(&sc.metadata.LogPath, "log-path", ".", fmt.Sprintf("Path to record execution logs. Default value is %s", "./"))
	sc.flagSet.BoolVar(&sc.metadata.NoSudo, "no-sudo", false, "If present, CTRR will not call internal commands with root privileges")
	return sc
}

func (sc *ServerCommand) Is(group string) bool {
	return strings.HasPrefix(group, "s")
}

func (sc *ServerCommand) Name() string {
	return "server"
}

func (sc *ServerCommand) Init(args []string) error {
	err := sc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	if err = sc.validate(); err != nil {
		return err
	}

	sc.metadata.InputProto, err = service.ParseServerStartReq(sc.inputFile)
	if err != nil {
		return fmt.Errorf("unable to parse CrosTestRunnerServerStartRequest proto: %w", err)
	}

	return nil
}

// validate checks if inputs are ok
func (sc *ServerCommand) validate() error {
	if sc.inputFile == "" {
		return errors.New("input file not specified")
	}

	return nil
}

func (sc *ServerCommand) Run() error {
	log.Printf("running server mode:")

	server, closer, err := service.NewCrosTestRunnerServer(sc.metadata)
	defer closer()
	if err != nil {
		log.Fatalln("failed to create cros-test-runner server: ", err)
		return err
	}

	if err := server.Start(); err != nil {
		log.Fatalln("failed to start cros-test-runner server: ", err)
		return err
	}

	return nil
}
