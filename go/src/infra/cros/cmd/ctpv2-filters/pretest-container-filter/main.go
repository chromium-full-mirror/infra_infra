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

func GenerateFilterExecutor() servertemplate.Filter {
	return &PreTestContainerUpdater{}
}

type PreTestContainerUpdater struct {
	servertemplate.FilterBase

	ContainerPath   string
	ContainerName   string
	ContainerRunCmd string
	Volumes         string
	TestCLIArg      string
	TestParamName   string
}

func (gcu *PreTestContainerUpdater) Init(args []string) error {
	fs := flag.NewFlagSet("Run pretest test container filter", flag.ContinueOnError)
	fs.StringVar(&gcu.ContainerPath, "path", "", "SHA256 value for the container")
	fs.StringVar(&gcu.ContainerName, "name", "", "name of the container to use in firestore")
	fs.StringVar(&gcu.ContainerRunCmd, "run-cmd", "", "the command to be used when launching the container")
	fs.StringVar(&gcu.Volumes, "volumes", "", "volumes to be mounted in the container")
	fs.StringVar(&gcu.TestCLIArg, "test-cli-arg", "", "name of the argument to use when appending the container address to execution metadata")
	fs.StringVar(&gcu.TestParamName, "test-param-name", "", "the name of the 'params' that the test-cli-args apply to (Mobly only)")

	return fs.Parse(args)
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
	//  Start the server
	err := servertemplate.Server(GenerateFilterExecutor, "request-updater")
	if err != nil {
		log.Println(fmt.Errorf("error when running server, %w", err))
		os.Exit(2)
	}
	os.Exit(0)
}
