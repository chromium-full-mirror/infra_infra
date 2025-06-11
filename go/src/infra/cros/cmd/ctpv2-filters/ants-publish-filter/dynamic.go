// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"strconv"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
)

const (
	artifactsDir      = "/tmp/ants-publish"
	runCmd            = "ants-publish server -port 0"
	containerID       = "ants-publish"
	internalAccountID = 1
	dynamicIdentifier = "ants-publish"
	skipTFUploadFlag  = "skip_ants_upload"
	alRunKey          = "is_al_run"
	partnerRunKey     = "is_partner_run"
)

// isPartnerRun  parses a string to determine if it represents a partner run.
// It returns true only if the string is a valid representation of "true".
func isPartnerRun(partnerRun string, log *log.Logger) bool {
	isPartner, err := strconv.ParseBool(partnerRun)
	if err != nil {
		log.Printf("Error converting partner run value '%s' to bool: %v. Defaulting to false.", partnerRun, err)
		return false // Explicitly return false on error
	}

	if isPartner {
		log.Println("External partner run detected.")
	}

	return isPartner
}

// updateTFUploadState determines whether the Tradefed AnTS upload should be
// skipped and updates the test plan's execution metadata accordingly.
//
// The function checks if the upload feature is enabled and whether the
// run is for a partner. The upload is skipped if the feature is disabled or if
// it is a partner run.
func updateTFUploadState(req *api.InternalTestplan, log *log.Logger, publishMetadata *metadata.PublishAntsMetadata) {
	isPartner := suiteExecutionMetadataArgValue(req, partnerRunKey)
	// Skips the upload for partner runs, as they are not supported.
	skipUpload := !publishMetadata.IsTfPluginEnabled || isPartnerRun(isPartner, log)

	if isPartnerRun(isPartner, log) {
		// Update TFPluginEnabled so that ants-publish is used upload test results and artifacts.
		publishMetadata.IsTfPluginEnabled = false
		log.Printf("Partner run detected; Tradefed AnTS upload is not supported and will be disabled.")
	}
	log.Printf("Tradefed AnTS upload enabled: %t. Skip upload flag set to: %t.", publishMetadata.IsTfPluginEnabled, skipUpload)

	metadata := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata()

	if metadata != nil {
		metadata.Args = append(metadata.GetArgs(), &api.Arg{
			Flag:  skipTFUploadFlag,
			Value: strconv.FormatBool(skipUpload),
		})
	} else {
		log.Printf("Execution metadata is nil, skipping update of skipTFUploadFlag.")
	}
}

func GeneratePublishTask(req *api.InternalTestplan, metadata *metadata.PublishAntsMetadata, publishPath string, log *log.Logger) error {
	alRunValue := suiteExecutionMetadataArgValue(req, alRunKey)
	var alRun bool
	var err error
	if alRunValue != "" {
		alRun, err = strconv.ParseBool(alRunValue)
		if err != nil {
			log.Printf("Unable to get value of %s due to: %q", alRunKey, err)
			return err
		}
	}

	if !alRun {
		log.Printf("Skipping ants-publish task.")

		// Since this is non-AL run, return without doing anything.
		return nil
	}

	// updateTFUploadState determines whether the Tradefed AnTS upload should be enabled.
	updateTFUploadState(req, log, metadata)

	antsContainerBuilder := builders.NewContainerBuilder(
		containerID,  //  ContainerID
		"",           //  ContainerImageKey
		publishPath,  //  Container ImagePath
		artifactsDir, //  ContainerArtifactDir
		runCmd,
	)

	//  Add test artifacts directory for container.
	antsContainerBuilder.DynamicDeps = append(antsContainerBuilder.DynamicDeps,
		&api.DynamicDep{
			Key:   "generic.additionalVolumes",
			Value: "FMT=${env-TEMPDIR}:/tmp/artifacts",
		},
		&api.DynamicDep{
			Key:   "generic.env.0",
			Value: "FMT=GCE_METADATA_HOST=${env-GCE_METADATA_HOST}",
		},
		&api.DynamicDep{
			Key:   "generic.env",
			Value: "FMT=GCE_METADATA_IP=${env-GCE_METADATA_IP}",
		},
		&api.DynamicDep{
			Key:   "generic.env",
			Value: "FMT=GCE_METADATA_ROOT=${env-GCE_METADATA_ROOT}",
		},
	)

	log.Printf("publishMetadata: %+v", metadata)
	publishRequestMetadata := &anypb.Any{}
	if err := publishRequestMetadata.MarshalFrom(metadata); err != nil {
		log.Printf("Failed to marshal request, %s", err)
	}
	log.Printf("publishRequestMetadata: %+v", publishRequestMetadata)

	dynamicDepsDefinition := defineDynamicDeps(antsContainerBuilder)
	log.Printf("dynamicDepsDefinition: %+v", dynamicDepsDefinition)

	// Setup an env array with a default value to init it correctly.
	antsContainer := antsContainerBuilder.Build()
	antsContainer.GetContainer().GetGeneric().Env = []string{"to_be_replaced"}

	generator := generators.NewInsertGenerator()
	generator.AddInsertion(
		&api.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: []*api.ContainerRequest{
				antsContainer,
			},
			Task: &api.CrosTestRunnerDynamicRequest_Task_Publish{
				Publish: &api.PublishTask{
					ServiceAddress: &labapi.IpEndpoint{},
					PublishRequest: &api.PublishRequest{
						Metadata: publishRequestMetadata,
					},
					DynamicDeps:       dynamicDepsDefinition,
					DynamicIdentifier: dynamicIdentifier,
				},
			},
			Required: true,
		},
		dynamic_common.AppendTaskWrapper(dynamic_common.FindFirst(api.FocalTaskFinder_PUBLISH)))

	// The dynamic updates are passed by reference and need the full path.
	// Do not use another var or substitution here.
	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}

func defineDynamicDeps(antsContainerBuilder *builders.ContainerBuilder) []*api.DynamicDep {
	dynamicDeps := []*api.DynamicDep{
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
		{
			Key:   "publishRequest.metadata.primaryExecutionInfo.skylabInfo.buildbucketInfo.ancestorIds.0",
			Value: "parentBBID",
		},
		{
			Key:   "publishRequest.metadata.antsInvocationId",
			Value: "ants_invocation_id",
		},
		{
			Key:   "publishRequest.metadata.parentWorkUnitId",
			Value: "parent_work_unit_id",
		},
		{
			Key:   "publishRequest.metadata.atpEnvironment",
			Value: "env",
		},
		{
			Key:   "publishRequest.metadata.schedulingMetadata.schedulingArgs",
			Value: "schedulingMetadata.schedulingArgs",
		},
	}
	return dynamicDeps
}
