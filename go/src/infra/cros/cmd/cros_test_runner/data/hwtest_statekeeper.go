// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"container/list"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/build/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/dutstate"
	vmlabapi "go.chromium.org/infra/libs/vmlab/api"
)

// HwTestStateKeeper represents all the data hw test execution flow requires.
type HwTestStateKeeper struct {
	interfaces.StateKeeper

	// Build related
	BuildState *build.State

	// Set from input
	CftTestRequest        *skylab_test_runner.CFTTestRequest
	CrosTestRunnerRequest *testapi.CrosTestRunnerDynamicRequest
	CommonConfig          *skylab_test_runner.CommonConfig
	IsAlRun               bool

	// Request Queues
	ContainerQueue *list.List
	ProvisionQueue *list.List
	PreTestQueue   *list.List
	TestQueue      *list.List
	PostTestQueue  *list.List
	PublishQueue   *list.List
	GenericQueue   *list.List

	// Dictionaries
	Injectables        *common.InjectableStorage
	ContainerInstances map[string]interfaces.ContainerInterface
	ContainerImages    map[string]*api.ContainerImageInfo

	// Dut related
	HostName                 string
	DeviceIdentifiers        []string
	Devices                  map[string]*testapi.CrosTestRequest_Device
	PrimaryDevice            *testapi.CrosTestRequest_Device
	PrimaryDeviceMetadata    *skylab_test_runner.CFTTestRequest_Device
	CompanionDevices         []*testapi.CrosTestRequest_Device
	CompanionDevicesMetadata []*skylab_test_runner.CFTTestRequest_Device
	DutTopology              *labapi.DutTopology
	DutServerAddress         *labapi.IpEndpoint
	AndroidDutServerAddress  *labapi.IpEndpoint
	CurrentDutState          dutstate.State
	PrimaryDutModel          *labapi.DutModel
	CompanionDutModels       []*labapi.DutModel
	CacheServer              *labapi.IpEndpoint
	// Only when DUT is a VM
	DutVmGceImage   *vmlabapi.GceImage
	DutVm           *vmlabapi.VmInstance
	LeaseVMResponse *testapi.LeaseVMResponse
	HostIp          string

	// Provision related
	InstallMetadata    *anypb.Any
	ProvisionResponses map[string][]*testapi.InstallResponse
	// UpdateFirmwares maps each board to whether
	// or not it should update firmware, parsed from
	// the input's config and set while parsing
	// the dut topology.
	UpdateFirmwares map[string]bool

	// Test related
	TestArgs               *testapi.AutotestExecutionMetadata
	TestResponses          *testapi.CrosTestResponse
	TestExecutionStartTime *timestamppb.Timestamp
	TestExecutionEndTime   *timestamppb.Timestamp

	// Publish related
	GcsURL              string
	TesthausURL         string
	GcsPublishSrcDir    string
	RdbPublishSrcDir    string
	CurrentInvocationId string
	TkoPublishSrcDir    string
	CpconPublishSrcDir  string
	TestResultForRdb    *artifactpb.TestResult
	BaseVariant         map[string]string

	// Build related
	SkylabResult *skylab_test_runner.Result

	// Tools and their related dependencies
	Ctr                   *crostoolrunner.CrosToolRunner
	DockerKeyFileLocation string

	ANTSInvocationID string

	//Curating logs to be consumed by AI for execution context
	ExecutionAIContext string
}

func NewHwTestStateKeeper() *HwTestStateKeeper {
	return &HwTestStateKeeper{
		ContainerQueue:           list.New(),
		ProvisionQueue:           list.New(),
		PreTestQueue:             list.New(),
		TestQueue:                list.New(),
		PostTestQueue:            list.New(),
		PublishQueue:             list.New(),
		GenericQueue:             list.New(),
		Injectables:              common.NewInjectableStorage(),
		ContainerInstances:       make(map[string]interfaces.ContainerInterface),
		ContainerImages:          make(map[string]*api.ContainerImageInfo),
		DeviceIdentifiers:        []string{},
		Devices:                  make(map[string]*testapi.CrosTestRequest_Device),
		CompanionDevices:         []*testapi.CrosTestRequest_Device{},
		CompanionDevicesMetadata: []*skylab_test_runner.CFTTestRequest_Device{},
		ProvisionResponses:       make(map[string][]*testapi.InstallResponse),
		PrimaryDutModel:          &labapi.DutModel{},
		CompanionDutModels:       []*labapi.DutModel{},
		UpdateFirmwares:          map[string]bool{},
	}
}

// AppendToAIExecutionContext exposes an api to easily append to AIExecutionContext
func (sk *HwTestStateKeeper) AppendToAIExecutionContext(context string) {
	sk.ExecutionAIContext = sk.ExecutionAIContext + "\n" + context
}
