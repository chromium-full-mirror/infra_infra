// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

type GenericTaskGenerator struct {
	// Inputs
	TaskId             string
	ContainerId        string
	ContainerBuilders  []*builders.ContainerBuilder
	UpdateInstructions map[string]interfaces.InsertInstructionGetter
	Start              *interfaces.GenericTaskMessageRequest
	Run                *interfaces.GenericTaskMessageRequest
	Stop               *interfaces.GenericTaskMessageRequest

	// Output
	DynamicUpdates []*api.UserDefinedDynamicUpdate
}

// NewGenericTaskGenerator initializes the generator
// for user containers.
func NewGenericTaskGenerator(
	taskId, containerId string,
	containerBuilders []*builders.ContainerBuilder,
	updateInstructions map[string]interfaces.InsertInstructionGetter,
	start, run, stop *interfaces.GenericTaskMessageRequest) *GenericTaskGenerator {

	return &GenericTaskGenerator{
		TaskId:             taskId,
		ContainerId:        containerId,
		ContainerBuilders:  containerBuilders,
		UpdateInstructions: updateInstructions,
		Start:              start,
		Run:                run,
		Stop:               stop,
		DynamicUpdates:     []*api.UserDefinedDynamicUpdate{},
	}
}

// Generate creates the start, run, and stop requests
// bundling off of the insertionId.
func (gen *GenericTaskGenerator) Generate() ([]*api.UserDefinedDynamicUpdate, error) {
	taskBuilder := builders.NewGenericTaskBuilder(gen.TaskId, gen.ContainerId)
	for _, containerBuilder := range gen.ContainerBuilders {
		taskBuilder.AddContainerRequest(containerBuilder.Build())
	}
	taskBuilder.AddStartRequest(gen.Start)
	if gen.Run.InsertionId != gen.Start.InsertionId {
		gen.addDynamicUpdate(gen.Start.InsertionId, taskBuilder.Task)
		taskBuilder = builders.NewGenericTaskBuilder(gen.TaskId, gen.ContainerId)
	}
	taskBuilder.AddRunRequest(gen.Run)
	if gen.Stop.InsertionId != gen.Run.InsertionId {
		gen.addDynamicUpdate(gen.Run.InsertionId, taskBuilder.Task)
		taskBuilder = builders.NewGenericTaskBuilder(gen.TaskId, gen.ContainerId)
	}
	taskBuilder.AddStopRequest(gen.Stop)
	gen.addDynamicUpdate(gen.Stop.InsertionId, taskBuilder.Task)
	return gen.DynamicUpdates, nil
}

// addDynamicUpdate grabs the finder and updater based on the insertionId,
// and adds an DynamicUpdates to the generator's statekeeper.
func (gen *GenericTaskGenerator) addDynamicUpdate(insertionId string, task *api.CrosTestRunnerDynamicRequest_Task) {
	gen.DynamicUpdates = append(gen.DynamicUpdates, gen.UpdateInstructions[insertionId](task))
}
