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
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	artifact "go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"

	"infra/cros/cmd/common_lib/common"
)

const (
	skipTFUpload = "skip_ants_upload"
)

type ANTSPublishUpdater struct {
	PublishPath  string
	InvocationID string
	WorkUnitID   string
	AccountID    string
}

func (apu *ANTSPublishUpdater) antsPublishMetadata(req *api.InternalTestplan) *metadata.PublishAntsMetadata {
	publishMetadata := &metadata.PublishAntsMetadata{
		PrimaryExecutionInfo: &artifact.ExecutionInfo{
			DutInfo: &artifact.DutInfo{
				Dut: &labapi.Dut{},
			},
		},
	}

	if apu.InvocationID == "" {
		publishMetadata.AntsInvocationId = suiteExecutionMetadataArgValue(req, "ants_invocation_id")
	} else {
		publishMetadata.AntsInvocationId = apu.InvocationID
	}

	if apu.WorkUnitID == "" {
		publishMetadata.ParentWorkUnitId = suiteExecutionMetadataArgValue(req, "ants_work_unit_id")
	} else {
		publishMetadata.ParentWorkUnitId = apu.WorkUnitID
	}

	if apu.AccountID == "" {
		publishMetadata.AccountId = strconv.Itoa(internalAccountID)
	} else {
		publishMetadata.AccountId = apu.AccountID
	}

	return publishMetadata
}

func suiteExecutionMetadataArgValue(req *api.InternalTestplan, flag string) string {
	for _, arg := range req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs() {
		if strings.EqualFold(arg.GetFlag(), flag) {
			return arg.GetValue()
		}
	}
	return ""
}

func (apu *ANTSPublishUpdater) skipTFUpload(req *api.InternalTestplan) {
	em := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata()
	args := append(em.GetArgs(), &api.Arg{Flag: skipTFUpload, Value: "true"})
	req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().Args = args
}

func (apu *ANTSPublishUpdater) executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	ctx := context.Background()
	log.Println("Executing ants publish request-updater filter")

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		log.Println(fmt.Errorf("unable to locate dockerKeyFile: %w", err))
	}

	apu.PublishPath, err = processContainerPath(ctx, dockerKeyFile, apu.PublishPath, "ants-publish")
	if err != nil {
		return req, err
	}

	// Skip uploading to Ants using TF plugin.
	apu.skipTFUpload(req)

	// Add request to publish using ants-publish container.
	if err := GeneratePublishTask(req, apu.antsPublishMetadata(req), apu.PublishPath, log); err != nil {
		log.Printf("Error while generating publish task, %s", err)
		return req, err
	}

	log.Println("Finished generating publish task.")
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

	fs := flag.NewFlagSet("Run ants publish filter", flag.ExitOnError)
	fs.StringVar(&publishRequestUpdater.PublishPath, "publish-path", common.LabelProd, "SHA256 value for testing publish container")
	fs.StringVar(&publishRequestUpdater.InvocationID, "invocation-id", "", "ants invocation id")
	fs.StringVar(&publishRequestUpdater.WorkUnitID, "workunit-id", "", "parent workunit id")
	fs.StringVar(&publishRequestUpdater.AccountID, "account-id", "", "account id")

	log.Printf("publishRequestUpdater %+v", publishRequestUpdater)

	//  Start the server
	err := server.ServerWithFlagSet(fs, publishRequestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
