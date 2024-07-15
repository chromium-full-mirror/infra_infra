// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_builders

import (
	"regexp"
	"strconv"

	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi_metadata "go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

	"infra/cros/cmd/common_lib/common"
)

// buildDynamicRequest constructs the base DynamicTrv2Builder for DynamicTrv2FromCft.
func (builder *DynamicTrv2FromCft) buildDynamicRequest() *DynamicTrv2Builder {
	keyvals := builder.Cft.GetAutotestKeyvals()
	if keyvals == nil {
		keyvals = make(map[string]string)
	}

	testSuites := builder.Cft.GetTestSuites()
	if testSuites == nil {
		testSuites = []*api.TestSuite{}
	}

	return &DynamicTrv2Builder{
		ParentBuildId:        builder.Cft.GetParentBuildId(),
		ParentRequestUid:     builder.Cft.GetParentRequestUid(),
		Deadline:             builder.Cft.GetDeadline(),
		ContainerMetadata:    builder.Cft.GetContainerMetadata(),
		ContainerMetadataKey: builder.Cft.GetPrimaryDut().GetContainerMetadataKey(),
		PrimaryDut:           builder.Cft.GetPrimaryDut().GetDutModel(),
		BuildString:          builder.Cft.GetAutotestKeyvals()["build"],
		TestSuites:           testSuites,
		Keyvals:              keyvals,
		CompanionDuts:        []*labapi.DutModel{},
		OrderedTaskBuilders:  []DynamicTaskBuilder{},
	}
}

// tryAppendProvisionTask enforces the SkipProvision field and attempts
// to add provision steps to the ordered task list.
func (builder *DynamicTrv2FromCft) tryAppendProvisionTask(dynamic *DynamicTrv2Builder) {
	if builder.Cft.GetStepsConfig().GetHwTestConfig().GetSkipProvision() {
		return
	}

	dynamic.OrderedTaskBuilders = append(dynamic.OrderedTaskBuilders,
		DefaultDynamicProvisionTasksWrapper(builder.Cft))
}

// tryAppendTestTask enforces the SkipTestExecution field and attempts
// to add the test execution step to the ordered task list.
func (builder *DynamicTrv2FromCft) tryAppendTestTask(dynamic *DynamicTrv2Builder) {
	if builder.Cft.GetStepsConfig().GetHwTestConfig().GetSkipTestExecution() {
		return
	}

	testKey := common.CrosTest
	isCqRun := common.IsCqRun(builder.Cft.GetTestSuites())
	platform := common.GetBotProvider()
	if isCqRun && platform == common.BotProviderGce {
		testKey = common.CrosTestCqLight
	}
	dynamic.OrderedTaskBuilders = append(dynamic.OrderedTaskBuilders,
		DefaultDynamicTestTaskWrapper(testKey))
}

// tryAppendPostProcessTask enforces the SkipPostProcess field and attempts
// to add the post process to the ordered task list.
func (builder *DynamicTrv2FromCft) tryAppendPostProcessTask(dynamic *DynamicTrv2Builder) {
	if builder.Cft.GetStepsConfig().GetHwTestConfig().GetSkipPostProcess() {
		return
	}

	dynamic.OrderedTaskBuilders = append(dynamic.OrderedTaskBuilders,
		DefaultDynamicPostProcessTaskWrapper())
}

// tryAppendPublishTasks enforces the SkipAllResultPublish field and attempts
// to add the various publish steps to the ordered task list.
func (builder *DynamicTrv2FromCft) tryAppendPublishTasks(dynamic *DynamicTrv2Builder) {
	if builder.Cft.GetStepsConfig().GetHwTestConfig().GetSkipAllResultPublish() {
		return
	}

	builder.tryAppendRdbPublishTask(dynamic)
	builder.tryAppendGcsPublishTask(dynamic)
}

// tryAppendRdbPublishTask enforces the SkipRdbPublish field and attempts
// to add the rdb publish step to the ordered task list.
func (builder *DynamicTrv2FromCft) tryAppendRdbPublishTask(dynamic *DynamicTrv2Builder) {
	if builder.Cft.GetStepsConfig().GetHwTestConfig().GetSkipRdbPublish() {
		return
	}

	dynamic.OrderedTaskBuilders = append(dynamic.OrderedTaskBuilders,
		DefaultDynamicRdbPublishTaskWrapper(
			builder.Cft.GetPrimaryDut().GetProvisionState().GetSystemImage().GetSystemImagePath().GetPath()+common.SourceMetadataPath,
			builder.Cft.GetPrimaryDut().GetProvisionState().GetFirmware() != nil || len(builder.Cft.GetPrimaryDut().GetProvisionState().GetPackages()) > 0,
		))
}

// tryAppendGcsPublishTask enforces the SkipGcsPublish field and attempts
// to add the gcs publish step to the ordered task list.
func (builder *DynamicTrv2FromCft) tryAppendGcsPublishTask(dynamic *DynamicTrv2Builder) {
	if builder.Cft.GetStepsConfig().GetHwTestConfig().GetSkipGcsPublish() {
		return
	}

	dynamic.OrderedTaskBuilders = append(dynamic.OrderedTaskBuilders,
		DefaultDynamicGcsPublishTask)
}

// BuildBaseVariant constructs the base variant for rdb publishes.
func BuildBaseVariant(board, model, buildTarget string) map[string]string {
	baseVariant := map[string]string{}

	if board != "" {
		baseVariant["board"] = board
	}
	if model != "" {
		baseVariant["model"] = model
	}
	if buildTarget != "" {
		baseVariant["build_target"] = buildTarget
	}

	return baseVariant
}

// BuildCrosDutRequest is a helper function to construct a ContainerRequest
// with the CrosDut template, using a deviceId to dynamically execute it.
func BuildCrosDutRequest(deviceId *common.DeviceIdentifier) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: deviceId.GetCrosDutServer(),
		Container: &api.Template{
			Container: &api.Template_CrosDut{
				CrosDut: &api.CrosDutTemplate{},
			},
		},
		ContainerImageKey: common.CrosDut,
		DynamicDeps: []*api.DynamicDep{
			{
				Key:   common.CrosDutCacheServer,
				Value: common.NewPrimaryDeviceIdentifier().GetDevice("dut", "cacheServer", "address"),
			},
			{
				Key:   common.CrosDutDutAddress,
				Value: deviceId.GetDevice("dutServer"),
			},
		},
	}
}

// BuildProvisionRequest takes a Cft device to construct a ProvisionRequest.
// Checks provision state to determine install request.
func BuildProvisionRequest(deviceId *common.DeviceIdentifier, device *skylab_test_runner.CFTTestRequest_Device) *api.ProvisionTask {
	var installRequest *api.InstallRequest
	var serviceAddress string
	var startupRequest *api.ProvisionStartupRequest
	var deps []*api.DynamicDep
	var dynamicIdentifier string
	if IsAndroidProvisionState(device.GetProvisionState()) {
		serviceAddress = common.NewTaskIdentifier(common.AndroidProvision).AddDeviceId(deviceId).Id
		installRequest = &api.InstallRequest{
			PreventReboot: false,
			Metadata:      device.GetProvisionState().GetProvisionMetadata(),
		}
		startupRequest = &api.ProvisionStartupRequest{}
		deps = append(deps, []*api.DynamicDep{
			{
				Key:   common.ProvisionStartupDut,
				Value: deviceId.GetDevice("dut"),
			},
			{
				Key:   common.ProvisionStartupDutServer,
				Value: deviceId.GetCrosDutServer(),
			},
		}...)
		dynamicIdentifier = common.NewTaskIdentifier(common.AndroidProvision).AddDeviceId(deviceId).Id
	} else {
		serviceAddress = common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(deviceId).Id
		crosProvisionMetadata, _ := anypb.New(&api.CrOSProvisionMetadata{})
		installRequest = &api.InstallRequest{
			ImagePath:     device.GetProvisionState().GetSystemImage().GetSystemImagePath(),
			PreventReboot: false,
			Metadata:      crosProvisionMetadata,
		}
		dynamicIdentifier = common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(deviceId).Id
		if !ContainsFwProvisionState(device.GetProvisionState()) {
			deps = append(deps, &api.DynamicDep{
				Key:   common.CrosProvisionMetadataUpdateFirmware,
				Value: deviceId.GetUpdateFirmware(),
			})
		}
	}
	return &api.ProvisionTask{
		ServiceAddress: &labapi.IpEndpoint{},
		StartupRequest: startupRequest,
		InstallRequest: installRequest,
		DynamicDeps: append([]*api.DynamicDep{
			{
				Key:   common.ServiceAddress,
				Value: serviceAddress,
			},
		}, deps...),
		Target:            deviceId.Id,
		DynamicIdentifier: dynamicIdentifier,
	}
}

// BuildFwProvisionRequest creates a generic provision request using the FirmwareConfig
// within the device's provided provision state as part of the install request.
func BuildFwProvisionRequest(deviceId *common.DeviceIdentifier, device *skylab_test_runner.CFTTestRequest_Device) *api.ProvisionTask {
	startUpMetadata, _ := anypb.New(&api.FirmwareProvisionStartupMetadata{})
	installMetadata, _ := anypb.New(&api.FirmwareProvisionInstallMetadata{
		FirmwareConfig: device.GetProvisionState().GetFirmware(),
	})
	return &api.ProvisionTask{
		ServiceAddress: &labapi.IpEndpoint{},
		StartupRequest: &api.ProvisionStartupRequest{
			Metadata: startUpMetadata,
		},
		InstallRequest: &api.InstallRequest{
			Metadata: installMetadata,
		},
		DynamicDeps: []*api.DynamicDep{
			{
				Key:   common.ServiceAddress,
				Value: common.NewTaskIdentifier(common.FwProvision).AddDeviceId(deviceId).Id,
			},
			{
				Key:   common.ProvisionStartupDut,
				Value: deviceId.GetDevice("dut"),
			},
			{
				Key:   common.ProvisionStartupDutServer,
				Value: deviceId.GetCrosDutServer(),
			},
		},
		Target:            deviceId.Id,
		DynamicIdentifier: common.NewTaskIdentifier(common.FwProvision).Id,
	}
}

// BuildFwProvisionContainerRequest creates a container request for a certain deviceId,
// specifically geared towards supported cros-fw-provisions.
func BuildFwProvisionContainerRequest(deviceId *common.DeviceIdentifier) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: common.NewTaskIdentifier(common.FwProvision).AddDeviceId(deviceId).Id,
		ContainerImageKey: common.FwProvision,
		Container: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "cros-fw-provision",
					BinaryArgs: []string{
						"server",
						"-port", "0",
					},
					DockerArtifactDir: "/tmp/cros-fw-provision",
					AdditionalVolumes: []string{
						"/creds:/creds",
					},
				},
			},
		},
		DynamicDeps: []*api.DynamicDep{},
	}
}

// BuildProvisionContainerRequest constructs a ContainerRequest for a certain deviceId
// with variations for android devices.
func BuildProvisionContainerRequest(deviceId *common.DeviceIdentifier, isAndroid bool) *api.ContainerRequest {
	var container *api.Template
	var imageKey string
	var deps []*api.DynamicDep
	if isAndroid {
		imageKey = common.AndroidProvision
		container = &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "android-provision",
					BinaryArgs: []string{
						"server",
						"-port", "0",
					},
					DockerArtifactDir: "/tmp/provision",
					AdditionalVolumes: []string{
						"/creds:/creds",
					},
				},
			},
		}
	} else {
		imageKey = common.CrosProvision
		container = &api.Template{
			Container: &api.Template_CrosProvision{
				CrosProvision: &api.CrosProvisionTemplate{
					InputRequest: &api.CrosProvisionRequest{},
				},
			},
		}
		deps = []*api.DynamicDep{
			{
				Key:   "crosProvision.inputRequest.dut",
				Value: deviceId.GetDevice("dut"),
			},
			{
				Key:   "crosProvision.inputRequest.dutServer",
				Value: deviceId.GetCrosDutServer(),
			},
		}
	}
	return &api.ContainerRequest{
		DynamicIdentifier: common.NewTaskIdentifier(imageKey).AddDeviceId(deviceId).Id,
		Container:         container,
		ContainerImageKey: imageKey,
		DynamicDeps:       deps,
	}
}

// IsAndroidProvisionState checks if the metadata of the
// provision state can unmarshal to an android metadata.
func IsAndroidProvisionState(state *api.ProvisionState) bool {
	androidMetadata := &api.AndroidProvisionRequestMetadata{}
	err := state.GetProvisionMetadata().UnmarshalTo(androidMetadata)
	if err != nil {
		return false
	}
	return true
}

// ContainsFwProvisionState checks if there is fw provision info in the
// provision state.
func ContainsFwProvisionState(state *api.ProvisionState) bool {
	return state != nil && state.Firmware != nil
}

// BuildPostProcessContainerRequest constructs a ContainerRequest for
// post-process.
func BuildPostProcessContainerRequest(identifier string, deps []*api.DynamicDep) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: identifier,
		Container: &api.Template{
			Container: &api.Template_PostProcess{
				PostProcess: &api.PostProcessTemplate{},
			},
		},
		ContainerImageKey: common.PostProcess,
		DynamicDeps:       deps,
	}
}

// BuildPostProcessRequest constructs a PostProcessRequest with provided dependencies.
func BuildPostProcessRequest(dynamicID string) *api.PostTestTask {
	dutServer := common.NewPrimaryDeviceIdentifier().GetCrosDutServer()
	return &api.PostTestTask{
		ServiceAddress: &labapi.IpEndpoint{},
		StartUpRequest: &api.PostTestStartUpRequest{},
		RunActivitiesRequest: &api.RunActivitiesRequest{
			Requests: []*api.Request{
				{
					Request: &api.Request_GetFwInfoRequest{
						GetFwInfoRequest: &api.GetFWInfoRequest{},
					},
				},
				{
					Request: &api.Request_GetGfxInfoRequest{
						GetGfxInfoRequest: &api.GetGfxInfoRequest{},
					},
				},
			},
		},
		DynamicIdentifier: dynamicID,
		DynamicDeps: []*api.DynamicDep{
			{
				Key:   common.ServiceAddress,
				Value: common.PostProcess,
			},
			{
				Key:   "startUpRequest.dutServer",
				Value: dutServer,
			},
			{
				Key:   "runActivitiesRequest.dutServer",
				Value: dutServer,
			},
		},
	}
}

// BuildPublishContainerRequest constructs a ContainerRequest for cros-publish
// using parameters marking the publish type.
func BuildPublishContainerRequest(identifier string, publishType api.CrosPublishTemplate_PublishType, deps []*api.DynamicDep) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: identifier,
		Container: &api.Template{
			Container: &api.Template_CrosPublish{
				CrosPublish: &api.CrosPublishTemplate{
					PublishType: publishType,
				},
			},
		},
		ContainerImageKey: common.CrosPublish,
		DynamicDeps:       deps,
	}
}

// BuildPublishRequest constructs a PublishRequest with provided dependencies.
func BuildPublishRequest(dynamicId, artifactPath string, metadata *anypb.Any, deps []*api.DynamicDep) *api.PublishTask {
	return &api.PublishTask{
		ServiceAddress: &labapi.IpEndpoint{},
		PublishRequest: &api.PublishRequest{
			ArtifactDirPath: &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     artifactPath},
			Metadata: metadata,
		},
		DynamicDeps:       deps,
		DynamicIdentifier: dynamicId,
	}
}

// AppendTestTask takes a test container request and
// a test request and appends it to orderedTasks.
func AppendTestTask(
	orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task,
	containerRequest *api.ContainerRequest,
	testRequest *api.TestTask) {

	*orderedTasks = append(*orderedTasks, &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{
			containerRequest,
		},
		Task: &api.CrosTestRunnerDynamicRequest_Task_Test{
			Test: testRequest,
		},
	})
}

// AppendPublishTask takes a publish container request and
// a publish request and appends it to orderedTasks.
func AppendPublishTask(
	orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task,
	containerRequest *api.ContainerRequest,
	publishRequest *api.PublishTask,
	required bool) {

	*orderedTasks = append(*orderedTasks, &api.CrosTestRunnerDynamicRequest_Task{
		Required: required,
		OrderedContainerRequests: []*api.ContainerRequest{
			containerRequest,
		},
		Task: &api.CrosTestRunnerDynamicRequest_Task_Publish{
			Publish: publishRequest,
		},
	})
}

// PatchContainerMetadata loops through each container info and applies patches
// to certain containers based on the build version.
func PatchContainerMetadata(metadata *buildapi.ContainerMetadata, buildStr string) *buildapi.ContainerMetadata {
	containerMaps := map[string]*buildapi.ContainerImageMap{}
	buildNumber := ExtractBuildRNumber(buildStr)

	for metadataKey, containerMap := range metadata.GetContainers() {
		containers := map[string]*buildapi.ContainerImageInfo{}
		for containerKey, containerInfo := range containerMap.GetImages() {
			containers[containerKey] = containerInfo
		}

		if buildNumber < 124 {
			// R#'s < 124 will be missing cros-fw-provision.
			// Provide hard-coded sha256 for backwards compatibility.
			common.AddTestServiceContainerToImages(containers, "cros-fw-provision", common.DefaultCrosFwProvisionSha)
		}

		containerMaps[metadataKey] = &buildapi.ContainerImageMap{
			Images: containers,
		}
	}

	return &buildapi.ContainerMetadata{
		Containers: containerMaps,
	}
}

// ExtractBuildRNumber takes any build string and extracts
// the major digits found within the R#.
// If no R number match found, return -1.
func ExtractBuildRNumber(buildStr string) int {
	rNumberRegex := regexp.MustCompile(`R(\d+)`)
	matches := rNumberRegex.FindStringSubmatch(buildStr)
	if len(matches) == 0 {
		return -1
	}
	// If there is a match, then there will also be a captured R#.
	rNum, _ := strconv.Atoi(matches[1])
	return rNum
}

// DefaultDynamicProvisionTasksWrapper constructs the default provisions for a Cft request.
func DefaultDynamicProvisionTasksWrapper(cft *skylab_test_runner.CFTTestRequest) DynamicTaskBuilder {
	return func(builder *DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task {
		orderedTasks := &[]*api.CrosTestRunnerDynamicRequest_Task{}

		BuildPrimaryDutProvision(orderedTasks, cft)
		BuildCompanionDutProvisions(orderedTasks, cft)

		return *orderedTasks
	}
}

// buildPrimaryDutProvision attempts to use the PrimaryDut from CftTestRequest
// to construct a cros-dut and provision task request.
func BuildPrimaryDutProvision(orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task, cft *skylab_test_runner.CFTTestRequest) {
	if !cft.GetStepsConfig().GetHwTestConfig().GetSkipStartingDutService() {
		AppendDutTask(orderedTasks, BuildCrosDutRequest(common.NewPrimaryDeviceIdentifier()))
	}

	BuildProvision(common.NewPrimaryDeviceIdentifier(), cft.GetPrimaryDut(), orderedTasks)
}

// buildCompanionDutProvisions attempts to use the CompanionDuts from CftTestRequest
// to construct multiple cros-dut and provision task requests.
func BuildCompanionDutProvisions(orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task, cft *skylab_test_runner.CFTTestRequest) {
	deviceIds := map[string]struct{}{}
	for _, dut := range cft.GetCompanionDuts() {
		deviceId := common.NewCompanionDeviceIdentifier(dut.GetDutModel().GetBuildTarget())
		if _, ok := deviceIds[deviceId.Id]; ok {
			// deviceId already exists, try postfixing
			// Standard within swarming when there are duplicate boards
			// is to postfix with `_2`. (e.g. `brya | brya_2`)
			postfix := 2
			for {
				if _, ok := deviceIds[deviceId.AddPostfix(strconv.Itoa(postfix)).Id]; !ok {
					deviceId = deviceId.AddPostfix(strconv.Itoa(postfix))
					break
				}
				postfix += 1
			}
		}
		deviceIds[deviceId.Id] = struct{}{}
		if !cft.GetStepsConfig().GetHwTestConfig().GetSkipStartingDutService() {
			AppendDutTask(orderedTasks, BuildCrosDutRequest(deviceId))
		}

		BuildProvision(deviceId, dut, orderedTasks)
	}
}

// buildProvision checks for each possible type of provision that might occur
// and calls into the corresponding provision builder function.
func BuildProvision(
	deviceId *common.DeviceIdentifier,
	dut *skylab_test_runner.CFTTestRequest_Device,
	orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task) {

	AppendProvisionTask(orderedTasks,
		BuildProvisionContainerRequest(deviceId, IsAndroidProvisionState(dut.GetProvisionState())),
		BuildProvisionRequest(deviceId, dut))

	if ContainsFwProvisionState(dut.GetProvisionState()) {
		AppendProvisionTask(orderedTasks,
			BuildFwProvisionContainerRequest(deviceId),
			BuildFwProvisionRequest(deviceId, dut))
	}
}

// AppendDutTask takes a DutContainer request and appends it
// to orderedTasks.
func AppendDutTask(
	orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task,
	containerRequest *api.ContainerRequest) {

	*orderedTasks = append(*orderedTasks, &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{
			containerRequest,
		},
	})
}

// AppendProvisionTask takes a provision container request and
// a provision request and appends it to orderedTasks.
func AppendProvisionTask(
	orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task,
	containerRequest *api.ContainerRequest,
	provisionRequest *api.ProvisionTask) {

	*orderedTasks = append(*orderedTasks, &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{
			containerRequest,
		},
		Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
			Provision: provisionRequest,
		},
	})
}

// DefaultDynamicTestTask constructs a TestRequest using
// default dependencies.
func DefaultDynamicTestTaskWrapper(containerImageKey string) DynamicTaskBuilder {
	return func(builder *DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task {
		return []*api.CrosTestRunnerDynamicRequest_Task{
			{
				OrderedContainerRequests: []*api.ContainerRequest{
					{
						DynamicIdentifier: common.CrosTest,
						Container: &api.Template{
							Container: &api.Template_CrosTest{
								CrosTest: &api.CrosTestTemplate{},
							},
						},
						ContainerImageKey: containerImageKey,
					},
				},
				Task: &api.CrosTestRunnerDynamicRequest_Task_Test{
					Test: &api.TestTask{
						DynamicIdentifier: common.CrosTest,
						ServiceAddress:    &labapi.IpEndpoint{},
						TestRequest:       &api.CrosTestRequest{},
						DynamicDeps: []*api.DynamicDep{
							{
								Key:   common.ServiceAddress,
								Value: common.CrosTest,
							},
							{
								Key:   common.TestRequestTestSuites,
								Value: common.RequestTestSuites,
							},
							{
								Key:   common.TestRequestPrimary,
								Value: common.PrimaryDevice,
							},
							{
								Key:   common.TestRequestCompanions,
								Value: common.CompanionDevices,
							},
						},
					},
				},
			},
		}
	}
}

// DefaultDynamicPostProcessTaskWrapper creates the default post-process task.
func DefaultDynamicPostProcessTaskWrapper() DynamicTaskBuilder {
	return func(builder *DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task {
		return []*api.CrosTestRunnerDynamicRequest_Task{
			{
				OrderedContainerRequests: []*api.ContainerRequest{
					BuildPostProcessContainerRequest(common.PostProcess, nil),
				},
				Task: &api.CrosTestRunnerDynamicRequest_Task_PostTest{
					PostTest: BuildPostProcessRequest(common.PostProcess),
				},
				Required: true,
			},
		}
	}
}

// DefaultDynamicRdbPublishTaskWrapper creates the default rdb publish task.
func DefaultDynamicRdbPublishTaskWrapper(gsPath string, isDeploymentDirty bool) DynamicTaskBuilder {
	return func(builder *DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task {
		rdbPublishMetadata, _ := anypb.New(&testapi_metadata.PublishRdbMetadata{
			Sources: &testapi_metadata.PublishRdbMetadata_Sources{
				GsPath:            gsPath,
				IsDeploymentDirty: isDeploymentDirty,
			},
			TestResult: &artifact.TestResult{},
			BaseVariant: BuildBaseVariant(
				builder.PrimaryDut.GetBuildTarget(),
				builder.PrimaryDut.GetModelName(),
				builder.ContainerMetadataKey,
			),
			PostProcessResponses: &api.RunActivitiesResponse{},
		})
		return []*api.CrosTestRunnerDynamicRequest_Task{
			{
				OrderedContainerRequests: []*api.ContainerRequest{
					BuildPublishContainerRequest(common.RdbPublish, api.CrosPublishTemplate_PUBLISH_RDB, nil),
				},
				Task: &api.CrosTestRunnerDynamicRequest_Task_Publish{
					Publish: BuildPublishRequest(common.RdbPublish, common.RdbPublishTestArtifactDir, rdbPublishMetadata, []*api.DynamicDep{
						{
							Key:   common.ServiceAddress,
							Value: common.RdbPublish,
						},
						{
							Key:   "publishRequest.metadata.currentInvocationId",
							Value: "invocation-id",
						},
						{
							Key:   "publishRequest.metadata.testhausUrl",
							Value: "testhaus-url",
						},
						{
							Key:   "publishRequest.metadata.testResult",
							Value: common.NewTaskIdentifier(common.CrosTest).GetRpcResponse("rdbTestResult"),
						},
						{
							Key:   "publishRequest.metadata.postProcessResponses",
							Value: common.NewTaskIdentifier(common.PostProcess).GetRpcResponse("runActivities"),
						},
					}),
				},
				Required: true,
			},
		}
	}
}

// DefaultDynamicGcsPublishTask creates the default gsc publish task.
func DefaultDynamicGcsPublishTask(builder *DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task {
	gcsPublishMetadata, _ := anypb.New(&api.PublishGcsMetadata{
		GcsPath: &_go.StoragePath{
			HostType: _go.StoragePath_GS,
		},
	})
	return []*api.CrosTestRunnerDynamicRequest_Task{
		{
			OrderedContainerRequests: []*api.ContainerRequest{
				BuildPublishContainerRequest(common.GcsPublish, api.CrosPublishTemplate_PUBLISH_GCS, []*api.DynamicDep{
					{
						Key:   "crosPublish.publishSrcDir",
						Value: "env-TEMPDIR",
					},
				}),
			},
			Task: &api.CrosTestRunnerDynamicRequest_Task_Publish{
				Publish: BuildPublishRequest(common.GcsPublish, common.GcsPublishTestArtifactsDir, gcsPublishMetadata, []*api.DynamicDep{
					{
						Key:   common.ServiceAddress,
						Value: common.GcsPublish,
					},
					{
						Key:   "publishRequest.metadata.gcsPath.path",
						Value: "gcs-url",
					},
				}),
			},
			Required: true,
		},
	}
}
