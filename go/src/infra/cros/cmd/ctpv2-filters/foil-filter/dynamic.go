// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates"
	dynamic_builders "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/generators"

	"infra/cros/cmd/common_lib/common"
)

func GenerateDynamicUpdates(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) error {
	modifyProvisionRequest(req, updater, log)
	modifyTestRequest(req, updater, log)
	filterOutFaultyTests(req, updater, log)
	removePostProcess(req, log)
	modifyRdbPublishRequest(req, updater, log)
	updateProvisionInstallPath(req, updater, log)
	return nil
}

func removePostProcess(req *api.InternalTestplan, log *log.Logger) {
	generator := generators.NewRemoveGenerator([]*api.FocalTaskFinder{
		dynamic_common.FindByDynamicIdentifier(common.PostProcess)})

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while creating remove update, %s", err)
	}
}

func modifyProvisionRequest(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) {
	if updater.ProvisionPath == "" && updater.ProvisionBinary == "" {
		return
	}

	taskID := dynamic_common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(dynamic_common.NewPrimaryDeviceIdentifier())
	generator := generators.NewModifyGenerator(
		dynamic_common.FindByDynamicIdentifier(
			taskID.Id))

	binary := common.CrosProvision
	if updater.ProvisionBinary != "" {
		binary = updater.ProvisionBinary
	}
	provisionContainerBuilder := dynamic_builders.NewContainerBuilder(
		taskID.Id, common.CrosProvision, updater.ProvisionPath,
		"/tmp/provisionservice", fmt.Sprintf("%s server -port 0", binary))

	containers := []*api.ContainerRequest{}
	container := provisionContainerBuilder.Build()
	container.Network = "adb-network"
	containers = append(containers, container)
	generator.AddModification(
		&api.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: containers,
		},
		map[string]string{
			"orderedContainerRequests": "orderedContainerRequests",
		},
	)

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying provision request, %s", err)
	}
}

func updateProvisionInstallPath(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) {
	req.SuiteInfo.SuiteMetadata.SchedulingUnits[0].DynamicUpdateLookupTable["installPath"] = fmt.Sprintf(
		"android-build/build_explorer/build_details/%s/%s/android-desktop-ota-packages.zip",
		updater.buildNum, updater.buildStr)
}

func modifyTestRequest(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) {
	if updater.TestPath == "" {
		return
	}

	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(common.CrosTest))
	generator.AddModification(
		structpb.NewStringValue(updater.TestPath),
		map[string]string{
			"orderedContainerRequests.0.containerImagePath": "value",
		},
	)
	generator.AddModification(
		structpb.NewStringValue("adb-network"),
		map[string]string{
			"orderedContainerRequests.0.network": "value",
		},
	)

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying test request, %s", err)
	}
}

func modifyRdbPublishRequest(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) {
	log.Println("Modifying rdb publish request")

	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(common.RdbPublish))
	generator.AddModification(
		&api.DynamicDep{
			Key:   "publishRequest.metadata.testResult.testInvocation.primaryExecutionInfo.buildInfo.name",
			Value: fmt.Sprintf("FMT=%s/%s", updater.buildStr, updater.buildNum),
		},
		map[string]string{
			"publish.dynamicDeps": "",
		},
	)
	generator.AddModification(
		&api.DynamicDep{
			Key:   "publishRequest.metadata.sources.gsPath",
			Value: fmt.Sprintf("FMT=%s", req.SuiteInfo.SuiteMetadata.SchedulingUnits[0].DynamicUpdateLookupTable["installPath"]+"/metadata/sources.jsonpb"),
		},
		map[string]string{
			"publish.dynamicDeps": "",
		},
	)

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying rdb publish request. %s", err)
	}
}

var faultTestCases = map[string]struct{}{
	"tradefed.CtsDevicePolicyTestCases":   {},
	"tradefed.CtsJobSchedulerTestCases":   {},
	"tradefed.CtsStatsdAtomHostTestCases": {},
}

func filterOutFaultyTests(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) {
	if !updater.FilterTests {
		log.Println("Skipping test filtering")
		return
	}

	filteredList := []*api.CTPTestCase{}
	for _, testCase := range req.GetTestCases() {
		if _, faulty := faultTestCases[testCase.GetName()]; !faulty {
			filteredList = append(filteredList, testCase)
		} else {
			log.Printf("Filtered out faulty test: %s", testCase.GetName())
		}
	}
	req.TestCases = filteredList
}
