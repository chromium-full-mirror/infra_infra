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

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type FoilRequestUpdater struct {
	TestPath          string
	GcsPublishPath    string
	RdbPublishPath    string
	FilterTests       bool
	EnableXtsArchiver bool
}

func (ru *FoilRequestUpdater) executor(req *api.InternalTestplan, log *log.Logger, commonParams *server.CommonFilterParams) (*api.InternalTestplan, error) {
	log.Println("Executing request-updater filter.")

	ctx := context.Background()

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VMLabDockerKeyFileLocation})
	if err != nil {
		log.Println(fmt.Errorf("unable to locate dockerKeyFile: %w", err))
	}

	ru.TestPath, err = common.ProcessContainerPath(ctx, commonParams, dockerKeyFile, ru.TestPath, "foil-test")
	if err != nil {
		return req, err
	}
	ru.GcsPublishPath, err = common.ProcessContainerPath(ctx, commonParams, dockerKeyFile, ru.GcsPublishPath, "gcs-publish")
	if err != nil {
		return req, err
	}
	ru.RdbPublishPath, err = common.ProcessContainerPath(ctx, commonParams, dockerKeyFile, ru.RdbPublishPath, "rdb-publish")
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

func processContainerPath(ctx context.Context, firestoreDatabasename, creds, path, firestoreName string) (processedPath string, err error) {
	switch path {
	case common.LabelProd, common.LabelStaging:
		testContainer, err := common.FetchFilterFromFirestore(ctx, firestoreDatabasename, creds, path, firestoreName)
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
	fs.StringVar(&requestUpdater.TestPath, "test-path", "", "SHA256 value for test container")
	fs.StringVar(&requestUpdater.GcsPublishPath, "gcs-path", "", "SHA256 value for gcs publish container")
	fs.StringVar(&requestUpdater.RdbPublishPath, "rdb-path", "", "SHA256 value for rdb publish container")
	fs.BoolVar(&requestUpdater.FilterTests, "filter-tests", false, "Filter out known faulty tests due to their device breaking behavior")
	fs.BoolVar(&requestUpdater.EnableXtsArchiver, "enable-xts-archiver", false, "Whether to archive xTS results for release qualification")

	err := server.ServerWithFlagSet(fs, requestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
