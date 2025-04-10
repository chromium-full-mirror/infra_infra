// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

type ProvisionTaskGenerator struct {
	// Inputs
	TaskId            string
	ContainerId       string
	DeviceId          string
	ContainerBuilders []*builders.ContainerBuilder
	UpdateInstruction interfaces.InsertInstructionGetter
	StartUp           *interfaces.ProvisionTaskStartUpRequest
	Install           *interfaces.ProvisionTaskInstallRequest

	// Output
	DynamicUpdates []*api.UserDefinedDynamicUpdate
}

// NewProvisionTaskGenerator initializes the generator
// for user containers.
func NewProvisionTaskGenerator(
	taskId, containerId, deviceId string,
	containerBuilders []*builders.ContainerBuilder,
	updateInstruction interfaces.InsertInstructionGetter,
	startup *interfaces.ProvisionTaskStartUpRequest,
	install *interfaces.ProvisionTaskInstallRequest) *ProvisionTaskGenerator {

	return &ProvisionTaskGenerator{
		TaskId:            taskId,
		ContainerId:       containerId,
		DeviceId:          deviceId,
		ContainerBuilders: containerBuilders,
		UpdateInstruction: updateInstruction,
		StartUp:           startup,
		Install:           install,
		DynamicUpdates:    []*api.UserDefinedDynamicUpdate{},
	}
}

// Generate creates the start, run, and stop requests
// bundling off of the insertionId.
func (gen *ProvisionTaskGenerator) Generate() ([]*api.UserDefinedDynamicUpdate, error) {
	taskBuilder := builders.NewProvisionTaskBuilder(gen.TaskId, gen.ContainerId, gen.DeviceId)
	for _, containerBuilder := range gen.ContainerBuilders {
		taskBuilder.AddContainerRequest(containerBuilder.Build())
	}
	taskBuilder.AddStartUpRequest(gen.StartUp)
	taskBuilder.AddInstallRequest(gen.Install)
	gen.addDynamicUpdate(taskBuilder.Task)
	return gen.DynamicUpdates, nil
}

// addDynamicUpdate grabs the finder and updater based on the insertionId,
// and adds an DynamicUpdates to the generator's statekeeper.
func (gen *ProvisionTaskGenerator) addDynamicUpdate(task *api.CrosTestRunnerDynamicRequest_Task) {
	gen.DynamicUpdates = append(gen.DynamicUpdates, gen.UpdateInstruction(task))
}
