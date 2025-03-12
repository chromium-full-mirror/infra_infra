// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Represents the server command grouping
package cli

import (
	"flag"
	"fmt"
	"strings"

	"go.chromium.org/chromiumos/test/provision/v2/common-utils/metadata"
	"go.chromium.org/chromiumos/test/provision/v2/common-utils/server"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/constants"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/executor"
)

// ServerCommand executed the provisioning as a Server
type ServerCommand struct {
	metadata     *metadata.ServerMetadata
	logFileName  string
	metadataFile string
	flagSet      *flag.FlagSet
}

func NewServerCommand() *ServerCommand {
	sc := &ServerCommand{
		flagSet:  flag.NewFlagSet("server", flag.ContinueOnError),
		metadata: &metadata.ServerMetadata{},
	}

	sc.flagSet.IntVar(&sc.metadata.Port, "port", constants.DefaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", constants.DefaultPort))
	sc.flagSet.StringVar(&sc.logFileName, "log-path", constants.DefaultLogDirectory, fmt.Sprintf("Path to record execution logs. Default value is %s", constants.DefaultLogDirectory))
	sc.flagSet.StringVar(&sc.metadataFile, "metadata", "", "Specify the request jsonproto input file. Provide service paths and ProvisionState.")
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

	sc.metadata.Log, err = SetUpLog(sc.logFileName)
	if err != nil {
		return fmt.Errorf("unable to set up logs: %s", err)
	}

	if err = sc.validateCLIInputs(); err != nil {
		return err
	}

	return nil
}

// validateCLIInputs ensures the CLI input values are valid
func (cc *ServerCommand) validateCLIInputs() error {
	return nil
}

func (sc *ServerCommand) Run() error {
	sc.metadata.Log.Printf("running server mode:")
	foilProvisionExecutor, err := executor.NewFoilProvisionExecutor(sc.metadata.Log)
	if err != nil {
		sc.metadata.Log.Fatalln("failed to create FoilProvisionExecutor: ", err)
		return err
	}
	ps, closer, err := server.NewProvisionServer(sc.metadata, foilProvisionExecutor)
	defer closer()
	if err != nil {
		sc.metadata.Log.Fatalln("failed to create provision: ", err)
		return err
	}

	if err := ps.Start(); err != nil {
		sc.metadata.Log.Fatalln("failed server execution: ", err)
		return err
	}

	return nil
}
