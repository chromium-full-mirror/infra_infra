// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package builders

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

const (
	GenericDynamicDepFormat = "%sRequest.message.values.%s"
	StartRpc                = "start"
	RunRpc                  = "run"
	StopRpc                 = "stop"
)

// GenericTaskBuilder builds out a GenericTask for ease of use within
// generators designed around generic tasks.
type GenericTaskBuilder struct {
	Task *api.CrosTestRunnerDynamicRequest_Task
}

// NewGenericTaskBuilder creates a GenericTaskBuilder
// based on the UserContainerGenerator.
func NewGenericTaskBuilder(dynamicId, containerId string) *GenericTaskBuilder {
	return &GenericTaskBuilder{
		Task: &api.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: []*api.ContainerRequest{},
			Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{
				Generic: &api.GenericTask{
					ServiceAddress: &labapi.IpEndpoint{},
					DynamicDeps: []*api.DynamicDep{
						{
							Key:   common.ServiceAddress,
							Value: containerId,
						},
					},
					DynamicIdentifier: dynamicId,
				},
			},
		},
	}
}

// AddContainerRequest appends the provided container request to the task's OrderedContainerRequests.
func (builder *GenericTaskBuilder) AddContainerRequest(contReq *api.ContainerRequest) {
	builder.Task.OrderedContainerRequests = append(builder.Task.OrderedContainerRequests, contReq)
}

// AddStartRequest creates the generic start request and generates the dynamic deps.
func (builder *GenericTaskBuilder) AddStartRequest(start *interfaces.GenericTaskMessageRequest) error {
	return builder.addGenericRequest(StartRpc, start)
}

// AddRunRequest creates the generic run request and generates the dynamic deps.
func (builder *GenericTaskBuilder) AddRunRequest(run *interfaces.GenericTaskMessageRequest) error {
	return builder.addGenericRequest(RunRpc, run)
}

// AddStopRequest creates the generic stop request and generates the dynamic deps.
func (builder *GenericTaskBuilder) AddStopRequest(stop *interfaces.GenericTaskMessageRequest) error {
	return builder.addGenericRequest(StopRpc, stop)
}

// addGenericRequest creates the generic request and generates the dynamic deps.
func (builder *GenericTaskBuilder) addGenericRequest(rpc string, req *interfaces.GenericTaskMessageRequest) error {
	genericValues, err := common.ConvertProtoMapToAnyMap(req.StaticInputs)
	if err != nil {
		return errors.Annotate(err, "failed to add stop request's static inputs").Err()
	}

	genericMessage := &api.GenericMessage{
		Values: genericValues,
	}

	switch rpc {
	case StartRpc:
		builder.Task.GetGeneric().StartRequest = &api.GenericStartRequest{
			Message: genericMessage,
		}
	case RunRpc:
		builder.Task.GetGeneric().RunRequest = &api.GenericRunRequest{
			Message: genericMessage,
		}
	case StopRpc:
		builder.Task.GetGeneric().StopRequest = &api.GenericStopRequest{
			Message: genericMessage,
		}
	default:
		return errors.New("unknown rpc for generic task builder")
	}

	taskDeps := builder.Task.GetGeneric().DynamicDeps
	generatedDeps := common.GenerateDynamicDeps(rpc, req.DynamicInputs, GenericDynamicDepFormat)
	builder.Task.GetGeneric().DynamicDeps = append(taskDeps, generatedDeps...)

	return nil
}
