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
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"

	"infra/cros/cmd/common_lib/common"
)

type FoilRequestUpdater struct {
	ProvisionPath   string
	ProvisionBinary string
	TestPath        string
	FilterTests     bool

	buildStr string
	buildNum string
}

func (ru *FoilRequestUpdater) executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	log.Println("Executing request-updater filter.")

	log.Println("Setting build.")
	ru.buildStr = "brya-trunk_staging-userdebug"
	ru.buildNum = "12330924"

	ctx := context.Background()

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		log.Println(fmt.Errorf("unable to locate dockerKeyFile: %w", err))
	}

	ru.ProvisionPath, err = processContainerPath(ctx, dockerKeyFile, ru.ProvisionPath, "foil-provision")
	if err != nil {
		return req, err
	}
	ru.TestPath, err = processContainerPath(ctx, dockerKeyFile, ru.TestPath, "foil-test")
	if err != nil {
		return req, err
	}

	if err := GenerateDynamicUpdates(req, ru, log); err != nil {
		log.Printf("Error while generating dynamic updates, %s", err)
		return req, err
	}
	log.Println("Finished generating dyanmic updates.")

	return req, nil
}

func processContainerPath(ctx context.Context, creds, path, firestoreName string) (processedPath string, err error) {
	switch path {
	case common.LabelProd, common.LabelStaging:
		testContainer, err := common.FetchFilterFromFirestore(ctx, creds, path, firestoreName)
		if err != nil {
			return "", fmt.Errorf("failed to fetch %s, %w", firestoreName, err)
		}
		processedPath, err = common.CreateImagePath(testContainer.GetContainerInfo().GetContainer())
	default:
		processedPath = path
	}

	return
}

func main() {
	requestUpdater := &FoilRequestUpdater{}
	fs := flag.NewFlagSet("Run foil request-updater", flag.ExitOnError)
	fs.StringVar(&requestUpdater.ProvisionPath, "prov-path", common.LabelProd, "SHA256 value for provision container")
	fs.StringVar(&requestUpdater.ProvisionBinary, "prov-bin", "foil-provision", "Binary called within provision container")
	fs.StringVar(&requestUpdater.TestPath, "test-path", common.LabelProd, "SHA256 value for test container")
	fs.BoolVar(&requestUpdater.FilterTests, "filter-tests", false, "Filter out known faulty tests due to their device breaking behavior")

	err := server.ServerWithFlagSet(fs, requestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
