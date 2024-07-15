// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_builders

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
)

type DynamicTrv2FromCft struct {
	interfaces.DynamicTRv2Builder

	Cft *skylab_test_runner.CFTTestRequest
}

func NewDynamicTrv2FromCftBuilder(cft *skylab_test_runner.CFTTestRequest) *DynamicTrv2FromCft {
	return &DynamicTrv2FromCft{
		Cft: cft,
	}
}

// BuildRequest extracts necessary information from the cft test request to build out the
// dynamic trv2 request.
func (builder *DynamicTrv2FromCft) BuildRequest(ctx context.Context) (*api.CrosTestRunnerDynamicRequest, error) {
	dynamic := builder.buildDynamicRequest()

	builder.tryAppendProvisionTask(dynamic)
	builder.tryAppendTestTask(dynamic)
	builder.tryAppendPostProcessTask(dynamic)
	builder.tryAppendPublishTasks(dynamic)

	for _, companionDut := range builder.Cft.GetCompanionDuts() {
		dynamic.CompanionDuts = append(dynamic.CompanionDuts, companionDut.GetDutModel())
	}

	return dynamic.BuildRequest(ctx)
}

type DynamicTaskBuilder func(*DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task

type DynamicTrv2Builder struct {
	// Inputs
	ParentBuildId    int64
	ParentRequestUid string
	Deadline         *timestamppb.Timestamp
	// Oneof
	ContainerGcsPath  string
	ContainerMetadata *buildapi.ContainerMetadata
	// End Oneof
	ContainerMetadataKey string
	BuildString          string
	TestSuites           []*api.TestSuite
	PrimaryDut           *labapi.DutModel
	CompanionDuts        []*labapi.DutModel
	Keyvals              map[string]string
	OrderedTaskBuilders  []DynamicTaskBuilder
}

// BuildRequest constructs the trv2 dynamic CrosTestRunnerDynamicRequest.
func (builder *DynamicTrv2Builder) BuildRequest(ctx context.Context) (*api.CrosTestRunnerDynamicRequest, error) {
	if builder.ContainerMetadata == nil {
		if builder.ContainerGcsPath == "" {
			return nil, fmt.Errorf("request missing `ContainerGcsPath`, can't fetch container metadata")
		}
		containerMetadata, err := common.FetchContainerMetadata(ctx, builder.ContainerGcsPath)
		if err != nil {
			logging.Infof(ctx, "error while fetching container metadata: %s", err)
			return nil, err
		}
		builder.ContainerMetadata = containerMetadata
	}

	orderedTasks := []*api.CrosTestRunnerDynamicRequest_Task{}
	for _, taskBuilder := range builder.OrderedTaskBuilders {
		orderedTasks = append(orderedTasks, taskBuilder(builder)...)
	}

	return &api.CrosTestRunnerDynamicRequest{
		StartRequest: builder.buildStartRequest(),
		Params:       builder.buildParams(),
		OrderedTasks: orderedTasks,
	}, nil
}

// buildStartRequest defaults to the BuildMode start request.
func (builder *DynamicTrv2Builder) buildStartRequest() *api.CrosTestRunnerDynamicRequest_Build {
	return &api.CrosTestRunnerDynamicRequest_Build{
		Build: &api.BuildMode{
			ParentBuildId:    builder.ParentBuildId,
			ParentRequestUid: builder.ParentRequestUid,
		},
	}
}

// buildParams constructs the CrosTestRunnerParams.
func (builder *DynamicTrv2Builder) buildParams() *api.CrosTestRunnerParams {
	return &api.CrosTestRunnerParams{
		ContainerMetadata:    PatchContainerMetadata(builder.ContainerMetadata, builder.BuildString),
		ContainerMetadataKey: builder.ContainerMetadataKey,
		Keyvals:              builder.Keyvals,
		PrimaryDut:           builder.PrimaryDut,
		CompanionDuts:        builder.CompanionDuts,
		TestSuites:           builder.TestSuites,
		Deadline:             builder.Deadline,
	}
}
