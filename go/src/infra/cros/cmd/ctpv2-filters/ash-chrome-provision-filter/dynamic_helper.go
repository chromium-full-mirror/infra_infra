// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"

	conf "go.chromium.org/chromiumos/config/go"
	gobuildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	commonlib "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

const (
	AshChromeProvision            = "ash-chrome-provision"
	AshChromeProvisionArtifactDir = "/tmp/ash-chrome-provision"
	AshChromeProvisionArgs        = "server -port 0"
)

var (
	// Ash-chrome lookup keys.
	AshChromeGcsPath     = helpers.LookupKey("ashChromeGcsPath")
	AshChromeBuildOutput = helpers.LookupKey("ashChromeBuildOutput")
)

type AshChromeProvisionLookupValues struct {
	AshChromeGcsPath        string
	AshChromeBuildOutputDir string
}

type DynamicAshChromeProvisionHelper struct {
	count int
}

func NewDynamicAshChromeProvisionHelper() *DynamicAshChromeProvisionHelper {
	return &DynamicAshChromeProvisionHelper{count: 0}
}

// GenerateProvisionRequest creates a dynamic ash-chrome-provision request for chromeos devices.
func (DH *DynamicAshChromeProvisionHelper) GenerateProvisionRequest(req *api.InternalTestplan, swarmingDef *api.SwarmingDefinition, swReq *api.LegacySW, isPrimary bool) error {
	deviceID := common.NewPrimaryDeviceIdentifier()
	if DH.count > 0 {
		deviceID = common.NewCompanionDeviceIdentifier(helpers.Board.WithIndex(DH.count).AsPlaceholder())
	}
	taskID := common.NewTaskIdentifier(AshChromeProvision).AddDeviceId(deviceID)
	containerBuilders := []*builders.ContainerBuilder{
		newAshChromeProvisionContainer(taskID),
	}

	installReq := DH.newAshChromeProvisionInstallRequest(swReq)

	DH.count += 1
	if installReq == nil {
		return nil
	}
	if commonlib.IsSupportedVMBoard(extractDutModel(swarmingDef).GetBuildTarget()) {
		return helpers.GenerateProvisionRequestForVM(req, taskID, deviceID, containerBuilders, installReq)
	}
	return helpers.GenerateProvisionRequest(
		req, taskID, deviceID,
		containerBuilders,
		installReq,
	)
}

// ApplyAshChromeProvisionToLookup sets the values provided by their availability within the lookup table.
func (DH *DynamicAshChromeProvisionHelper) ApplyAshChromeProvisionToLookup(lookupTable map[string]string, values *AshChromeProvisionLookupValues) {
	count := DH.count

	if values.AshChromeGcsPath != "" {
		lookupTable[AshChromeGcsPath.WithIndex(count).AsKey()] = values.AshChromeGcsPath
	}
	if values.AshChromeBuildOutputDir != "" {
		lookupTable[AshChromeBuildOutput.WithIndex(count).AsKey()] = values.AshChromeBuildOutputDir
	}

	DH.count += 1
}

// newAshChromeProvisionInstallRequest creates the ash-chrome-provision configs.
func (DH *DynamicAshChromeProvisionHelper) newAshChromeProvisionInstallRequest(swReq *api.LegacySW) *interfaces.ProvisionTaskInstallRequest {
	path := ""
	for _, kvs := range swReq.GetKeyValues() {
		if kvs.Key == commonbuilders.AshChromeGcsPath {
			path = kvs.Value
			break
		}
	}

	if path == "" {
		return nil
	}

	metadata, _ := anypb.New(&api.AshChromeProvisionInstallMetadata{
		AshChromeConfig: &gobuildapi.AshChromeConfig{
			ChromeBuilderArtifactPath: &conf.StoragePath{
				HostType: conf.StoragePath_GS,
				Path:     AshChromeGcsPath.WithIndex(DH.count).AsPlaceholder(),
			},
			BuildOutputDir: AshChromeBuildOutput.WithIndex(DH.count).AsPlaceholder(),
		},
		Strip: false,
	})

	return &interfaces.ProvisionTaskInstallRequest{
		StaticMetadata: metadata,
	}
}

// newAshChromeProvisionContainer creates a default generic container
// to run the ash-chrome-provision container.
func newAshChromeProvisionContainer(taskID *common.TaskIdentifier) *builders.ContainerBuilder {
	container := builders.NewContainerBuilder(
		taskID.Id, AshChromeProvision, "",
		AshChromeProvisionArtifactDir,
		fmt.Sprintf("%s %s", AshChromeProvision, AshChromeProvisionArgs),
	)
	// We can safely append these Envs regardless of skylab drone, cloudbot or vmlab
	// since TRv2 handles all situations correctly.
	container.AppendEnv("USE_GCE_METADATA=True",
		"USE_CLOUDBOT_SSH_CONFIG=/home/ash-chrome-provision")
	return container
}

func extractDutModel(swarmingDef *api.SwarmingDefinition) *dut_api.DutModel {
	switch dutType := swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		return dutType.Chromeos.GetDutModel()
	case *dut_api.Dut_Android_:
		return dutType.Android.GetDutModel()
	default:
		return nil
	}
}
