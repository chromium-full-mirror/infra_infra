// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/generators"
)

func GenerateDynamicUpdates(req *testapi.InternalTestplan, apu *ANTSPublishUpdater, log *log.Logger) error {
	modifyAntsPublishRequest(req, apu, log)
	return nil
}

func modifyAntsPublishRequest(req *testapi.InternalTestplan, apu *ANTSPublishUpdater, log *log.Logger) {
	log.Println("Modifying ants publish request")

	antsContainerBuilder := builders.NewContainerBuilder(
		"ants-publish",      //  ContainerID
		"",                  //  ContainerImageKey
		apu.PublishPath,     //	 Container ImagePath
		"/tmp/ants-publish", //  ContainerArtifactDir
		"ants-publish server -port 0",
	)
	//  Add test artifacts directory for container.
	antsContainerBuilder.DynamicDeps = append(antsContainerBuilder.DynamicDeps,
		&testapi.DynamicDep{
			Key:   "generic.additionalVolumes",
			Value: "FMT=${env-TEMPDIR}:/tmp/artifacts",
		})

	publishMetadata := &metadata.PublishAntsMetadata{
		AntsInvocationId: getSuiteExecutionMetadataFlag(req, "ants_invocation_id"),
		ParentWorkUnitId: getSuiteExecutionMetadataFlag(req, "ants_work_unit_id"),
	}
	log.Printf("publishMetadata %+v", publishMetadata)
	publishRequestMetadata := &anypb.Any{}

	if err := publishRequestMetadata.MarshalFrom(publishMetadata); err != nil {
		log.Printf("Failed to marshal request, %s", err)
	}

	dynamicDeps := []*testapi.DynamicDep{
		{
			Key:   dynamic_common.ServiceAddress,
			Value: antsContainerBuilder.ContainerId,
		},
		{
			Key:   "publishRequest.metadata.accountId",
			Value: "account-id",
		},
		{
			Key:   "publishRequest.testResponse",
			Value: "cros-test_runTests",
		},
	}
	dynamicIdentifier := "ants-publish"

	generator := generators.NewInsertGenerator()
	generator.AddInsertion(
		&testapi.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: []*testapi.ContainerRequest{
				antsContainerBuilder.Build(),
			},
			Task: &testapi.CrosTestRunnerDynamicRequest_Task_Publish{
				Publish: &testapi.PublishTask{
					ServiceAddress: &labapi.IpEndpoint{},
					PublishRequest: &testapi.PublishRequest{
						Metadata: publishRequestMetadata,
					},
					DynamicDeps:       dynamicDeps,
					DynamicIdentifier: dynamicIdentifier,
				},
			},
			Required: true,
		},
		dynamic_common.AppendTaskWrapper(dynamic_common.FindLast(testapi.FocalTaskFinder_PUBLISH)))

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying provision request, %s", err)
	}
}

func getSuiteExecutionMetadataFlag(req *testapi.InternalTestplan, flag string) string {
	for _, arg := range req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs() {
		if strings.EqualFold(arg.GetFlag(), flag) {
			return arg.GetValue()
		}
	}
	return ""
}
