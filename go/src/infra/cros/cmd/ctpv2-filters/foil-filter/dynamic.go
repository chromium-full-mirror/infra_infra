// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	dynamic_common "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
)

func GenerateDynamicUpdates(req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) error {
	modifyTestRequest(req, updater, log)
	modifyPublishPath(common.RdbPublish, req, updater, log)
	modifyPublishPath(common.GcsPublish, req, updater, log)
	modifyPublishPath(common.CpconPublish, req, updater, log)
	filterOutFaultyTests(req, updater, log)
	removePostProcess(req, log)
	modifyRdbPublishRequest(req, log)
	modifyGCSPublishRequest(req, updater.EnableXtsArchiver, log)
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
	generator.AddModification(
		&api.DynamicDep{

			Key:   "testRequest.testSuites.0.executionMetadata.args",
			Value: `JSON={"flag":"attempt_wu_id","value":"${schedulingMetadata.schedulingArgs.attempt_wu_id}"}`,
		},
		map[string]string{
			"test.dynamicDeps": "",
		},
	)
	generator.AddModification(
		&api.DynamicDep{

			Key:   "testRequest.testSuites.0.executionMetadata.args",
			Value: `JSON={"flag":"account_id","value":"${account-id}"}`,
		},
		map[string]string{
			"test.dynamicDeps": "",
		},
	)

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying test request, %s", err)
	}
}

func modifyPublishPath(publishType string, req *api.InternalTestplan, updater *FoilRequestUpdater, log *log.Logger) {
	var publishPath string
	switch publishType {
	case common.RdbPublish:
		publishPath = updater.RdbPublishPath
	case common.GcsPublish:
		publishPath = updater.GcsPublishPath
	case common.CpconPublish:
		publishPath = updater.CpconPublishPath
	default:
		return
	}

	if publishPath == "" {
		return
	}

	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(publishType))
	generator.AddModification(
		structpb.NewStringValue(publishPath),
		map[string]string{
			"orderedContainerRequests.0.containerImagePath": "value",
		},
	)

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying test request, %s", err)
	}
}

func modifyRdbPublishRequest(req *api.InternalTestplan, log *log.Logger) {
	log.Println("Modifying rdb publish request")

	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(common.RdbPublish))
	generator.AddModification(
		&api.DynamicDep{
			Key:   "publishRequest.metadata.testResult.testInvocation.primaryExecutionInfo.buildInfo.name",
			Value: fmt.Sprintf("FMT=%s-trunk_staging-userdebug/%s", dynamic_common.SetPlaceholder("board"), dynamic_common.SetPlaceholder("buildNumber")),
		},
		map[string]string{
			"publish.dynamicDeps": "",
		},
	)
	// Remove sources from metadata, not currently supported
	// on android gcs paths.
	generator.AddModification(
		&api.DynamicDep{
			Key:   "publishRequest.metadata.sources",
			Value: "NIL",
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

// modifyGCSPublishRequest modifies the GCS publish dynamic request to set
// the enableXtsArchiver flag in the metadata.
func modifyGCSPublishRequest(req *api.InternalTestplan, enableXtsArchiver bool, log *log.Logger) {
	if !enableXtsArchiver {
		return
	}

	log.Println("Modifying GCS publish request")
	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(common.GcsPublish))
	err := generator.AddModification(
		&api.DynamicDep{
			Key:   "publishRequest.metadata.enableXtsArchiver",
			Value: "BOOL=true",
		},
		map[string]string{
			"publish.dynamicDeps": "",
		},
	)
	if err != nil {
		log.Printf("Error while generating modifications to GCS publish request. %s", err)
		return
	}

	err = dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying GCS publish request. %s", err)
	}
}

var faultTestCases = map[string]struct{}{
	"tradefed.CtsDevicePolicyTestCases":   {},
	"tradefed.CtsJobSchedulerTestCases":   {},
	"tradefed.CtsStatsdAtomHostTestCases": {},
	"tradefed.CtsDeqpTestCases":           {},
	"tradefed.CtsInputMethodTestCases":    {},
	"tradefed.CtsUsageStatsTestCases":     {},
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
