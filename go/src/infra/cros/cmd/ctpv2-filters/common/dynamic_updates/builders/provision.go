// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package builders

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

const (
	ProvisionDynamicDepFormat = "%sRequest.%s"
)

// ProvisionTaskBuilder builds out a ProvisionTask for ease of use within
// generators designed around generic tasks.
type ProvisionTaskBuilder struct {
	Task *api.CrosTestRunnerDynamicRequest_Task
}

// NewProvisionTaskBuilder creates a ProvisionTaskBuilder
// based on the UserContainerGenerator.
func NewProvisionTaskBuilder(dynamicId, containerId, targetDevice string) *ProvisionTaskBuilder {
	return &ProvisionTaskBuilder{
		Task: &api.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: []*api.ContainerRequest{},
			Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
				Provision: &api.ProvisionTask{
					ServiceAddress: &labapi.IpEndpoint{},
					DynamicDeps: []*api.DynamicDep{
						{
							Key:   common.ServiceAddress,
							Value: containerId,
						},
					},
					DynamicIdentifier: dynamicId,
					Target:            targetDevice,
				},
			},
		},
	}
}

// AddContainerRequest appends the provided container request to the task's OrderedContainerRequests.
func (builder *ProvisionTaskBuilder) AddContainerRequest(contReq *api.ContainerRequest) {
	builder.Task.OrderedContainerRequests = append(builder.Task.OrderedContainerRequests, contReq)
}

// AddStartRequest creates the generic start request and generates the dynamic deps.
func (builder *ProvisionTaskBuilder) AddStartUpRequest(startup *interfaces.ProvisionTaskStartUpRequest) {
	builder.Task.GetProvision().StartupRequest = &api.ProvisionStartupRequest{
		Dut:       startup.StaticDut,
		DutServer: startup.StaticDutServer,
	}
	taskDeps := builder.Task.GetProvision().DynamicDeps
	generatedDeps := common.GenerateDynamicDeps("startup", startup.DynamicInputs, ProvisionDynamicDepFormat)
	builder.Task.GetProvision().DynamicDeps = append(taskDeps, generatedDeps...)
}

// AddRunRequest creates the generic run request and generates the dynamic deps.
func (builder *ProvisionTaskBuilder) AddInstallRequest(install *interfaces.ProvisionTaskInstallRequest) {
	builder.Task.GetProvision().InstallRequest = &api.InstallRequest{
		ImagePath: install.StaticImagePath,
		Metadata:  install.StaticMetadata,
	}
	taskDeps := builder.Task.GetProvision().DynamicDeps
	generatedDeps := common.GenerateDynamicDeps("install", install.DynamicInputs, ProvisionDynamicDepFormat)

	defaultInstallDynamicInputs := map[string]string{
		"partnerMetadata": "partner-metadata",
	}
	generatedDeps = append(generatedDeps, common.GenerateDynamicDeps("install", defaultInstallDynamicInputs, ProvisionDynamicDepFormat)...)

	builder.Task.GetProvision().DynamicDeps = append(taskDeps, generatedDeps...)
}
