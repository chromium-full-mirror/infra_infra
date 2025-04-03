// Copyright 2024 The Chromium Authors
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

const (
	binName                            = "al-provision-filter"
	androidBuildInternalScope          = "https://www.googleapis.com/auth/androidbuild.internal"
	cloudPlatformScope                 = "https://www.googleapis.com/auth/cloud-platform"
	androidBuildInternalBuildsEndpoint = "https://androidbuildinternal.googleapis.com/android/internal/build/v3/builds"
	gceServiceAccountJSONPath          = "/creds/service_accounts/service-account-chromeos.json"
)

// ALProvisionRequestUpdater struct stores
type ALProvisionRequestUpdater struct {
	ProvisionPath string
	ServoPath     string

	LatestBuildsByBoard map[string]int
}

func (pru *ALProvisionRequestUpdater) executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	log.Println("Executing AL provision Filter - Updates provision request.")
	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VMLabDockerKeyFileLocation})
	if err != nil {
		log.Println(fmt.Errorf("unable to locate dockerKeyFile: %w", err))
	}

	pru.ProvisionPath, err = common.ProcessContainerPath(context.Background(), commonParams, dockerKeyFile, pru.ProvisionPath, "foil-provision")
	if err != nil {
		return req, err
	}
	pru.ServoPath, err = common.ProcessContainerPath(context.Background(), commonParams, dockerKeyFile, pru.ServoPath, "servo-nexus")
	if err != nil {
		return req, err
	}

	if err := GenerateDynamicProvisionUpdates(req, pru, log); err != nil {
		log.Printf("Error while generating dynamic updates, %s", err)
		return req, err
	}
	log.Println("Finished generating dynamic updates.")

	return req, nil

}

func main() {
	provisionRequestUpdater := &ALProvisionRequestUpdater{
		LatestBuildsByBoard: make(map[string]int),
	}
	fs := flag.NewFlagSet("Run Al provision filter", flag.ExitOnError)
	fs.StringVar(&provisionRequestUpdater.ProvisionPath, "prov-path", "", "SHA256 value for provision container")
	fs.StringVar(&provisionRequestUpdater.ServoPath, "servo-path", "", "SHA256 value for servo-nexus container")
	err := servertemplate.ServerWithFlagSet(fs, provisionRequestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
