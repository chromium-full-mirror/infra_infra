// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	dynamiccommon "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
	dynamichelpers "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
)

// GenerateDynamicInfo creates dynamic updates for provision
// requests, and adds their relevant information to each
// scheduling unit's dynamic lookup table.
func GenerateDynamicInfo(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) error {
	// Create Dynamic Updates.
	if err := generateLSNexusRequest(req, log, commonParams); err != nil {
		return err
	}

	return nil
}

// generateLSNexusRequest creates the primary and companion cros-provision
// requests and sets the relevant placeholders.
func generateLSNexusRequest(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (err error) {
	var schedulingUnits []*api.SchedulingUnit
	targetOptions := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnitOptions()
	if len(targetOptions) > 0 {
		schedulingUnits = targetOptions[0].GetSchedulingUnits()
	} else {
		schedulingUnits = req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
	}
	if len(schedulingUnits) == 0 {
		log.Println("No scheduling units found, skipping lsnexus-filter")
		return
	}
	// Select the first scheduling unit.
	// Assumption is that each scheduling unit option has the same number of targets,
	// primary and companions.
	firstSchedulingUnit := schedulingUnits[0]

	count := 0
	for _, target := range append(
		[]*api.Target{firstSchedulingUnit.GetPrimaryTarget()},
		firstSchedulingUnit.GetCompanionTargets()...) {

		swarmingDef := target.GetSwarmingDef()
		// Only produce container requests on chromeos devices.
		// FYI: AL devices are still marked as Dut_Chromeos.
		switch swarmingDef.GetDutInfo().GetDutType().(type) {
		case *labapi.Dut_Chromeos:
			insertContainerForChromeos(req, &count, log, commonParams)
		}
	}
	return nil
}

func insertContainerForChromeos(req *api.InternalTestplan,
	count *int, log *log.Logger, commonParams *common.CommonFilterParams) {
	defer func() {
		*count++
	}()
	container := createContainerForDevice(*count, log, commonParams)
	if container == nil {
		return
	}

	boardValue := structpb.NewStringValue(dynamichelpers.Board.WithIndex(*count).AsPlaceholder())

	device := "device_primary"
	if *count > 0 {
		device = "device_companion_" + boardValue.GetStringValue()
	}
	device = fmt.Sprintf("%s.dut", device)

	dutAsAny, _ := anypb.New(&labapi.Dut{})

	// Add the container to the beginning of the test runner execution.
	containersStartTask := &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{container},
		// Provide a Generic Task with a dynamic identifier to ensure
		// it is findable by other filters.
		// Example: If VMs cannot run this filter, then VMs need to set
		// a secondary dynamic update that removes this task for VM targets,
		// and this requires knowing the dynamic identifier of this task.
		Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{
			Generic: &api.GenericTask{
				DynamicIdentifier: container.GetDynamicIdentifier() + "_task",
				StartRequest: &api.GenericStartRequest{
					Message: &api.GenericMessage{
						Values: map[string]*anypb.Any{
							"ignoredValue": nil,
						},
					},
				},
				// Some values cannot be passed in during the filter's execution
				// because they are not concrete until test runner's execution.
				// These values must be resolved during test runner execution and
				// that resolution is defined as Dynamic Dependencies.
				DynamicDeps: []*api.DynamicDep{
					{
						// In the StartRequest, set the value of `dut` in the map
						// to be the value of the `dut` stored by test runner.
						Key: "startRequest.message.values.dut",
						// Because the generic service operates on type `Any`, the `typeUrl` must be predefined.
						// This can be achieved programmatically by using anypb.New(&labapi.DutTopology),
						// which will provide an object that has the typeUrl already defined.
						Value: fmt.Sprintf("ANY(%s)=%s", dutAsAny.TypeUrl, device),
					},
					{
						Key:   "serviceAddress",
						Value: container.GetDynamicIdentifier(),
					},
				},
			},
		},
		Required: true,
	}

	// Add the container to the end of the test runner execution to save log.
	containersEndTask := &api.CrosTestRunnerDynamicRequest_Task{
		// Provide a Generic Task with a dynamic identifier to ensure
		// it is findable by other filters.
		// Example: If VMs cannot run this filter, then VMs need to set
		// a secondary dynamic update that removes this task for VM targets,
		// and this requires knowing the dynamic identifier of this task.
		Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{
			Generic: &api.GenericTask{
				DynamicIdentifier: container.GetDynamicIdentifier() + "_task",
				StopRequest: &api.GenericStopRequest{
					Message: &api.GenericMessage{
						Values: map[string]*anypb.Any{
							"ignoredValue": nil,
						},
					},
				},
				// Some values cannot be passed in during the filter's execution
				// because they are not concrete until test runner's execution.
				// These values must be resolved during test runner execution and
				// that resolution is defined as Dynamic Dependencies.
				DynamicDeps: []*api.DynamicDep{
					{
						Key:   "serviceAddress",
						Value: container.GetDynamicIdentifier(),
					},
				},
			},
		},
		Required: true,
	}

	insertAtBeginning := dynamiccommon.PrependTaskWrapper(dynamiccommon.FindBeginning())
	insertAtEnd := dynamiccommon.PrependTaskWrapper(dynamiccommon.FindFirst(api.FocalTaskFinder_PUBLISH))

	generator := generators.NewInsertGenerator()
	generator.AddInsertion(containersStartTask, insertAtBeginning)
	generator.AddInsertion(containersEndTask, insertAtEnd)

	dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}

// createContainerForDevice will create a container for Chromeos devices.
// The container will be marked with the identifier of the device it corresponds to.
func createContainerForDevice(count int, log *log.Logger, commonParams *common.CommonFilterParams) *api.ContainerRequest {
	deviceIdentifer := dynamiccommon.NewPrimaryDeviceIdentifier()
	if count > 0 {
		deviceIdentifer = dynamiccommon.NewCompanionDeviceIdentifier(dynamichelpers.Board.WithIndex(count).AsPlaceholder())
	}

	imagePath, err := common.ProcessContainerPath(context.Background(), commonParams, "", "lsnexus")
	if err != nil {
		log.Println("Failed to process container path for lsnexus: ", err)
		return nil
	}

	containerId := dynamiccommon.NewTaskIdentifier("lsnexus").AddDeviceId(deviceIdentifer).Id
	const (
		lsnexus            = "lsnexus"
		lsnexusArtifactDir = "/tmp/lsnexus"
		lsnexusArgs        = "server -port 0"
	)
	containerBuilder := builders.NewContainerBuilder(
		containerId, "", imagePath,
		lsnexusArtifactDir,
		fmt.Sprintf("%s %s", lsnexus, lsnexusArgs),
	)
	return containerBuilder.Build()
}
