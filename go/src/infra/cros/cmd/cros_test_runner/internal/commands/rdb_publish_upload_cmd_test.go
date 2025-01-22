// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/types/known/timestamppb"

	_go "go.chromium.org/chromiumos/config/go"
	configpb "go.chromium.org/chromiumos/config/go"
	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.99Z", s)
	return t
}

func TestRdbPublishPublishCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosRdbPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestRdbPublishPublishCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosRdbPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestRdbPublishPublishCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosRdbPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishUploadCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestRdbPublishPublishCmd_ExtractSources(t *testing.T) {
	t.Parallel()
	ftt.Run("With CFT Test Request", t, func(t *ftt.Test) {
		request := &skylab_test_runner.CFTTestRequest{
			PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
				ProvisionState: &api.ProvisionState{
					SystemImage: &api.ProvisionState_SystemImage{
						SystemImagePath: &_go.StoragePath{
							HostType: _go.StoragePath_GS,
							Path:     "gs://some-bucket/builder/build-12345",
						},
					},
				},
			},
		}
		expectedSources := &metadata.PublishRdbMetadata_Sources{
			GsPath:            "gs://some-bucket/builder/build-12345/metadata/sources.jsonpb",
			IsDeploymentDirty: false,
		}
		t.Run("Base case", func(t *ftt.Test) {
			sources, err := commands.SourcesFromCFTTestRequest(request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, sources, should.Resemble(expectedSources))
		})
		t.Run("Invalid input", func(t *ftt.Test) {
			t.Run("No gs:// prefix", func(t *ftt.Test) {
				request.PrimaryDut.ProvisionState.SystemImage.SystemImagePath.Path = "/a/b/c"
				_, err := commands.SourcesFromCFTTestRequest(request)
				assert.Loosely(t, err, should.ErrLike("system_image_path.path: must start with gs://"))
			})
			t.Run("Trailing slash", func(t *ftt.Test) {
				request.PrimaryDut.ProvisionState.SystemImage.SystemImagePath.Path = "gs://some-bucket/builder/build-12345/"
				_, err := commands.SourcesFromCFTTestRequest(request)
				assert.Loosely(t, err, should.ErrLike("system_image_path.path: must not have trailing '/'"))
			})
		})
		t.Run("Local testing", func(t *ftt.Test) {
			request.PrimaryDut.ProvisionState.SystemImage.SystemImagePath = &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     "/builds/build-12345",
			}
			sources, err := commands.SourcesFromCFTTestRequest(request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, sources, should.BeNil)
		})
		t.Run("Lacros testing", func(t *ftt.Test) {
			request.PrimaryDut.ProvisionState.Packages = []*api.ProvisionState_Package{
				{
					PortagePackage: &buildapi.Portage_Package{},
				},
			}
			expectedSources.IsDeploymentDirty = true

			sources, err := commands.SourcesFromCFTTestRequest(request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, sources, should.Resemble(expectedSources))
		})
		t.Run("Firmware testing", func(t *ftt.Test) {
			request.PrimaryDut.ProvisionState.Firmware = &buildapi.FirmwareConfig{
				MainRoPayload: &buildapi.FirmwarePayload{},
			}
			expectedSources.IsDeploymentDirty = true

			sources, err := commands.SourcesFromCFTTestRequest(request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, sources, should.Resemble(expectedSources))
		})
	})
}

func TestRdbPublishPublishCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	// Common setup
	ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
	ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
	cont := containers.NewCrosPublishTemplatedContainer(
		containers.CrosRdbPublishTemplatedContainerType,
		"container/image/path",
		ctr)
	exec := executors.NewCrosPublishExecutor(
		cont,
		executors.CrosRdbPublishExecutorType)
	cmd := commands.NewRdbPublishUploadCmd(exec)

	ftt.Run("Populate TestResultForRdb with full info", t, func(t *ftt.Test) {
		ctx := context.Background()
		createTime := timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z"))
		startedTime := timestamppb.New(parseTime("2022-09-07T20:53:33.983328614Z"))
		duration := &duration.Duration{Seconds: 60}
		requester := "chrome-ci-builder@chops-service-accounts.iam.gserviceaccount.com"
		parentCreatedBy := "user:" + requester
		primaryDUT := &labapi.Dut{
			Id: &labapi.Dut_Id{Value: "0wgtfqin2033834d-ecghcra"},
			DutType: &labapi.Dut_Chromeos{
				Chromeos: &labapi.Dut_ChromeOS{
					Name: "0wgtfqin2033834d-ecghcra",
					DutModel: &labapi.DutModel{
						BuildTarget: "hatch",
						ModelName:   "nipperkin",
					},
					ModemInfo: &labapi.ModemInfo{
						Type: labapi.ModemType_MODEM_TYPE_FIBOCOMM_L850GL,
					},
					Sku: "CRAASK-HULX D4B-F4E-F3F-B2K-L3I-Q6I",
				},
			},
		}
		wantTestResult := &artifactpb.TestResult{
			TestInvocation: &artifactpb.TestInvocation{
				PrimaryExecutionInfo: &artifactpb.ExecutionInfo{
					BuildInfo: &artifactpb.BuildInfo{
						Name:        "hatch-cq/R106-15048.0.0",
						Board:       "hatch",
						BoardType:   "HW",
						BuildTarget: "hatch",
						BuildMetadata: &artifactpb.BuildMetadata{
							Sku: &artifactpb.BuildMetadata_Sku{
								HwidSku:  "CRAASK-HULX D4B-F4E-F3F-B2K-L3I-Q6I",
								DlmSkuId: "16968",
							},
							Chipset: &artifactpb.BuildMetadata_Chipset{
								WifiChip:         "INTEL_GFP2_AX211",
								WifiRouterModels: "gale",
							},
							Cellular: &artifactpb.BuildMetadata_Cellular{
								Carrier: "CARRIER_ESIM",
							},
							Firmware: &artifactpb.BuildMetadata_Firmware{},
							Kernel:   &artifactpb.BuildMetadata_Kernel{},
							Lacros:   &artifactpb.BuildMetadata_Lacros{},
							ModemInfo: &labapi.ModemInfo{
								Type: labapi.ModemType_MODEM_TYPE_FIBOCOMM_L850GL,
							},
							ChameleonInfo: &artifactpb.BuildMetadata_ChameleonInfo{
								ChameleonType: []artifactpb.BuildMetadata_ChameleonType{
									artifactpb.BuildMetadata_CHAMELEON_TYPE_V2,
									artifactpb.BuildMetadata_CHAMELEON_TYPE_V3,
								},
								ChameleonConnectionTypes: []artifactpb.BuildMetadata_ChameleonConnectionType{
									artifactpb.BuildMetadata_CHAMELEON_CONNECTION_TYPE_HDMI,
								},
							},
						},
					},
					DutInfo: &artifactpb.DutInfo{
						Dut: primaryDUT,
						ProvisionState: &testapi.ProvisionState{
							SystemImage: &testapi.ProvisionState_SystemImage{
								SystemImagePath: &_go.StoragePath{
									HostType: _go.StoragePath_GS,
									Path:     "gs://some-bucket/builder/build-12345",
								},
							},
						},
					},
					EnvInfo: &artifactpb.ExecutionInfo_SkylabInfo{
						SkylabInfo: &artifactpb.SkylabInfo{
							DroneInfo: &artifactpb.DroneInfo{
								Drone:       "skylab-drone-deployment-prod-6dc79d4f9-czjlj",
								DroneServer: "chromeos4-row4-rack1-drone8",
							},
							BuildbucketInfo: &artifactpb.BuildbucketInfo{
								Id:          100,
								AncestorIds: []int64{98, 99},
								Builder: &artifactpb.BuilderID{
									Project: "chromeos",
									Bucket:  "test_runner",
									Builder: "test_runner-dev",
								},
							},
							SwarmingInfo: &artifactpb.SwarmingInfo{
								TaskId:      "taskId0",
								SuiteTaskId: "parentId0",
								TaskName:    "bb-100-chromeos/test_runner/test_runner-dev",
								Pool:        "ChromeOSSkylab",
								LabelPool:   "DUT_POOL_QUOTA",
								BotId:       "cloudbots-prod-1715342009263-7kz6",
							},
						},
					},
					InventoryInfo: &artifactpb.InventoryInfo{
						UfsZone: "ZONE_SFO36_OS",
					},
				},
				DutTopology: &labapi.DutTopology{
					Id:   &labapi.DutTopology_Id{Value: "0wgtfqin2033834d-ecghcra"},
					Duts: []*labapi.Dut{primaryDUT},
				},
				SchedulingMetadata: &artifactpb.SchedulingMetadata{
					SchedulingArgs: map[string]string{
						"display_name":      "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq",
						"analytics_name":    "Bluetooth_Sa_Perbuild",
						"ctp-fwd-task-name": "Bluetooth_Sa_Perbuild",
						"qs_account":        "unmanaged_p2",
						"parent_task_id":    "parentId1",
						"branch-trigger":    "DEV",
						"bug_id":            "1234",
						"parent_created_by": parentCreatedBy,
					},
				},
				ProjectTrackerMetadata: &artifactpb.ProjectTrackerMetadata{
					BugId: "1234",
				},
				PartnerInfo: &artifactpb.PartnerInfo{
					AccountId: 4,
				},
				IsCftRun:  true,
				IsTrv2Run: true,
			},
			TestRuns: []*artifactpb.TestRun{
				{
					TestCaseInfo: &artifactpb.TestCaseInfo{
						TestCaseResult: &testapi.TestCaseResult{
							TestHarness: &testapi.TestHarness{
								TestHarnessType: &testapi.TestHarness_Tast_{
									Tast: &testapi.TestHarness_Tast{},
								},
							},
							TestCaseId: &testapi.TestCase_Id{
								Value: "tast.rlz_CheckPing",
							},
							Verdict:   &testapi.TestCaseResult_Pass_{},
							StartTime: startedTime,
							Duration:  duration,
						},
						DisplayName:     "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq",
						Suite:           "arc-cts-vm",
						Branch:          "main",
						MainBuilderName: "main-release",
						Channel:         "DEV",
						Requester:       requester,
					},
					LogsInfo: []*configpb.StoragePath{
						{
							HostType: configpb.StoragePath_GS,
							Path:     "gs://some-bucket/builder/build-12345",
						},
					},
					TimeInfo: &artifactpb.TimingInfo{
						QueuedTime:  createTime,
						StartedTime: startedTime,
						Duration:    duration,
					},
					ExecutionMetadata: &artifactpb.ExecutionMetadata{
						TestArgs: map[string]string{
							"bug_id":      "12345",
							"qual_run_id": "1712172839652",
						},
					},
				},
			},
		}

		// Sets up the build info.
		buildPb := &bbpb.Build{
			Id:     100,
			Status: bbpb.Status_SUCCESS,
			Builder: &bbpb.BuilderID{
				Project: "chromeos",
				Bucket:  "test_runner",
				Builder: "test_runner-dev",
			},
			AncestorIds: []int64{98, 99},
			Tags: []*buildbucketpb.StringPair{
				{Key: "display_name", Value: "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq"},
				{Key: "analytics_name", Value: "Bluetooth_Sa_Perbuild"},
				{Key: "ctp-fwd-task-name", Value: "Bluetooth_Sa_Perbuild"},
				{Key: "qs_account", Value: "unmanaged_p2"},
				{Key: "parent_task_id", Value: "parentId1"},
				{Key: "branch-trigger", Value: "DEV"},
				{Key: "bug_id", Value: "1234"},
				{Key: "parent_created_by", Value: parentCreatedBy},
			},
			CreateTime: createTime,
			Infra: &bbpb.BuildInfra{Swarming: &bbpb.BuildInfra_Swarming{
				TaskId:      "taskId1",
				ParentRunId: "parentId1",
				BotDimensions: []*buildbucketpb.StringPair{
					{Key: "label-wifi_chip", Value: "INTEL_GFP2_AX211"},
					{Key: "label-wifi_router_models", Value: "gale"},
					{Key: "label-dlm_sku_id", Value: "16968"},
					{Key: "label-carrier", Value: "CARRIER_ESIM"},
					{Key: "drone", Value: "skylab-drone-deployment-prod-6dc79d4f9-czjlj"},
					{Key: "drone_server", Value: "chromeos4-row4-rack1-drone8"},
					{Key: "pool", Value: "ChromeOSSkylab"},
					{Key: "label-pool", Value: "DUT_POOL_QUOTA"},
					{Key: "ufs_zone", Value: "ZONE_SFO36_OS"},
					{Key: "label-chameleon_type", Value: "CHAMELEON_TYPE_V2"},
					{Key: "label-chameleon_connection_types", Value: "CHAMELEON_CONNECTION_TYPE_HDMI"},
					{Key: "label-chameleon_type", Value: "CHAMELEON_TYPE_V3"},
					{Key: "id", Value: "cloudbots-prod-1715342009263-7kz6"},
				},
			}},
		}
		buildState, ctx, err := build.Start(ctx, buildPb)
		defer func() { buildState.End(err) }()

		sk := &data.HwTestStateKeeper{
			Injectables:         common.NewInjectableStorage(),
			CurrentInvocationId: "Inv-1234",
			TesthausURL:         "www.testhaus.com",
			BaseVariant: map[string]string{
				"board":        "asurada",
				"model":        "hayato",
				"build_target": "asurada",
			},
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				RunViaTrv2: true,
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					DutModel: &labapi.DutModel{
						BuildTarget: primaryDUT.GetChromeos().GetDutModel().GetBuildTarget(),
					},
					ProvisionState: &api.ProvisionState{
						SystemImage: &api.ProvisionState_SystemImage{
							SystemImagePath: &_go.StoragePath{
								HostType: _go.StoragePath_GS,
								Path:     "gs://some-bucket/builder/build-12345",
							},
						},
					},
				},
				AutotestKeyvals: map[string]string{
					"build_target":        "hatch",
					"build":               "hatch-cq/R106-15048.0.0",
					"suite":               "arc-cts-vm",
					"branch":              "main",
					"master_build_config": "main-release",
				},
				TestSuites: []*testapi.TestSuite{
					{
						Spec: &testapi.TestSuite_TestCaseIds{
							TestCaseIds: &testapi.TestCaseIdList{

								TestCaseIds: []*testapi.TestCase_Id{
									{
										Value: "tast.rlz_CheckPing",
									},
								},
							},
						},
						ExecutionMetadata: &testapi.ExecutionMetadata{
							Args: []*testapi.Arg{
								{
									Flag:  "bug_id",
									Value: "12345",
								},
								{
									Flag:  "qual_run_id",
									Value: "1712172839652",
								},
							},
						},
					},
				},
			},
			CommonConfig: &skylab_test_runner.CommonConfig{
				PartnerConfig: &skylab_test_runner.CommonConfig_PartnerConfig{
					AccountId: 4,
				},
			},
			Devices: map[string]*testapi.CrosTestRequest_Device{
				common.Primary: {
					Dut: primaryDUT,
				},
			},
			PrimaryDevice: &testapi.CrosTestRequest_Device{
				Dut: primaryDUT,
			},
			GcsURL:     "gs://some-bucket/builder/build-12345",
			BuildState: buildState,
			DutTopology: &labapi.DutTopology{
				Id:   &labapi.DutTopology_Id{Value: "0wgtfqin2033834d-ecghcra"},
				Duts: []*labapi.Dut{primaryDUT},
			},
			TestResponses: &testapi.CrosTestResponse{
				TestCaseResults: []*testapi.TestCaseResult{
					{
						TestHarness: &testapi.TestHarness{
							TestHarnessType: &testapi.TestHarness_Tast_{
								Tast: &testapi.TestHarness_Tast{},
							},
						},
						TestCaseId: &testapi.TestCase_Id{
							Value: "tast.rlz_CheckPing",
						},
						Verdict:   &testapi.TestCaseResult_Pass_{},
						StartTime: startedTime,
						Duration:  duration,
					},
				},
			},
		}

		// Extract deps first
		err = cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.TestResultForRdb, should.Resemble(wantTestResult))
	})

	ftt.Run("Populate TestResultForRdb with board type based on builder name", t, func(t *ftt.Test) {
		ctx := context.Background()
		createTime := timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z"))
		startedTime := timestamppb.New(parseTime("2022-09-07T20:53:33.983328614Z"))
		primaryDUT := &labapi.Dut{
			Id: &labapi.Dut_Id{Value: "0wgtfqin2033834d-ecghcra"},
			DutType: &labapi.Dut_Chromeos{
				Chromeos: &labapi.Dut_ChromeOS{
					Name:      "0wgtfqin2033834d-ecghcra",
					DutModel:  &labapi.DutModel{},
					ModemInfo: &labapi.ModemInfo{},
				},
			},
		}
		wantTestResult := &artifactpb.TestResult{
			TestInvocation: &artifactpb.TestInvocation{
				PrimaryExecutionInfo: &artifactpb.ExecutionInfo{
					BuildInfo: &artifactpb.BuildInfo{
						Name:        "hatch-cq/R106-15048.0.0",
						Board:       "hatch",
						BoardType:   "VM",
						BuildTarget: "hatch",
						BuildMetadata: &artifactpb.BuildMetadata{
							Sku:           &artifactpb.BuildMetadata_Sku{},
							Chipset:       &artifactpb.BuildMetadata_Chipset{},
							Cellular:      &artifactpb.BuildMetadata_Cellular{},
							Firmware:      &artifactpb.BuildMetadata_Firmware{},
							Kernel:        &artifactpb.BuildMetadata_Kernel{},
							Lacros:        &artifactpb.BuildMetadata_Lacros{},
							ModemInfo:     &labapi.ModemInfo{},
							ChameleonInfo: &artifactpb.BuildMetadata_ChameleonInfo{},
						},
					},
					DutInfo: &artifactpb.DutInfo{
						Dut: primaryDUT,
						ProvisionState: &testapi.ProvisionState{
							SystemImage: &testapi.ProvisionState_SystemImage{
								SystemImagePath: &_go.StoragePath{
									HostType: _go.StoragePath_GS,
									Path:     "gs://some-bucket/builder/build-12345",
								},
							},
						},
					},
					EnvInfo: &artifactpb.ExecutionInfo_SkylabInfo{
						SkylabInfo: &artifactpb.SkylabInfo{
							DroneInfo: &artifactpb.DroneInfo{},
							BuildbucketInfo: &artifactpb.BuildbucketInfo{
								Id: 100,
								Builder: &artifactpb.BuilderID{
									Project: "chromeos",
									Bucket:  "test_runner",
									Builder: "test_runner_gce",
								},
							},
							SwarmingInfo: &artifactpb.SwarmingInfo{
								TaskName: "bb-100-chromeos/test_runner/test_runner_gce",
							},
						},
					},
					InventoryInfo: &artifactpb.InventoryInfo{},
				},
				DutTopology: &labapi.DutTopology{
					Id:   &labapi.DutTopology_Id{Value: "0wgtfqin2033834d-ecghcra"},
					Duts: []*labapi.Dut{primaryDUT},
				},
				SchedulingMetadata: &artifactpb.SchedulingMetadata{
					SchedulingArgs: map[string]string{
						"display_name": "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq",
					},
				},
				ProjectTrackerMetadata: &artifactpb.ProjectTrackerMetadata{},
				PartnerInfo:            &artifactpb.PartnerInfo{},
				IsCftRun:               true,
				IsTrv2Run:              true,
			},
			TestRuns: []*artifactpb.TestRun{
				{
					TestCaseInfo: &artifactpb.TestCaseInfo{
						TestCaseResult: &testapi.TestCaseResult{
							TestHarness: &testapi.TestHarness{
								TestHarnessType: &testapi.TestHarness_Tast_{
									Tast: &testapi.TestHarness_Tast{},
								},
							},
							TestCaseId: &testapi.TestCase_Id{
								Value: "tast.rlz_CheckPing",
							},
							Verdict:   &testapi.TestCaseResult_Pass_{},
							StartTime: startedTime,
						},
						DisplayName:     "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq",
						Suite:           "arc-cts-vm",
						Branch:          "main",
						MainBuilderName: "main-release",
					},
					LogsInfo: []*configpb.StoragePath{
						{
							HostType: configpb.StoragePath_GS,
							Path:     "gs://some-bucket/builder/build-12345",
						},
					},
					TimeInfo: &artifactpb.TimingInfo{
						QueuedTime:  createTime,
						StartedTime: startedTime,
					},
					ExecutionMetadata: &artifactpb.ExecutionMetadata{},
				},
			},
		}

		// Sets up the build info.
		buildPb := &bbpb.Build{
			Id:     100,
			Status: bbpb.Status_SUCCESS,
			Builder: &bbpb.BuilderID{
				Project: "chromeos",
				Bucket:  "test_runner",
				Builder: "test_runner_gce",
			},
			Tags: []*buildbucketpb.StringPair{
				{Key: "display_name", Value: "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq"},
			},
			CreateTime: createTime,
		}
		buildState, ctx, err := build.Start(ctx, buildPb)
		defer func() { buildState.End(err) }()

		sk := &data.HwTestStateKeeper{
			Injectables:         common.NewInjectableStorage(),
			CurrentInvocationId: "Inv-1234",
			TesthausURL:         "www.testhaus.com",
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				RunViaTrv2: true,
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					DutModel: &labapi.DutModel{
						BuildTarget: primaryDUT.GetChromeos().GetDutModel().GetBuildTarget(),
					},
					ProvisionState: &api.ProvisionState{
						SystemImage: &api.ProvisionState_SystemImage{
							SystemImagePath: &_go.StoragePath{
								HostType: _go.StoragePath_GS,
								Path:     "gs://some-bucket/builder/build-12345",
							},
						},
					},
				},
				AutotestKeyvals: map[string]string{
					"build_target":        "hatch",
					"build":               "hatch-cq/R106-15048.0.0",
					"suite":               "arc-cts-vm",
					"branch":              "main",
					"master_build_config": "main-release",
				},
				TestSuites: []*testapi.TestSuite{
					{
						Spec: &testapi.TestSuite_TestCaseIds{
							TestCaseIds: &testapi.TestCaseIdList{

								TestCaseIds: []*testapi.TestCase_Id{
									{
										Value: "tast.rlz_CheckPing",
									},
								},
							},
						},
						ExecutionMetadata: &testapi.ExecutionMetadata{
							Args: []*testapi.Arg{},
						},
					},
				},
			},
			Devices: map[string]*testapi.CrosTestRequest_Device{
				common.Primary: {
					Dut: primaryDUT,
				},
			},
			PrimaryDevice: &testapi.CrosTestRequest_Device{
				Dut: primaryDUT,
			},
			GcsURL:     "gs://some-bucket/builder/build-12345",
			BuildState: buildState,
			DutTopology: &labapi.DutTopology{
				Id:   &labapi.DutTopology_Id{Value: "0wgtfqin2033834d-ecghcra"},
				Duts: []*labapi.Dut{primaryDUT},
			},
			TestResponses: &testapi.CrosTestResponse{
				TestCaseResults: []*testapi.TestCaseResult{
					{
						TestHarness: &testapi.TestHarness{
							TestHarnessType: &testapi.TestHarness_Tast_{
								Tast: &testapi.TestHarness_Tast{},
							},
						},
						TestCaseId: &testapi.TestCase_Id{
							Value: "tast.rlz_CheckPing",
						},
						Verdict:   &testapi.TestCaseResult_Pass_{},
						StartTime: startedTime,
					},
				},
			},
		}

		// Extract deps first
		err = cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.TestResultForRdb, should.Resemble(wantTestResult))
	})

	ftt.Run("Populate TestResultForRdb for multi-dut testing", t, func(t *ftt.Test) {
		ctx := context.Background()
		createTime := timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z"))
		startedTime := timestamppb.New(parseTime("2022-09-07T20:53:33.983328614Z"))
		duration := &duration.Duration{Seconds: 60}
		primaryDUT := &labapi.Dut{
			Id: &labapi.Dut_Id{Value: "0wgtfqin2033834d-ecghcra"},
			DutType: &labapi.Dut_Chromeos{
				Chromeos: &labapi.Dut_ChromeOS{
					Name: "0wgtfqin2033834d-ecghcra",
					DutModel: &labapi.DutModel{
						BuildTarget: "hatch",
						ModelName:   "nipperkin",
					},
					ModemInfo: &labapi.ModemInfo{
						Type: labapi.ModemType_MODEM_TYPE_FIBOCOMM_L850GL,
					},
					Sku: "CRAASK-HULX D4B-F4E-F3F-B2K-L3I-Q6I",
				},
			},
		}
		secondaryDUT := &labapi.Dut{
			Id: &labapi.Dut_Id{Value: "chromeos8-row1-rack10-unit1"},
			DutType: &labapi.Dut_Chromeos{
				Chromeos: &labapi.Dut_ChromeOS{
					Name: "chromeos8-row1-rack10-unit1",
					DutModel: &labapi.DutModel{
						BuildTarget: "dedede",
						ModelName:   "drawman",
					},
					ModemInfo: &labapi.ModemInfo{
						Type: labapi.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
					},
					Sku: "CRAASK-HULX D4B-F4E-F3F-B2K-L3I-ABC",
				},
			},
		}
		wantTestResult := &artifactpb.TestResult{
			TestInvocation: &artifactpb.TestInvocation{
				PrimaryExecutionInfo: &artifactpb.ExecutionInfo{
					BuildInfo: &artifactpb.BuildInfo{
						Name:        "hatch-cq/R106-15048.0.0",
						Board:       "hatch",
						BoardType:   "HW",
						BuildTarget: "hatch",
						BuildMetadata: &artifactpb.BuildMetadata{
							Sku: &artifactpb.BuildMetadata_Sku{
								HwidSku:  "CRAASK-HULX D4B-F4E-F3F-B2K-L3I-Q6I",
								DlmSkuId: "16968",
							},
							Chipset: &artifactpb.BuildMetadata_Chipset{
								WifiChip:         "INTEL_GFP2_AX211",
								WifiRouterModels: "gale",
							},
							Cellular: &artifactpb.BuildMetadata_Cellular{
								Carrier: "CARRIER_ESIM",
							},
							Firmware:      &artifactpb.BuildMetadata_Firmware{},
							Kernel:        &artifactpb.BuildMetadata_Kernel{},
							Lacros:        &artifactpb.BuildMetadata_Lacros{},
							ChameleonInfo: &artifactpb.BuildMetadata_ChameleonInfo{},
							ModemInfo: &labapi.ModemInfo{
								Type: labapi.ModemType_MODEM_TYPE_FIBOCOMM_L850GL,
							},
						},
					},
					DutInfo: &artifactpb.DutInfo{
						Dut: primaryDUT,
						ProvisionState: &testapi.ProvisionState{
							SystemImage: &testapi.ProvisionState_SystemImage{
								SystemImagePath: &_go.StoragePath{
									HostType: _go.StoragePath_GS,
									Path:     "gs://some-bucket/builder/build-12345",
								},
							},
						},
					},
					EnvInfo: &artifactpb.ExecutionInfo_SkylabInfo{
						SkylabInfo: &artifactpb.SkylabInfo{
							DroneInfo: &artifactpb.DroneInfo{
								Drone:       "skylab-drone-deployment-prod-6dc79d4f9-czjlj",
								DroneServer: "chromeos4-row4-rack1-drone8",
							},
							BuildbucketInfo: &artifactpb.BuildbucketInfo{
								Id:          100,
								AncestorIds: []int64{98, 99},
								Builder: &artifactpb.BuilderID{
									Project: "chromeos",
									Bucket:  "test_runner",
									Builder: "test_runner-dev",
								},
							},
							SwarmingInfo: &artifactpb.SwarmingInfo{
								TaskId:      "taskId0",
								SuiteTaskId: "parentId0",
								TaskName:    "bb-100-chromeos/test_runner/test_runner-dev",
								Pool:        "ChromeOSSkylab",
								LabelPool:   "DUT_POOL_QUOTA",
								BotId:       "cloudbots-prod-1711483217016-h2cz",
							},
						},
					},
					InventoryInfo: &artifactpb.InventoryInfo{
						UfsZone: "ZONE_SFO36_OS",
					},
				},
				SecondaryExecutionsInfo: []*artifactpb.ExecutionInfo{
					{
						BuildInfo: &artifactpb.BuildInfo{
							Name:        "hatch-cq/R106-15048.0.0",
							Board:       "dedede",
							BoardType:   "HW",
							BuildTarget: "dedede",
							BuildMetadata: &artifactpb.BuildMetadata{
								Sku: &artifactpb.BuildMetadata_Sku{
									HwidSku:  "CRAASK-HULX D4B-F4E-F3F-B2K-L3I-ABC",
									DlmSkuId: "16968",
								},
								Chipset: &artifactpb.BuildMetadata_Chipset{
									WifiChip:         "INTEL_GFP2_AX211",
									WifiRouterModels: "gale",
								},
								Cellular: &artifactpb.BuildMetadata_Cellular{
									Carrier: "CARRIER_ESIM",
								},
								Firmware:      &artifactpb.BuildMetadata_Firmware{},
								Kernel:        &artifactpb.BuildMetadata_Kernel{},
								Lacros:        &artifactpb.BuildMetadata_Lacros{},
								ChameleonInfo: &artifactpb.BuildMetadata_ChameleonInfo{},
								ModemInfo: &labapi.ModemInfo{
									Type: labapi.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
								},
							},
						},
						DutInfo: &artifactpb.DutInfo{
							Dut: secondaryDUT,
							ProvisionState: &testapi.ProvisionState{
								SystemImage: &testapi.ProvisionState_SystemImage{
									SystemImagePath: &_go.StoragePath{
										HostType: _go.StoragePath_GS,
										Path:     "gs://some-bucket/builder/build-6789",
									},
								},
							},
						},
						EnvInfo: &artifactpb.ExecutionInfo_SkylabInfo{
							SkylabInfo: &artifactpb.SkylabInfo{
								DroneInfo: &artifactpb.DroneInfo{
									Drone:       "skylab-drone-deployment-prod-6dc79d4f9-czjlj",
									DroneServer: "chromeos4-row4-rack1-drone8",
								},
								BuildbucketInfo: &artifactpb.BuildbucketInfo{
									Id:          100,
									AncestorIds: []int64{98, 99},
									Builder: &artifactpb.BuilderID{
										Project: "chromeos",
										Bucket:  "test_runner",
										Builder: "test_runner-dev",
									},
								},
								SwarmingInfo: &artifactpb.SwarmingInfo{
									TaskId:      "taskId0",
									SuiteTaskId: "parentId0",
									TaskName:    "bb-100-chromeos/test_runner/test_runner-dev",
									Pool:        "ChromeOSSkylab",
									LabelPool:   "DUT_POOL_QUOTA",
									BotId:       "cloudbots-prod-1711483217016-h2cz",
								},
							},
						},
						InventoryInfo: &artifactpb.InventoryInfo{
							UfsZone: "ZONE_SFO36_OS",
						},
					},
				},
				DutTopology: &labapi.DutTopology{
					Id:   &labapi.DutTopology_Id{Value: "0wgtfqin2033834d-ecghcra"},
					Duts: []*labapi.Dut{primaryDUT, secondaryDUT},
				},
				SchedulingMetadata: &artifactpb.SchedulingMetadata{
					SchedulingArgs: map[string]string{
						"display_name":      "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq",
						"analytics_name":    "Bluetooth_Sa_Perbuild",
						"ctp-fwd-task-name": "Bluetooth_Sa_Perbuild",
						"qs_account":        "unmanaged_p2",
						"parent_task_id":    "parentId1",
						"branch-trigger":    "DEV",
					},
				},
				ProjectTrackerMetadata: &artifactpb.ProjectTrackerMetadata{},
				PartnerInfo:            &artifactpb.PartnerInfo{},
				IsCftRun:               true,
				IsTrv2Run:              true,
			},
			TestRuns: []*artifactpb.TestRun{
				{
					TestCaseInfo: &artifactpb.TestCaseInfo{
						TestCaseResult: &testapi.TestCaseResult{
							TestHarness: &testapi.TestHarness{
								TestHarnessType: &testapi.TestHarness_Tast_{
									Tast: &testapi.TestHarness_Tast{},
								},
							},
							TestCaseId: &testapi.TestCase_Id{
								Value: "tast.rlz_CheckPing",
							},
							Verdict:   &testapi.TestCaseResult_Pass_{},
							StartTime: startedTime,
							Duration:  duration,
						},
						DisplayName:     "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq",
						Suite:           "arc-cts-vm",
						Branch:          "main",
						MainBuilderName: "main-release",
						Channel:         "DEV",
					},
					LogsInfo: []*configpb.StoragePath{
						{
							HostType: configpb.StoragePath_GS,
							Path:     "gs://some-bucket/builder/build-12345",
						},
					},
					TimeInfo: &artifactpb.TimingInfo{
						QueuedTime:  createTime,
						StartedTime: startedTime,
						Duration:    duration,
					},
				},
			},
		}

		// Sets up the build info.
		buildPb := &bbpb.Build{
			Id:     100,
			Status: bbpb.Status_SUCCESS,
			Builder: &bbpb.BuilderID{
				Project: "chromeos",
				Bucket:  "test_runner",
				Builder: "test_runner-dev",
			},
			AncestorIds: []int64{98, 99},
			Tags: []*buildbucketpb.StringPair{
				{Key: "display_name", Value: "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq"},
				{Key: "analytics_name", Value: "Bluetooth_Sa_Perbuild"},
				{Key: "ctp-fwd-task-name", Value: "Bluetooth_Sa_Perbuild"},
				{Key: "qs_account", Value: "unmanaged_p2"},
				{Key: "parent_task_id", Value: "parentId1"},
				{Key: "branch-trigger", Value: "DEV"},
			},
			CreateTime: createTime,
			Infra: &bbpb.BuildInfra{Swarming: &bbpb.BuildInfra_Swarming{
				TaskId:      "taskId1",
				ParentRunId: "parentId1",
				BotDimensions: []*buildbucketpb.StringPair{
					{Key: "label-wifi_chip", Value: "INTEL_GFP2_AX211"},
					{Key: "label-wifi_router_models", Value: "gale"},
					{Key: "label-dlm_sku_id", Value: "16968"},
					{Key: "label-carrier", Value: "CARRIER_ESIM"},
					{Key: "drone", Value: "skylab-drone-deployment-prod-6dc79d4f9-czjlj"},
					{Key: "drone_server", Value: "chromeos4-row4-rack1-drone8"},
					{Key: "pool", Value: "ChromeOSSkylab"},
					{Key: "label-pool", Value: "DUT_POOL_QUOTA"},
					{Key: "ufs_zone", Value: "ZONE_SFO36_OS"},
					{Key: "id", Value: "cloudbots-prod-1711483217016-h2cz"},
				},
			}},
		}
		buildState, ctx, err := build.Start(ctx, buildPb)
		defer func() { buildState.End(err) }()

		secondaryDutPrivisionState := &api.ProvisionState{
			SystemImage: &api.ProvisionState_SystemImage{
				SystemImagePath: &_go.StoragePath{
					HostType: _go.StoragePath_GS,
					Path:     "gs://some-bucket/builder/build-6789",
				},
			},
		}
		sk := &data.HwTestStateKeeper{
			Injectables:         common.NewInjectableStorage(),
			CurrentInvocationId: "Inv-1234",
			TesthausURL:         "www.testhaus.com",
			BaseVariant: map[string]string{
				"board":        "asurada",
				"model":        "hayato",
				"build_target": "asurada",
			},
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					DutModel: &labapi.DutModel{
						BuildTarget: primaryDUT.GetChromeos().GetDutModel().GetBuildTarget(),
					},
					ProvisionState: &api.ProvisionState{
						SystemImage: &api.ProvisionState_SystemImage{
							SystemImagePath: &_go.StoragePath{
								HostType: _go.StoragePath_GS,
								Path:     "gs://some-bucket/builder/build-12345",
							},
						},
					},
				},
				CompanionDuts: []*skylab_test_runner.CFTTestRequest_Device{
					{
						DutModel: &labapi.DutModel{
							BuildTarget: secondaryDUT.GetChromeos().GetDutModel().GetBuildTarget(),
						},
						ProvisionState: secondaryDutPrivisionState,
					},
				},
				AutotestKeyvals: map[string]string{
					"build_target":        "hatch",
					"build":               "hatch-cq/R106-15048.0.0",
					"suite":               "arc-cts-vm",
					"branch":              "main",
					"master_build_config": "main-release",
				},
			},
			Devices: map[string]*testapi.CrosTestRequest_Device{
				common.Primary: {
					Dut: primaryDUT,
				},
				common.Companion: {
					Dut: secondaryDUT,
				},
			},
			PrimaryDevice: &testapi.CrosTestRequest_Device{
				Dut: primaryDUT,
			},
			CompanionDevices: []*testapi.CrosTestRequest_Device{
				{
					Dut: secondaryDUT,
				},
			},
			CompanionDevicesMetadata: []*skylab_test_runner.CFTTestRequest_Device{
				{
					ProvisionState: secondaryDutPrivisionState,
				},
			},
			GcsURL:     "gs://some-bucket/builder/build-12345",
			BuildState: buildState,
			DutTopology: &labapi.DutTopology{
				Id:   &labapi.DutTopology_Id{Value: "0wgtfqin2033834d-ecghcra"},
				Duts: []*labapi.Dut{primaryDUT, secondaryDUT},
			},
			TestResponses: &testapi.CrosTestResponse{
				TestCaseResults: []*testapi.TestCaseResult{
					{
						TestHarness: &testapi.TestHarness{
							TestHarnessType: &testapi.TestHarness_Tast_{
								Tast: &testapi.TestHarness_Tast{},
							},
						},
						TestCaseId: &testapi.TestCase_Id{
							Value: "tast.rlz_CheckPing",
						},
						Verdict:   &testapi.TestCaseResult_Pass_{},
						StartTime: startedTime,
						Duration:  duration,
					},
				},
			},
		}

		// Extract deps first
		err = cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.TestResultForRdb, should.Resemble(wantTestResult))
	})

	ftt.Run("Populate additional info from buildbucket tags", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantTestResult := &artifactpb.TestResult{
			TestInvocation: &artifactpb.TestInvocation{
				PrimaryExecutionInfo: &artifactpb.ExecutionInfo{
					BuildInfo: &artifactpb.BuildInfo{
						BoardType: "HW",
						BuildMetadata: &artifactpb.BuildMetadata{
							Sku:      &artifactpb.BuildMetadata_Sku{},
							Chipset:  &artifactpb.BuildMetadata_Chipset{},
							Cellular: &artifactpb.BuildMetadata_Cellular{},
							Firmware: &artifactpb.BuildMetadata_Firmware{},
							Kernel:   &artifactpb.BuildMetadata_Kernel{},
							Lacros:   &artifactpb.BuildMetadata_Lacros{},
							ChameleonInfo: &artifactpb.BuildMetadata_ChameleonInfo{
								ChameleonType:            []artifactpb.BuildMetadata_ChameleonType{},
								ChameleonConnectionTypes: []artifactpb.BuildMetadata_ChameleonConnectionType{},
							},
						},
					},
					DutInfo: &artifactpb.DutInfo{
						ProvisionState: &testapi.ProvisionState{
							SystemImage: &testapi.ProvisionState_SystemImage{
								SystemImagePath: &_go.StoragePath{
									HostType: _go.StoragePath_GS,
									Path:     "gs://some-bucket/builder/build-12345",
								},
							},
						},
					},
					EnvInfo: &artifactpb.ExecutionInfo_SkylabInfo{
						SkylabInfo: &artifactpb.SkylabInfo{
							DroneInfo: &artifactpb.DroneInfo{},
							BuildbucketInfo: &artifactpb.BuildbucketInfo{
								Id:          100,
								AncestorIds: []int64{99},
							},
							SwarmingInfo: &artifactpb.SwarmingInfo{
								LabelPool: "vmlab",
							},
						},
					},
					InventoryInfo: &artifactpb.InventoryInfo{},
				},
				DutTopology: &labapi.DutTopology{Duts: []*labapi.Dut{{}}},
				SchedulingMetadata: &artifactpb.SchedulingMetadata{
					SchedulingArgs: map[string]string{
						"label-pool":            "vmlab",
						"parent_buildbucket_id": "99",
						"suite":                 "bvt-tast-cq",
					},
				},
				ProjectTrackerMetadata: &artifactpb.ProjectTrackerMetadata{},
				PartnerInfo:            &artifactpb.PartnerInfo{},
				IsCftRun:               true,
				IsTrv2Run:              true,
			},
			TestRuns: []*artifactpb.TestRun{
				{
					TestCaseInfo: &artifactpb.TestCaseInfo{
						TestCaseResult: &testapi.TestCaseResult{
							TestHarness: &testapi.TestHarness{
								TestHarnessType: &testapi.TestHarness_Tast_{
									Tast: &testapi.TestHarness_Tast{},
								},
							},
							TestCaseId: &testapi.TestCase_Id{
								Value: "tast.rlz_CheckPing",
							},
							Verdict: &testapi.TestCaseResult_Pass_{},
						},
						Suite: "bvt-tast-cq",
					},
					LogsInfo: []*configpb.StoragePath{
						{
							HostType: configpb.StoragePath_GS,
							Path:     "gs://some-bucket/builder/build-12345",
						},
					},
					TimeInfo: &artifactpb.TimingInfo{},
				},
			},
		}

		// Sets up the build info.
		buildPb := &bbpb.Build{
			Id:     100,
			Status: bbpb.Status_SUCCESS,
			Tags: []*buildbucketpb.StringPair{
				{Key: "label-pool", Value: "vmlab"},
				{Key: "parent_buildbucket_id", Value: "99"},
				{Key: "suite", Value: "bvt-tast-cq"},
			},
		}
		buildState, ctx, err := build.Start(ctx, buildPb)
		defer func() { buildState.End(err) }()

		sk := &data.HwTestStateKeeper{
			Injectables:         common.NewInjectableStorage(),
			CurrentInvocationId: "Inv-1234",
			TesthausURL:         "www.testhaus.com",
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					DutModel: &labapi.DutModel{},
					ProvisionState: &api.ProvisionState{
						SystemImage: &api.ProvisionState_SystemImage{
							SystemImagePath: &_go.StoragePath{
								HostType: _go.StoragePath_GS,
								Path:     "gs://some-bucket/builder/build-12345",
							},
						},
					},
				},
			},
			GcsURL:     "gs://some-bucket/builder/build-12345",
			BuildState: buildState,
			TestResponses: &testapi.CrosTestResponse{
				TestCaseResults: []*testapi.TestCaseResult{
					{
						TestHarness: &testapi.TestHarness{
							TestHarnessType: &testapi.TestHarness_Tast_{
								Tast: &testapi.TestHarness_Tast{},
							},
						},
						TestCaseId: &testapi.TestCase_Id{
							Value: "tast.rlz_CheckPing",
						},
						Verdict: &testapi.TestCaseResult_Pass_{},
					},
				},
			},
		}

		// Extract deps first
		err = cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.TestResultForRdb, should.Resemble(wantTestResult))
	})

	ftt.Run("ProvisionStartCmd extract deps with TestResultForRdb", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantInvId := "Inv-1234"
		wantTesthausURL := "www.testhaus.com"
		wantBaseVariant := map[string]string{
			"board":        "asurada",
			"model":        "hayato",
			"build_target": "asurada",
		}
		sk := &data.HwTestStateKeeper{
			Injectables:         common.NewInjectableStorage(),
			CurrentInvocationId: wantInvId,
			TesthausURL:         wantTesthausURL,
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					ProvisionState: &api.ProvisionState{
						SystemImage: &api.ProvisionState_SystemImage{
							SystemImagePath: &_go.StoragePath{
								HostType: _go.StoragePath_GS,
								Path:     "gs://some-bucket/builder/build-12345",
							},
						},
					},
				},
			},
			BaseVariant:      wantBaseVariant,
			TestResultForRdb: &artifact.TestResult{Version: 1234},
		}

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.CurrentInvocationId, should.Equal(wantInvId))
		assert.Loosely(t, cmd.TesthausURL, should.Equal(wantTesthausURL))
		assert.Loosely(t, cmd.Sources, should.Resemble(&metadata.PublishRdbMetadata_Sources{
			GsPath:            "gs://some-bucket/builder/build-12345/metadata/sources.jsonpb",
			IsDeploymentDirty: false,
		}))
		assert.Loosely(t, cmd.BaseVariant, should.Match(wantBaseVariant))
	})

	ftt.Run("ProvisionStartCmd extract deps without TestResultForRdb", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantInvId := "Inv-1234"
		wantTesthausURL := "www.testhaus.com"
		wantBaseVariant := map[string]string{
			"board":        "asurada",
			"model":        "hayato",
			"build_target": "asurada",
		}
		sk := &data.HwTestStateKeeper{
			Injectables:         common.NewInjectableStorage(),
			CurrentInvocationId: wantInvId,
			TesthausURL:         wantTesthausURL,
			BaseVariant:         wantBaseVariant,
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					ProvisionState: &api.ProvisionState{},
				},
			},
		}

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, cmd.CurrentInvocationId, should.Equal(wantInvId))
		assert.Loosely(t, cmd.TesthausURL, should.Equal(wantTesthausURL))
		assert.Loosely(t, cmd.BaseVariant, should.Match(wantBaseVariant))
		assert.Loosely(t, err, should.ErrLike("missing dependency: BuildState"))
	})
}
