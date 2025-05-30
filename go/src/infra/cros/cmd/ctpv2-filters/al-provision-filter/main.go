// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/android"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

const (
	binName                            = "al-provision-filter"
	androidBuildInternalScope          = "https://www.googleapis.com/auth/androidbuild.internal"
	cloudPlatformScope                 = "https://www.googleapis.com/auth/cloud-platform"
	androidBuildInternalBuildsEndpoint = "https://androidbuildinternal.googleapis.com/android/internal/build/v3/builds"
	gceServiceAccountJSONPath          = "/creds/service_accounts/service-account-chromeos.json"
)

func GenerateFilterExecutor() servertemplate.Filter {
	return &ALProvisionRequestUpdater{
		LatestBuildsByBoard: make(map[string]int),
	}
}

// ALProvisionRequestUpdater struct stores
type ALProvisionRequestUpdater struct {
	servertemplate.FilterBase

	ProvisionPath string
	ServoPath     string

	LatestBuildsByBoard map[string]int
	AndroidAuthHandler  *android.CloudRunFilterAuthenticator
}

func (pru *ALProvisionRequestUpdater) Init(fs *flag.FlagSet, args []string) error {
	fs.StringVar(&pru.ProvisionPath, "prov-path", "", "SHA256 value for provision container")
	fs.StringVar(&pru.ServoPath, "servo-path", "", "SHA256 value for servo-nexus container")

	return fs.Parse(args)
}

func (pru *ALProvisionRequestUpdater) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	var err error
	pru.AndroidAuthHandler = &android.CloudRunFilterAuthenticator{
		AuthHandler: commonParams.AuthHelper,
	}
	log.Println("Executing AL provision Filter - Updates provision request.")

	pru.ProvisionPath, err = common.ProcessContainerPath(context.Background(), commonParams, pru.ProvisionPath, "foil-provision")
	if err != nil {
		return req, err
	}
	pru.ServoPath, err = common.ProcessContainerPath(context.Background(), commonParams, pru.ServoPath, "servo-nexus")
	if err != nil {
		return req, err
	}

	if err := GenerateDynamicProvisionUpdates(req, pru, log, commonParams); err != nil {
		log.Printf("Error while generating dynamic updates, %s", err)
		return req, err
	}
	log.Println("Finished generating dynamic updates.")

	return req, nil

}

func main() {
	err := servertemplate.Server(GenerateFilterExecutor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
