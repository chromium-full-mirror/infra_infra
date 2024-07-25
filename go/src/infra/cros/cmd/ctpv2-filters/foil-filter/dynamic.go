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
	return nil
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
	containers = append(containers, provisionContainerBuilder.Build())
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

	err := dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying test request, %s", err)
	}
}
