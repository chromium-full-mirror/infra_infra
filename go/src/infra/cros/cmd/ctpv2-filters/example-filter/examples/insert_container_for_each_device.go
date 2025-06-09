// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package examples

import (
	"fmt"
	"log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	dynamiccommon "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
	dynamichelpers "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
)

// InsertContainerForEachDevice will insert a generic container for each chromeos/AL device.
//
// For each device within the first scheduling unit, create a new test runner task and
// prepend it to the beginning of the task list.
func InsertContainerForEachDevice(req *api.InternalTestplan) {
	targetOptions := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnitOptions()
	if len(targetOptions) == 0 || len(targetOptions[0].GetSchedulingUnits()) == 0 {
		return
	}
	// Select the first scheduling unit.
	// Assumption is that each scheduling unit option has the same number of targets,
	// primary and companions.
	firstSchedulingUnit := targetOptions[0].GetSchedulingUnits()[0]

	chromeOs := 0
	for _, target := range append(
		[]*api.Target{firstSchedulingUnit.GetPrimaryTarget()},
		firstSchedulingUnit.GetCompanionTargets()...) {

		swarmingDef := target.GetSwarmingDef()
		// Only produce container requests on chromeos devices.
		// FYI: AL devices are still marked as Dut_Chromeos.
		switch swarmingDef.GetDutInfo().GetDutType().(type) {
		case *labapi.Dut_Chromeos:
			InsertContainerForChromeos(req, &chromeOs)
		}
	}
}

func InsertContainerForChromeos(req *api.InternalTestplan, count *int) {
	defer func() {
		*count++
	}()
	container := CreateContainerForChromeos(*count)
	if container == nil {
		return
	}

	startRequestProtos := map[string]proto.Message{
		// Placeholder values such as board are resolved during CTP's execution
		// before sending the request off to test runner.
		"board": structpb.NewStringValue(dynamichelpers.Board.WithIndex(*count).AsPlaceholder()),
	}
	startRequestValues, err := common.ProtoMapToAnyMap(startRequestProtos)
	if err != nil {
		log.Println("failed to convert start request's proto map to any map: ", err)
		return
	}

	// Add the container to the beginning of the test runner execution.
	containersTask := &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{container},
		// Provide a Generic Task with a dynamic identifier to ensure
		// it is findable by other filters.
		// Example: If VMs cannot run this filter, then VMs need to set
		// a secondary dynamic update that removes this task for VM targets,
		// and this requires knowing the dynamic identifier of this task.
		Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{
			Generic: &api.GenericTask{
				DynamicIdentifier: "inserted_containers",
				StartRequest: &api.GenericStartRequest{
					Message: &api.GenericMessage{
						Values: startRequestValues,
					},
				},
				// Some values cannot be passed in during the filter's execution
				// because they are not concrete until test runner's execution.
				// These values must be resolved during test runner execution and
				// that resolution is defined as Dynamic Dependencies.
				DynamicDeps: []*api.DynamicDep{
					{
						// In the StartRequest, set the value of `dut_topology` in the map
						// to be the value of the `dutTopology` stored by test runner.
						Key: "startRequest.message.values.dut_topology",
						// Because the generic service operates on type `Any`, the `typeUrl` must be predefined.
						// This can be achieved programmatically by using anypb.New(&labapi.DutTopology),
						// which will provide an object that has the typeUrl already defined.
						Value: "ANY(type.googleapis.com/chromiumos.test.lab.api.DutTopology)=dutTopology",
					},
				},
			},
		},
		Required: true,
	}

	insertAt := dynamiccommon.PrependTaskWrapper(dynamiccommon.FindBeginning())

	generator := generators.NewInsertGenerator()
	generator.AddInsertion(containersTask, insertAt)
}

func CreateContainerForChromeos(count int) *api.ContainerRequest {
	deviceIdentifer := dynamiccommon.NewPrimaryDeviceIdentifier()
	if count > 0 {
		deviceIdentifer = dynamiccommon.NewCompanionDeviceIdentifier(dynamichelpers.Board.WithIndex(count).AsPlaceholder())
	}

	containerId := dynamiccommon.NewTaskIdentifier("inserted_container").AddDeviceId(deviceIdentifer).Id
	containerBuilder := builders.NewContainerBuilder(
		containerId,                         // ContainerID
		"",                                  // ContainerImageKey
		"path-to-pull-container",            // Container ImagePath
		fmt.Sprintf("/tmp/%s", containerId), // ContainerArtifactDir
		"binaryName server -port 0",         // Cmd
	)
	return containerBuilder.Build()
}
