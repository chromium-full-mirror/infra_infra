// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonbuilders_test

import (
	"context"
	"fmt"
	"testing"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	tpcommon "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/testing/citest"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	builders "go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
)

func TestCrosTestRunnerRequestBuilder(t *testing.T) {
	// This test reaches out over the network to grab information about filters applied by default.
	// It is thus an itnegration test.
	// Run it by setting CROS_TEST_RUNNER_INTEGRATION_TESTS or INTEGRATION_TESTS.
	citest.IntegrationTest(t, "CROS_TEST_RUNNER_INTEGRATION_TESTS")
	ftt.Run("Empty CftTestRequest All Skipped", t, func(t *ftt.Test) {
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
		}).BuildRequest(context.Background(), false, false, nil, nil)

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

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, request.GetOrderedTasks(), should.HaveLength(0))
		assert.Loosely(t, request.GetStartRequest(), should.Match(expected.GetStartRequest()))
		assert.Loosely(t, request.GetParams(), should.Match(expected.GetParams()))
	})

	ftt.Run("Build Params and StartRequest", t, func(t *ftt.Test) {
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
		}).BuildRequest(context.Background(), false, false, nil, nil)

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

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, request.GetOrderedTasks(), should.HaveLength(0))
		assert.Loosely(t, request.GetStartRequest(), should.Match(expected.GetStartRequest()))
		assert.Loosely(t, request.GetParams(), should.Match(expected.GetParams()))
	})

	ftt.Run("Builds Tasks", t, func(t *ftt.Test) {
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
		}).BuildRequest(context.Background(), false, false, nil, nil)

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

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, request.GetOrderedTasks(), should.HaveLength(6))
		assert.Loosely(t, request.GetStartRequest(), should.Match(expected.GetStartRequest()))
		assert.Loosely(t, request.GetParams(), should.Match(expected.GetParams()))
	})

	ftt.Run("Builds Tasks with Companions", t, func(t *ftt.Test) {
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
		}).BuildRequest(context.Background(), false, false, nil, nil)

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

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, request.GetOrderedTasks(), should.HaveLength(12))
		assert.Loosely(t, request.GetStartRequest(), should.Match(expected.GetStartRequest()))
		assert.Loosely(t, request.GetParams().GetTestSuites(), should.Match(expected.GetParams().GetTestSuites()))
		assert.Loosely(t, request.GetParams().GetKeyvals(), should.Match(expected.GetParams().GetKeyvals()))
		assert.Loosely(t, request.GetParams().GetPrimaryDut(), should.Match(expected.GetParams().GetPrimaryDut()))
		assert.Loosely(t, request.GetParams().GetCompanionDuts(), should.Match(expected.GetParams().GetCompanionDuts()))
		assert.Loosely(t, request.GetParams().GetContainerMetadata().GetContainers()["default"].GetImages()["cros-fw-provision"].GetDigest(), should.Equal(fmt.Sprintf("sha256:%s", common.DefaultCrosFwProvisionSha)))
	})
}
