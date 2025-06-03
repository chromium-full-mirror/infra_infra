// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package helpers

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

var (
	// Commonly used base-keys in the dynamic
	// lookup table.
	InstallPath  = LookupKey("installPath")
	Board        = LookupKey("board")
	AndroidBoard = LookupKey("androidBoard")
)

// CrosProvisionLookupValues adds a concrete helper layer
// for defining which values are being loaded into the dynamic
// lookup table.
type CrosProvisionLookupValues struct {
	Board       string
	InstallPath string
}

// AndroidProvisionLookupValues adds a concrete helper layer
// for defining which values are being loaded into the dynamic
// lookup table.
type AndroidProvisionLookupValues struct {
	Board string
}

// DynamicProvisionHelper tracks the count of provision types
// relating to lookup keys.
type DynamicProvisionHelper struct {
	crosProvisionCount    int
	androidProvisionCount int
}

// NewDynamicProvisionHelper creates an initialized helper
// with counts set to 0.
func NewDynamicProvisionHelper() *DynamicProvisionHelper {
	return &DynamicProvisionHelper{
		crosProvisionCount:    0,
		androidProvisionCount: 0,
	}
}

// ApplyCrosProvisionToLookup adds the cros provision lookup values to the lookup table
// using the globally defined common lookup keys.
// Increments the counter.
func (LH *DynamicProvisionHelper) ApplyCrosProvisionToLookup(lookupTable map[string]string, values *CrosProvisionLookupValues) {
	count := LH.crosProvisionCount
	lookupTable[Board.WithIndex(count).AsKey()] = values.Board
	lookupTable[InstallPath.WithIndex(count).AsKey()] = values.InstallPath

	LH.crosProvisionCount += 1
}

// ApplyAndroidProvisionToLookup adds the android provision lookup values to the lookup table
// using the globally defined common lookup keys.
// Increments the counter.
func (LH *DynamicProvisionHelper) ApplyAndroidProvisionToLookup(lookupTable map[string]string, values *AndroidProvisionLookupValues) {
	count := LH.androidProvisionCount
	lookupTable[AndroidBoard.WithIndex(count).AsKey()] = values.Board

	LH.androidProvisionCount += 1
}

func (LH *DynamicProvisionHelper) GenerateProvisionRequest(req *api.InternalTestplan, swarmingDef *api.SwarmingDefinition, isPrimary bool) error {
	var taskId *common.TaskIdentifier
	var provisionContainer *builders.ContainerBuilder
	var provisionInstallRequest *interfaces.ProvisionTaskInstallRequest
	deviceId := LH.getDeviceIdentifier(swarmingDef, isPrimary)

	switch swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		taskId = common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(deviceId)
		provisionContainer = NewCrosProvisionContainer(taskId)
		installPath := InstallPath.WithIndex(LH.crosProvisionCount).AsPlaceholder()
		provisionInstallRequest = DefaultCrosProvisionInstallRequest(installPath)
		LH.crosProvisionCount += 1

	case *dut_api.Dut_Android_:
		taskId = common.NewTaskIdentifier(common.AndroidProvision).AddDeviceId(deviceId)
		provisionContainer = NewAndroidProvisionContainer(taskId)
		provisionInstallRequest = DefaultAndroidProvisionInstallRequest()
		LH.androidProvisionCount += 1

	default:
		return nil
	}

	return GenerateProvisionRequest(
		req, taskId, deviceId,
		[]*builders.ContainerBuilder{
			NewServoNexusContainer(deviceId),
			NewCrosDutContainer(deviceId),
			provisionContainer,
		},
		provisionInstallRequest)
}

// getDeviceIdentifier grabs the correct placeholder value
// for the various flavors of dut types.
func (LH *DynamicProvisionHelper) getDeviceIdentifier(swarmingDef *api.SwarmingDefinition, isPrimary bool) *common.DeviceIdentifier {
	if isPrimary {
		return common.NewPrimaryDeviceIdentifier()
	}

	switch swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		return common.NewCompanionDeviceIdentifier(Board.WithIndex(LH.crosProvisionCount).AsPlaceholder())
	case *dut_api.Dut_Android_:
		return common.NewCompanionDeviceIdentifier(AndroidBoard.WithIndex(LH.androidProvisionCount).AsPlaceholder())
	}

	return nil
}

// GenerateProvisionRequest provides some defaults for a provision request,
// while also requiring some variable inputs between differing flavors.
func GenerateProvisionRequest(
	req *api.InternalTestplan,
	taskId *common.TaskIdentifier,
	deviceId *common.DeviceIdentifier,
	provisionContainers []*builders.ContainerBuilder,
	provisionInstallRequest *interfaces.ProvisionTaskInstallRequest) error {

	provisionGenerator := generators.NewProvisionTaskGenerator(
		taskId.Id, taskId.Id, deviceId.Id,
		provisionContainers,
		common.PrependTaskWrapper(common.FindFirst(api.FocalTaskFinder_TEST)),
		DefaultProvisionStartUpRequest(deviceId),
		provisionInstallRequest,
	)

	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, provisionGenerator.Generate)
}

// GenerateProvisionRequest provides more configurable variables.
func GenerateProvisionRequestForVM(
	req *api.InternalTestplan,
	taskId *common.TaskIdentifier,
	deviceId *common.DeviceIdentifier,
	provisionContainers []*builders.ContainerBuilder,
	provisionInstallRequest *interfaces.ProvisionTaskInstallRequest) error {

	provisionGenerator := generators.NewProvisionTaskGenerator(
		taskId.Id, taskId.Id, deviceId.Id,
		provisionContainers,
		common.PrependTaskWrapper(common.FindFirst(api.FocalTaskFinder_TEST)),
		DefaultProvisionStartUpRequestForVM(deviceId),
		provisionInstallRequest,
	)

	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, provisionGenerator.Generate)

}
