// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

type PreTestContainerServerArgs struct {
	ContainerPath   string
	ContainerName   string
	ContainerRunCmd string
	Volumes         string
	TestCLIArg      string
	TestParamName   string
}

func (serverArgs *PreTestContainerServerArgs) GenerateFilterExecutor() servertemplate.Filter {
	return &PreTestContainerUpdater{
		ContainerPath:   serverArgs.ContainerPath,
		ContainerName:   serverArgs.ContainerName,
		ContainerRunCmd: serverArgs.ContainerRunCmd,
		Volumes:         serverArgs.Volumes,
		TestCLIArg:      serverArgs.TestCLIArg,
		TestParamName:   serverArgs.TestParamName,
	}
}

type PreTestContainerUpdater struct {
	servertemplate.Filter

	ContainerPath   string
	ContainerName   string
	ContainerRunCmd string
	Volumes         string
	TestCLIArg      string
	TestParamName   string
}

func (gcu *PreTestContainerUpdater) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	var err error
	log.Println("Executing request-updater filter.")

	ctx := context.Background()

	gcu.ContainerPath, err = common.ProcessContainerPath(ctx, commonParams, gcu.ContainerPath, gcu.ContainerName)
	if err != nil {
		return req, err
	}

	if err := InsertContainer(req, gcu, log); err != nil {
		log.Printf("Error while generating dynamic updates, %s", err)
		return req, err
	}
	if err := AddContainerArg(req, gcu, log); err != nil {
		log.Printf("Error while adding additional container test args, %s", err)
		return req, err
	}
	log.Println("Finished generating dyanmic updates.")

	return req, nil
}

func main() {
	serverArgs := &PreTestContainerServerArgs{}

	fs := flag.NewFlagSet("Run pretest test container filter", flag.ExitOnError)
	fs.StringVar(&serverArgs.ContainerPath, "path", "", "SHA256 value for the container")
	fs.StringVar(&serverArgs.ContainerName, "name", "", "name of the container to use in firestore")
	fs.StringVar(&serverArgs.ContainerRunCmd, "run-cmd", "", "the command to be used when launching the container")
	fs.StringVar(&serverArgs.Volumes, "volumes", "", "volumes to be mounted in the container")
	fs.StringVar(&serverArgs.TestCLIArg, "test-cli-arg", "", "name of the argument to use when appending the container address to execution metadata")
	fs.StringVar(&serverArgs.TestParamName, "test-param-name", "", "the name of the 'params' that the test-cli-args apply to (Mobly only)")

	log.Printf("serverArgs %+v", serverArgs)

	//  Start the server
	err := servertemplate.ServerWithFlagSet(fs, serverArgs.GenerateFilterExecutor, "request-updater")
	if err != nil {
		log.Println(fmt.Errorf("error when running server, %w", err))
		os.Exit(2)
	}
	os.Exit(0)
}
