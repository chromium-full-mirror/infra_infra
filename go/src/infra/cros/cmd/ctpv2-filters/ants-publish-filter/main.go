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

type ANTSPublishUpdater struct {
	PublishPath string
}

func (apu *ANTSPublishUpdater) executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	log.Println("Executing ants publish request-updater filter")

	ctx := context.Background()
	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		log.Println(fmt.Errorf("unable to locate dockerKeyFile: %w", err))
	}

	apu.PublishPath, err = processContainerPath(ctx, dockerKeyFile, apu.PublishPath, "ants-publish")
	if err != nil {
		return req, err
	}

	if err := GenerateDynamicUpdates(req, apu, log); err != nil {
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
		if err != nil {
			return "", fmt.Errorf("failed to create image path, %w", err)
		}
	default:
		processedPath = path
	}

	return
}

func main() {
	publishRequestUpdater := &ANTSPublishUpdater{}

	//  Hello!
	fs := flag.NewFlagSet("Run ants publish filter", flag.ExitOnError)
	fs.StringVar(&publishRequestUpdater.PublishPath, "publish-path", common.LabelProd, "SHA256 value for testing publish container")
	log.Printf("publishRequestUpdater %+v", publishRequestUpdater)

	//  Do we have any cli options to handle?

	//  Start the server
	err := server.ServerWithFlagSet(fs, publishRequestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
