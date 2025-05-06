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
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

type FoilRequestUpdater struct {
	TestPath          string
	GcsPublishPath    string
	RdbPublishPath    string
	CpconPublishPath  string
	FilterTests       bool
	EnableXtsArchiver bool
}

func (ru *FoilRequestUpdater) executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	var err error
	log.Println("Executing request-updater filter.")

	ctx := context.Background()

	ru.TestPath, err = common.ProcessContainerPath(ctx, commonParams, ru.TestPath, "foil-test")
	if err != nil {
		return req, err
	}
	ru.GcsPublishPath, err = common.ProcessContainerPath(ctx, commonParams, ru.GcsPublishPath, "gcs-publish")
	if err != nil {
		return req, err
	}
	ru.RdbPublishPath, err = common.ProcessContainerPath(ctx, commonParams, ru.RdbPublishPath, "rdb-publish")
	if err != nil {
		return req, err
	}
	if commonParams.FirestoreDatabaseName == common.PartnerTestPlatformFireStore {
		ru.CpconPublishPath, err = common.ProcessContainerPath(ctx, commonParams, ru.CpconPublishPath, common.CpconPublish)
		if err != nil {
			return req, err
		}
	} else {
		ru.CpconPublishPath = ""
	}

	if err := GenerateDynamicUpdates(req, ru, log); err != nil {
		log.Printf("Error while generating dynamic updates, %s", err)
		return req, err
	}
	log.Println("Finished generating dyanmic updates.")

	return req, nil
}

func main() {
	requestUpdater := &FoilRequestUpdater{}
	fs := flag.NewFlagSet("Run foil request-updater", flag.ExitOnError)
	fs.StringVar(&requestUpdater.TestPath, "test-path", "", "SHA256 value for test container")
	fs.StringVar(&requestUpdater.GcsPublishPath, "gcs-path", "", "SHA256 value for gcs publish container")
	fs.StringVar(&requestUpdater.RdbPublishPath, "rdb-path", "", "SHA256 value for rdb publish container")
	fs.StringVar(&requestUpdater.CpconPublishPath, "cpcon-path", "", "SHA256 value for cpcon publish container")
	fs.BoolVar(&requestUpdater.FilterTests, "filter-tests", false, "Filter out known faulty tests due to their device breaking behavior")
	fs.BoolVar(&requestUpdater.EnableXtsArchiver, "enable-xts-archiver", false, "Whether to archive xTS results for release qualification")

	err := servertemplate.ServerWithFlagSet(fs, requestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
