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
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	artifact "go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

func GenerateFilterExecutor() servertemplate.Filter {
	return &ANTSPublishUpdater{}
}

type ANTSPublishUpdater struct {
	servertemplate.Filter

	PublishPath  string
	InvocationID string
	WorkUnitID   string
	AccountID    string
}

func (apu *ANTSPublishUpdater) Init(args []string) error {
	fs := flag.NewFlagSet("Run ants publish filter", flag.ExitOnError)
	fs.StringVar(&apu.PublishPath, "publish-path", "", "SHA256 value for testing publish container")
	fs.StringVar(&apu.InvocationID, "invocation-id", "", "ants invocation id")
	fs.StringVar(&apu.WorkUnitID, "workunit-id", "", "parent workunit id")
	fs.StringVar(&apu.AccountID, "account-id", "", "account id")

	return fs.Parse(args)
}

func (apu *ANTSPublishUpdater) antsPublishMetadata() *metadata.PublishAntsMetadata {
	publishMetadata := &metadata.PublishAntsMetadata{
		PrimaryExecutionInfo: &artifact.ExecutionInfo{
			DutInfo: &artifact.DutInfo{
				Dut: &labapi.Dut{},
			},
			EnvInfo: &artifact.ExecutionInfo_SkylabInfo{
				SkylabInfo: &artifact.SkylabInfo{
					BuildbucketInfo: &artifact.BuildbucketInfo{
						//  Default value required to bypass protobufs omitempty.
						AncestorIds: []int64{0},
					},
				},
			},
		},
		SchedulingMetadata: &artifact.SchedulingMetadata{
			SchedulingArgs: map[string]string{},
		},
	}

	if apu.InvocationID != "" {
		publishMetadata.AntsInvocationId = apu.InvocationID
	}

	if apu.WorkUnitID != "" {
		publishMetadata.ParentWorkUnitId = apu.WorkUnitID
	}

	if apu.AccountID != "" {
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

func (apu *ANTSPublishUpdater) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	var err error
	ctx := context.Background()

	log.Println("Executing ants publish request-updater filter")

	apu.PublishPath, err = common.ProcessContainerPath(ctx, commonParams, apu.PublishPath, "ants-publish")
	if err != nil {
		return req, err
	}

	// Add request to publish using ants-publish container.
	if err := GeneratePublishTask(req, apu.antsPublishMetadata(), apu.PublishPath, log); err != nil {
		log.Printf("Error while generating publish task, %s", err)
		return req, err
	}

	log.Println("Finished generating publish task.")
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
