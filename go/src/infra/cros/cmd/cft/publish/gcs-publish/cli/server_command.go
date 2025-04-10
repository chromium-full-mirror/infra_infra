// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Represents the server command grouping
package cli

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/metadata"
	"go.chromium.org/infra/cros/cmd/cft/publish/gcs-publish/constants"
	"go.chromium.org/infra/cros/cmd/cft/publish/gcs-publish/server"
)

// ServerCommand executed the GCS publish as a Server
type ServerCommand struct {
	metadata    *metadata.ServerMetadata
	logFileName string
	flagSet     *flag.FlagSet
}

func NewServerCommand() *ServerCommand {
	sc := &ServerCommand{
		flagSet:  flag.NewFlagSet("server", flag.ContinueOnError),
		metadata: &metadata.ServerMetadata{},
	}

	sc.flagSet.IntVar(&sc.metadata.Port, "port", constants.DefaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", constants.DefaultPort))
	sc.flagSet.StringVar(&sc.logFileName, "log-path", constants.DefaultLogDirectory, fmt.Sprintf("Path to record execution logs. Default value is %s", constants.DefaultLogDirectory))
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

	if err = SetUpLog(sc.logFileName); err != nil {
		return fmt.Errorf("unable to set up logs: %s", err)
	}

	return nil
}

func (sc *ServerCommand) Run() error {
	log.Printf("running server mode:")

	ps, closer, err := server.NewGcsPublishServer(sc.metadata)
	defer closer()
	if err != nil {
		log.Fatalln("failed to create provision: ", err)
		return err
	}

	if err := ps.Start(); err != nil {
		log.Fatalln("failed server execution: ", err)
		return err
	}

	return nil
}
