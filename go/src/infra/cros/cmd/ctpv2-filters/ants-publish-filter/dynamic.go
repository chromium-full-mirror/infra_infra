// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"strconv"

	"google.golang.org/protobuf/types/known/anypb"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/generators"
)

const (
	artifactsDir      = "/tmp/ants-publish"
	runCmd            = "ants-publish server -port 0"
	containerID       = "ants-publish"
	internalAccountID = 1
)

func isInternal(accountID string) bool {
	// Sometimes, we do not have accountId for internal users.
	if accountID == "" {
		return true
	}

	id, err := strconv.Atoi(accountID)
	if err != nil {
		log.Printf("Cannot convert account %s to int", accountID)
		return false
	}

	if id < 1 {
		log.Printf("Account ID %s not supported", accountID)
		return false
	}

	return id == internalAccountID
}

func GeneratePublishTask(req *testapi.InternalTestplan, metadata *metadata.PublishAntsMetadata, publishPath string, log *log.Logger) error {
	if !isInternal(metadata.AccountId) {
		// Skip calling ants-publish for external partners.
		log.Printf("Skipping ants-publish task for external partners. Found accountId: %s", metadata.AccountId)
		return nil
	}

	if metadata.AntsInvocationId == "" {
		// Skip if the invocation or parent workunit do not exist.
		log.Printf("Skipping ants-publish task, AntsInvocationId is not populated")
		return nil
	}

	if metadata.ParentWorkUnitId == "" {
		// Skip if the invocation or parent workunit do not exist.
		log.Printf("Skipping ants-publish task, ParentWorkUnitId is not populated")
		return nil
	}

	antsContainerBuilder := builders.NewContainerBuilder(
		containerID,  //  ContainerID
		"",           //  ContainerImageKey
		publishPath,  //  Container ImagePath
		artifactsDir, //  ContainerArtifactDir
		runCmd,
	)

	//  Add test artifacts directory for container.
	antsContainerBuilder.DynamicDeps = append(antsContainerBuilder.DynamicDeps,
		&testapi.DynamicDep{
			Key:   "generic.additionalVolumes",
			Value: "FMT=${env-TEMPDIR}:/tmp/artifacts",
		},
	)

	log.Printf("publishMetadata %+v", metadata)
	publishRequestMetadata := &anypb.Any{}
	if err := publishRequestMetadata.MarshalFrom(metadata); err != nil {
		log.Printf("Failed to marshal request, %s", err)
	}
	log.Printf("publishRequestMetadata: %+v", publishRequestMetadata)

	dynamicDepsDefinition := defineDynamicDeps(antsContainerBuilder)
	log.Printf("dynamicDepsDefinition: %+v", dynamicDepsDefinition)
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
					DynamicDeps:       dynamicDepsDefinition,
					DynamicIdentifier: dynamicIdentifier,
				},
			},
			Required: true,
		},
		dynamic_common.AppendTaskWrapper(dynamic_common.FindLast(testapi.FocalTaskFinder_PUBLISH)))

	// The dynamic updates are passed by reference and need the full path.
	// Do not use another var or substitution here.
	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}

func defineDynamicDeps(antsContainerBuilder *builders.ContainerBuilder) []*testapi.DynamicDep {
	dynamicDeps := []*testapi.DynamicDep{
		{
			Key:   dynamic_common.ServiceAddress,
			Value: antsContainerBuilder.ContainerId,
		},
		{
			Key:   "publishRequest.testResponse",
			Value: "cros-test_runTests",
		},
		{
			Key:   "publishRequest.metadata.accountId",
			Value: "account-id",
		},
		{
			Key:   "publishRequest.metadata.luciInvocationId",
			Value: "invocation-id",
		},
		{
			Key:   "publishRequest.metadata.primaryExecutionInfo.dutInfo.dut",
			Value: "device_primary.dut",
		},
	}
	return dynamicDeps
}
