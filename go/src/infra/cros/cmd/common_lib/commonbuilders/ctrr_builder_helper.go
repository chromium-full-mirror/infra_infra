// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commonbuilders defines common builders.
package commonbuilders

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi_metadata "go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"infra/cros/cmd/common_lib/common"
)

var (
	PullFromFirestore = []string{
		common.CrosDut,
		common.CrosProvision,
		common.ServoNexus,
	}
)

// buildDynamicRequest constructs the base DynamicTrv2Builder for DynamicTrv2FromCft.
func (builder *DynamicTrv2FromCft) buildDynamicRequest(firestoreDBName string, botDims []*buildbucketpb.StringPair, buildExperiments []string, isALRun bool) *DynamicTrv2Builder {
	keyvals := builder.Cft.GetAutotestKeyvals()
	if keyvals == nil {
		keyvals = make(map[string]string)
	}

	testSuites := builder.Cft.GetTestSuites()
	if testSuites == nil {
		testSuites = []*api.TestSuite{}
	}

	return &DynamicTrv2Builder{
		ParentBuildID:        builder.Cft.GetParentBuildId(),
		ParentRequestUID:     builder.Cft.GetParentRequestUid(),
		Deadline:             builder.Cft.GetDeadline(),
		ContainerMetadata:    builder.Cft.GetContainerMetadata(),
		ContainerMetadataKey: builder.Cft.GetPrimaryDut().GetContainerMetadataKey(),
		PrimaryDut:           builder.Cft.GetPrimaryDut().GetDutModel(),
		BuildString:          builder.Cft.GetAutotestKeyvals()["build"],
		TestSuites:           testSuites,
		Keyvals:              keyvals,
		CompanionDuts:        []*labapi.DutModel{},
		OrderedTaskBuilders:  []DynamicTaskBuilder{},
		FirestoreDBName:      firestoreDBName,
		IsALRun:              isALRun,
		BotDims:              botDims,
		BuildExperiments:     buildExperiments,
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
			dynamic.Is3DRun,
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

func (builder *DynamicTrv2Builder) buildCrosTestMetadata() (metadata *anypb.Any) {
	if len(builder.TestSuites) == 0 || len(builder.TestSuites[0].GetTestCaseIds().GetTestCaseIds()) == 0 {
		return
	}

	firstTest := builder.TestSuites[0].GetTestCaseIds().GetTestCaseIds()[0].Value
	gcsPath := builder.GcsArtifactPath

	if gcsPath != "" {
		if !strings.HasSuffix(gcsPath, "/") {
			gcsPath = gcsPath + "/"
		}

		if strings.HasPrefix(firstTest, "tast") {
			arg := &api.Arg{
				Flag:  "buildartifactsurl",
				Value: gcsPath,
			}
			tastExecutionMetadata := &api.TastExecutionMetadata{
				Args: []*api.Arg{arg},
			}

			metadata, _ = anypb.New(tastExecutionMetadata)
		}
	}

	return
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
func BuildCrosDutRequest(deviceID *common.DeviceIdentifier) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: deviceID.GetCrosDutServer(),
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
				Value: deviceID.GetDevice("dutServer"),
			},
		},
	}
}

// BuildProvisionRequest takes a Cft device to construct a ProvisionRequest.
// Checks provision state to determine install request.
func BuildProvisionRequest(deviceID *common.DeviceIdentifier, device *skylab_test_runner.CFTTestRequest_Device) *api.ProvisionTask {
	var installRequest *api.InstallRequest
	var serviceAddress string
	var startupRequest *api.ProvisionStartupRequest
	var deps []*api.DynamicDep
	var dynamicIdentifier string
	if IsAndroidProvisionState(device.GetProvisionState()) {
		serviceAddress = common.NewTaskIdentifier(common.AndroidProvision).AddDeviceID(deviceID).ID
		installRequest = &api.InstallRequest{
			PreventReboot: false,
			Metadata:      device.GetProvisionState().GetProvisionMetadata(),
		}
		startupRequest = &api.ProvisionStartupRequest{}
		deps = append(deps, []*api.DynamicDep{
			{
				Key:   common.ProvisionStartupDut,
				Value: deviceID.GetDevice("dut"),
			},
			{
				Key:   common.ProvisionStartupDutServer,
				Value: deviceID.GetCrosDutServer(),
			},
		}...)
		dynamicIdentifier = common.NewTaskIdentifier(common.AndroidProvision).AddDeviceID(deviceID).ID
	} else {
		serviceAddress = common.NewTaskIdentifier(common.CrosProvision).AddDeviceID(deviceID).ID
		crosProvisionMetadata, _ := anypb.New(&api.CrOSProvisionMetadata{})
		installRequest = &api.InstallRequest{
			ImagePath:     device.GetProvisionState().GetSystemImage().GetSystemImagePath(),
			PreventReboot: false,
			Metadata:      crosProvisionMetadata,
		}
		dynamicIdentifier = common.NewTaskIdentifier(common.CrosProvision).AddDeviceID(deviceID).ID
		if !ContainsFwProvisionState(device.GetProvisionState()) {
			deps = append(deps, &api.DynamicDep{
				Key:   common.CrosProvisionMetadataUpdateFirmware,
				Value: deviceID.GetUpdateFirmware(),
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
		Target:            deviceID.ID,
		DynamicIdentifier: dynamicIdentifier,
	}
}

// BuildFwProvisionRequest creates a generic provision request using the FirmwareConfig
// within the device's provided provision state as part of the install request.
func BuildFwProvisionRequest(deviceID *common.DeviceIdentifier, device *skylab_test_runner.CFTTestRequest_Device) *api.ProvisionTask {
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
				Value: common.NewTaskIdentifier(common.FwProvision).AddDeviceID(deviceID).ID,
			},
			{
				Key:   common.ProvisionStartupDut,
				Value: deviceID.GetDevice("dut"),
			},
			{
				Key:   common.ProvisionStartupDutServer,
				Value: deviceID.GetCrosDutServer(),
			},
		},
		Target:            deviceID.ID,
		DynamicIdentifier: common.NewTaskIdentifier(common.FwProvision).ID,
	}
}

// BuildFwProvisionContainerRequest creates a container request for a certain deviceId,
// specifically geared towards supported cros-fw-provisions.
func BuildFwProvisionContainerRequest(deviceID *common.DeviceIdentifier) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: common.NewTaskIdentifier(common.FwProvision).AddDeviceID(deviceID).ID,
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
func BuildProvisionContainerRequest(deviceID *common.DeviceIdentifier, isAndroid bool) *api.ContainerRequest {
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
				Value: deviceID.GetDevice("dut"),
			},
			{
				Key:   "crosProvision.inputRequest.dutServer",
				Value: deviceID.GetCrosDutServer(),
			},
		}
	}
	return &api.ContainerRequest{
		DynamicIdentifier: common.NewTaskIdentifier(imageKey).AddDeviceID(deviceID).ID,
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
	return err == nil
}

// ContainsFwProvisionState checks if there is fw provision info in the
// provision state.
func ContainsFwProvisionState(state *api.ProvisionState) bool {
	return state != nil && state.Firmware != nil
}

// BuildPostProcessContainerRequest constructs a ContainerRequest for
// post-process.
func BuildPostProcessContainerRequest(identifier string) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: identifier,
		Container: &api.Template{
			Container: &api.Template_PostProcess{
				PostProcess: &api.PostProcessTemplate{},
			},
		},
		ContainerImageKey: common.PostProcess,
		DynamicDeps: []*api.DynamicDep{
			{
				Key:   "postProcess.postProcessSrcDir",
				Value: "env-TEMPDIR",
			},
		},
	}
}

// BuildPostProcessRequest constructs a PostProcessRequest with provided dependencies.
func BuildPostProcessRequest(dynamicID string) *api.PostTestTask {
	dutServer := common.NewPrimaryDeviceIdentifier().GetCrosDutServer()
	testResultAnyProto, _ := anypb.New(&artifact.TestResult{})
	testResultAnyProtoDep := fmt.Sprintf("ANY(%s)=%s", testResultAnyProto.GetTypeUrl(), common.NewTaskIdentifier(common.CrosTest).GetRPCResponse("rdbTestResult"))
	return &api.PostTestTask{
		ServiceAddress: &labapi.IpEndpoint{},
		StartUpRequest: &api.PostTestStartUpRequest{},
		RunActivitiesRequest: &api.RunActivitiesRequest{
			Requests: []*api.Request{
				{
					Request: &api.Request_GetAvlInfoRequest{
						GetAvlInfoRequest: &api.GetAvlInfoRequest{},
					},
				},
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
				{
					Request: &api.Request_GetGscInfoRequest{
						GetGscInfoRequest: &api.GetGscInfoRequest{},
					},
				},
				{
					Request: &api.Request_GetServoInfoRequest{
						GetServoInfoRequest: &api.GetServoInfoRequest{},
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
			{
				Key:   "runActivitiesRequest.testResult",
				Value: testResultAnyProtoDep,
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
func BuildPublishRequest(dynamicID, artifactPath string, metadata *anypb.Any, deps []*api.DynamicDep, is3DRun bool) *api.PublishTask {
	return &api.PublishTask{
		ServiceAddress: &labapi.IpEndpoint{},
		PublishRequest: &api.PublishRequest{
			ArtifactDirPath: &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     artifactPath},
			Metadata: metadata,
			Is_3DRun: is3DRun,
		},
		DynamicDeps:       deps,
		DynamicIdentifier: dynamicID,
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
func PatchContainerMetadata(ctx context.Context, metadata *buildapi.ContainerMetadata, buildStr, creds, envVersion string, firestoreDBName string) *buildapi.ContainerMetadata {
	if metadata == nil {
		return nil
	}
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

		for _, firestoreDocName := range PullFromFirestore {
			containerInfo, err := common.FetchContainerInfoFromFirestore(ctx, firestoreDBName, creds, envVersion, firestoreDocName)
			common.LogWarningIfErr(ctx, err)
			if containerInfo != nil {
				containers[containerInfo.GetContainer().GetName()] = containerInfo.GetContainer()
			}
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

// BuildPrimaryDutProvision attempts to use the PrimaryDut from CftTestRequest
// to construct a cros-dut and provision task request.
func BuildPrimaryDutProvision(orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task, cft *skylab_test_runner.CFTTestRequest) {
	if !cft.GetStepsConfig().GetHwTestConfig().GetSkipStartingDutService() {
		AppendDutTask(orderedTasks, BuildCrosDutRequest(common.NewPrimaryDeviceIdentifier()))
	}

	BuildProvision(common.NewPrimaryDeviceIdentifier(), cft.GetPrimaryDut(), orderedTasks)
}

// BuildCompanionDutProvisions attempts to use the CompanionDuts from CftTestRequest
// to construct multiple cros-dut and provision task requests.
func BuildCompanionDutProvisions(orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task, cft *skylab_test_runner.CFTTestRequest) {
	deviceIds := map[string]struct{}{}
	for _, dut := range cft.GetCompanionDuts() {
		deviceID := common.NewCompanionDeviceIdentifier(dut.GetDutModel().GetBuildTarget())
		if _, ok := deviceIds[deviceID.ID]; ok {
			// deviceId already exists, try postfixing
			// Standard within swarming when there are duplicate boards
			// is to postfix with `_2`. (e.g. `brya | brya_2`)
			postfix := 2
			for {
				if _, ok := deviceIds[deviceID.AddPostfix(strconv.Itoa(postfix)).ID]; !ok {
					deviceID = deviceID.AddPostfix(strconv.Itoa(postfix))
					break
				}
				postfix += 1
			}
		}
		deviceIds[deviceID.ID] = struct{}{}
		if !cft.GetStepsConfig().GetHwTestConfig().GetSkipStartingDutService() {
			AppendDutTask(orderedTasks, BuildCrosDutRequest(deviceID))
		}

		BuildProvision(deviceID, dut, orderedTasks)
	}
}

// BuildProvision checks for each possible type of provision that might occur
// and calls into the corresponding provision builder function.
func BuildProvision(
	deviceID *common.DeviceIdentifier,
	dut *skylab_test_runner.CFTTestRequest_Device,
	orderedTasks *[]*api.CrosTestRunnerDynamicRequest_Task) {

	AppendProvisionTask(orderedTasks,
		BuildProvisionContainerRequest(deviceID, IsAndroidProvisionState(dut.GetProvisionState())),
		BuildProvisionRequest(deviceID, dut))

	if ContainsFwProvisionState(dut.GetProvisionState()) {
		AppendProvisionTask(orderedTasks,
			BuildFwProvisionContainerRequest(deviceID),
			BuildFwProvisionRequest(deviceID, dut))
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

// DefaultDynamicTestTaskWrapper constructs a TestRequest using
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
						TestRequest: &api.CrosTestRequest{
							Metadata: builder.buildCrosTestMetadata(),
						},
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
							{
								Key:   common.TestRequestPrimary + ".dutServer",
								Value: common.NewPrimaryDeviceIdentifier().GetCrosDutServer(),
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
					BuildPostProcessContainerRequest(common.PostProcess),
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
func DefaultDynamicRdbPublishTaskWrapper(gsPath string, isDeploymentDirty, is3DRun bool) DynamicTaskBuilder {
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
			PostProcessResponses:      &api.RunActivitiesResponse{},
			FirmwareProvisionResponse: &api.FirmwareProvisionResponse{},
		})
		return []*api.CrosTestRunnerDynamicRequest_Task{
			{
				OrderedContainerRequests: []*api.ContainerRequest{
					BuildPublishContainerRequest(common.RdbPublish, api.CrosPublishTemplate_PUBLISH_RDB, []*api.DynamicDep{
						{
							Key:   "crosPublish.publishSrcDir",
							Value: "env-TEMPDIR",
						},
					}),
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
							Value: common.NewTaskIdentifier(common.CrosTest).GetRPCResponse("rdbTestResult"),
						},
						{
							Key:   "publishRequest.metadata.postProcessResponses",
							Value: common.NewTaskIdentifier(common.PostProcess).GetRPCResponse("runActivities"),
						},
						{
							Key:   "publishRequest.metadata.firmwareProvisionResponse",
							Value: common.NewTaskIdentifier(common.FwProvision).AddDeviceID(common.NewPrimaryDeviceIdentifier()).GetRPCResponse("install", "metadata"),
						},
					}, is3DRun),
				},
				Required: true,
			},
		}
	}
}

// DefaultDynamicGcsPublishTask creates the default gsc publish task.
func DefaultDynamicGcsPublishTask(builder *DynamicTrv2Builder) []*api.CrosTestRunnerDynamicRequest_Task {
	product := common.GetProductName(builder.PrimaryDut, builder.BotDims, builder.BuildString)
	gcsPublishMetadata, _ := anypb.New(&api.PublishGcsMetadata{
		GcsPath: &_go.StoragePath{
			HostType: _go.StoragePath_GS,
		},
		XtsArchiverMetadata: &api.XtsArchiverMetadata{
			AlRun:                builder.IsALRun,
			Product:              product,
			Build:                builder.BuildString,
			ParentSwarmingTaskId: builder.Keyvals["parent_job_id"],
		},
		EnableXtsArchiver: slices.Contains(builder.BuildExperiments, common.EnableXTSArchiverExperiment),
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
					{
						Key:   "publishRequest.metadata.xtsArchiverMetadata.resultsGcsPrefix",
						Value: common.XTSArchiverResultsGCS,
					},
					{
						Key:   "publishRequest.metadata.xtsArchiverMetadata.apfeGcsPrefix",
						Value: common.XTSArchiverAPFEGCS,
					},
				}, builder.Is3DRun),
			},
			Required: true,
		},
	}
}

// BuildServoNexusContainerRequest constructs a ContainerRequest for servo-nexus
func BuildServoNexusContainerRequest(deviceID *common.DeviceIdentifier) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: common.NewTaskIdentifier(common.ServoNexus).AddDeviceID(deviceID).ID,
		Container: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName:        "cros-servod",
					BinaryArgs:        []string{"server", "-server_port", "0"},
					DockerArtifactDir: "/tmp/servod",
					AdditionalVolumes: []string{"/creds:/creds"},
				},
			},
		},
		ContainerImageKey: common.ServoNexus,
	}
}
