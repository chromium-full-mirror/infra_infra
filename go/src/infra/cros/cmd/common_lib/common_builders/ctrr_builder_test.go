// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_builders_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	tpcommon "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

	"infra/cros/cmd/common_lib/common"
	builders "infra/cros/cmd/common_lib/common_builders"
)

func TestCrosTestRunnerRequestBuilder(t *testing.T) {
	Convey("Empty CftTestRequest All Skipped", t, func() {
		request, err := builders.NewDynamicTrv2FromCftBuilder(&skylab_test_runner.CFTTestRequest{
			StepsConfig: &tpcommon.CftStepsConfig{
				ConfigType: &tpcommon.CftStepsConfig_HwTestConfig{
					HwTestConfig: &tpcommon.HwTestConfig{
						SkipStartingDutService: true,
						SkipProvision:          true,
						SkipTestExecution:      true,
						SkipAllResultPublish:   true,
						SkipPostProcess:        true,
					},
				},
			},
			ContainerMetadata: &buildapi.ContainerMetadata{
				Containers: make(map[string]*buildapi.ContainerImageMap),
			},
		}).BuildRequest(context.Background())

		expected := &api.CrosTestRunnerDynamicRequest{
			StartRequest: &api.CrosTestRunnerDynamicRequest_Build{
				Build: &api.BuildMode{},
			},
			Params: &api.CrosTestRunnerParams{
				ContainerMetadata: &buildapi.ContainerMetadata{
					Containers: make(map[string]*buildapi.ContainerImageMap),
				},
				TestSuites:    []*api.TestSuite{},
				Keyvals:       make(map[string]string),
				CompanionDuts: []*labapi.DutModel{},
			},
		}

		So(err, ShouldBeNil)
		So(request.GetOrderedTasks(), ShouldHaveLength, 0)
		So(request.GetStartRequest(), ShouldResemble, expected.GetStartRequest())
		So(request.GetParams(), ShouldResemble, expected.GetParams())
	})

	Convey("Build Params and StartRequest", t, func() {
		request, err := builders.NewDynamicTrv2FromCftBuilder(&skylab_test_runner.CFTTestRequest{
			ParentRequestUid: "parent",
			PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
				DutModel: &labapi.DutModel{
					BuildTarget: "test-board",
				},
			},
			AutotestKeyvals: map[string]string{
				"fizz": "buzz",
			},
			TestSuites: []*api.TestSuite{
				{
					Name: "test1",
				},
			},
			StepsConfig: &tpcommon.CftStepsConfig{
				ConfigType: &tpcommon.CftStepsConfig_HwTestConfig{
					HwTestConfig: &tpcommon.HwTestConfig{
						SkipStartingDutService: true,
						SkipProvision:          true,
						SkipTestExecution:      true,
						SkipAllResultPublish:   true,
						SkipPostProcess:        true,
					},
				},
			},
			ContainerMetadata: &buildapi.ContainerMetadata{
				Containers: make(map[string]*buildapi.ContainerImageMap),
			},
		}).BuildRequest(context.Background())

		expected := &api.CrosTestRunnerDynamicRequest{
			StartRequest: &api.CrosTestRunnerDynamicRequest_Build{
				Build: &api.BuildMode{
					ParentRequestUid: "parent",
				},
			},
			Params: &api.CrosTestRunnerParams{
				ContainerMetadata: &buildapi.ContainerMetadata{
					Containers: make(map[string]*buildapi.ContainerImageMap),
				},
				TestSuites: []*api.TestSuite{
					{
						Name: "test1",
					},
				},
				Keyvals: map[string]string{
					"fizz": "buzz",
				},
				PrimaryDut: &labapi.DutModel{
					BuildTarget: "test-board",
				},
				CompanionDuts: []*labapi.DutModel{},
			},
		}

		So(err, ShouldBeNil)
		So(request.GetOrderedTasks(), ShouldHaveLength, 0)
		So(request.GetStartRequest(), ShouldResemble, expected.GetStartRequest())
		So(request.GetParams(), ShouldResemble, expected.GetParams())
	})

	Convey("Builds Tasks", t, func() {
		request, err := builders.NewDynamicTrv2FromCftBuilder(&skylab_test_runner.CFTTestRequest{
			ParentRequestUid: "parent",
			PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
				DutModel: &labapi.DutModel{
					BuildTarget: "test-board",
				},
			},
			AutotestKeyvals: map[string]string{
				"fizz": "buzz",
			},
			TestSuites: []*api.TestSuite{
				{
					Name: "test1",
				},
			},
			ContainerMetadata: &buildapi.ContainerMetadata{
				Containers: make(map[string]*buildapi.ContainerImageMap),
			},
		}).BuildRequest(context.Background())

		expected := &api.CrosTestRunnerDynamicRequest{
			StartRequest: &api.CrosTestRunnerDynamicRequest_Build{
				Build: &api.BuildMode{
					ParentRequestUid: "parent",
				},
			},
			Params: &api.CrosTestRunnerParams{
				ContainerMetadata: &buildapi.ContainerMetadata{
					Containers: make(map[string]*buildapi.ContainerImageMap),
				},
				TestSuites: []*api.TestSuite{
					{
						Name: "test1",
					},
				},
				Keyvals: map[string]string{
					"fizz": "buzz",
				},
				PrimaryDut: &labapi.DutModel{
					BuildTarget: "test-board",
				},
				CompanionDuts: []*labapi.DutModel{},
			},
		}

		So(err, ShouldBeNil)
		So(request.GetOrderedTasks(), ShouldHaveLength, 6)
		So(request.GetStartRequest(), ShouldResemble, expected.GetStartRequest())
		So(request.GetParams(), ShouldResemble, expected.GetParams())
	})

	Convey("Builds Tasks with Companions", t, func() {
		request, err := builders.NewDynamicTrv2FromCftBuilder(&skylab_test_runner.CFTTestRequest{
			ParentRequestUid: "parent",
			PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
				DutModel: &labapi.DutModel{
					BuildTarget: "test-board",
				},
			},
			CompanionDuts: []*skylab_test_runner.CFTTestRequest_Device{
				{
					DutModel: &labapi.DutModel{
						BuildTarget: "test-board",
					},
				},
				{
					DutModel: &labapi.DutModel{
						BuildTarget: "test-board",
					},
				},
				{
					DutModel: &labapi.DutModel{
						BuildTarget: "test-board",
					},
				},
			},
			ContainerMetadata: &buildapi.ContainerMetadata{
				Containers: map[string]*buildapi.ContainerImageMap{
					"default": {
						Images: map[string]*buildapi.ContainerImageInfo{
							"cros-fw-provision": common.CreateTestServicesContainer("cros-fw-provision", common.DefaultCrosFwProvisionSha),
						},
					},
				},
			},
			AutotestKeyvals: map[string]string{
				"fizz":  "buzz",
				"build": "Release/R123.0.0-123",
			},
			TestSuites: []*api.TestSuite{
				{
					Name: "test1",
				},
			},
		}).BuildRequest(context.Background())

		expected := &api.CrosTestRunnerDynamicRequest{
			StartRequest: &api.CrosTestRunnerDynamicRequest_Build{
				Build: &api.BuildMode{
					ParentRequestUid: "parent",
				},
			},
			Params: &api.CrosTestRunnerParams{
				ContainerMetadata: &buildapi.ContainerMetadata{
					Containers: map[string]*buildapi.ContainerImageMap{
						"default": {
							Images: map[string]*buildapi.ContainerImageInfo{
								"cros-fw-provision": common.CreateTestServicesContainer("cros-fw-provision", common.DefaultCrosFwProvisionSha),
							},
						},
					},
				},
				TestSuites: []*api.TestSuite{
					{
						Name: "test1",
					},
				},
				Keyvals: map[string]string{
					"fizz":  "buzz",
					"build": "Release/R123.0.0-123",
				},
				PrimaryDut: &labapi.DutModel{
					BuildTarget: "test-board",
				},
				CompanionDuts: []*labapi.DutModel{
					{
						BuildTarget: "test-board",
					},
					{
						BuildTarget: "test-board",
					},
					{
						BuildTarget: "test-board",
					},
				},
			},
		}

		So(err, ShouldBeNil)
		So(request.GetOrderedTasks(), ShouldHaveLength, 12)
		So(request.GetStartRequest(), ShouldResemble, expected.GetStartRequest())
		So(request.GetParams(), ShouldResemble, expected.GetParams())
		So(request.GetParams().GetContainerMetadata().GetContainers()["default"].GetImages()["cros-fw-provision"].GetDigest(), ShouldEqual, fmt.Sprintf("sha256:%s", common.DefaultCrosFwProvisionSha))
	})
}
