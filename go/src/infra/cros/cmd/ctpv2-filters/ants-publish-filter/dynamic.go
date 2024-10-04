// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"

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

	publishMetadata, _ := anypb.New(&metadata.PublishAntsMetadata{})
	dynamicDeps := []*testapi.DynamicDep{
		{
			Key:   dynamic_common.ServiceAddress,
			Value: antsContainerBuilder.ContainerId,
		},
		{
			Key:   "publishRequest.metadata.antsInvocationId",
			Value: "ants-invocation-id",
		},
		{
			Key:   "publishRequest.metadata.parentWorkUnitId",
			Value: "parent-workunit-id",
		},
		{
			Key:   "publishRequest.metadata.accountId",
			Value: "account-id",
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
						Metadata: publishMetadata,
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
