// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	goconfig "go.chromium.org/chromiumos/config/go"
	gobuildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"

	commonlib "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/test_plan"
)

const (
	// Firmware request information.
	CrosFwProvision            = "cros-fw-provision"
	CrosFwProvisionArtifactDir = "/tmp/cros-fw-provision"
	CrosFwProvisionArgs        = "server -port 0"
)

var (
	// Firmware lookup keys.
	FirmwareRo   = helpers.LookupKey("apFwRo")
	FirmwareRw   = helpers.LookupKey("apFwRw")
	FirmwareECRO = helpers.LookupKey("ecFwRo")
	FirmwareECRW = helpers.LookupKey("ecFwRw")
)

// FirmwareProvisionLookupValues stores the relevant firmware
// values that need to be stored within the lookup tables.
type FirmwareProvisionLookupValues struct {
	Ro   string
	Rw   string
	ECRO string
	ECRW string
}

// DynamicFirmwareProvisionHelper provides a wrapper around
// the logic building out the dynamic firmware provision requests.
type DynamicFirmwareProvisionHelper struct {
	specs *FirmwareSpecs

	count int
}

func NewDynamicFirmwareProvisionHelper(specs *FirmwareSpecs) *DynamicFirmwareProvisionHelper {
	return &DynamicFirmwareProvisionHelper{
		specs: specs,
		count: 0,
	}
}

// ApplyFirmwareProvisionToLookup sets the values provided by their availability
// within the lookup table.
func (DH *DynamicFirmwareProvisionHelper) ApplyFirmwareProvisionToLookup(lookupTable map[string]string, values *FirmwareProvisionLookupValues) {
	count := DH.count

	if values.Ro != "" {
		lookupTable[FirmwareRo.WithIndex(count).AsKey()] = values.Ro
	}
	if values.Rw != "" {
		lookupTable[FirmwareRw.WithIndex(count).AsKey()] = values.Rw
	}
	if values.ECRO != "" {
		lookupTable[FirmwareECRO.WithIndex(count).AsKey()] = values.ECRO
	}
	if values.ECRW != "" {
		lookupTable[FirmwareECRW.WithIndex(count).AsKey()] = values.ECRW
	}

	DH.count += 1
}

// GenerateProvisionRequest creates a dynamic firmware request for chromeos devices.
func (DH *DynamicFirmwareProvisionHelper) GenerateProvisionRequest(ctx context.Context, commonParams *commonlib.CommonFilterParams, req *api.InternalTestplan, log *log.Logger) error {
	deviceID := common.NewPrimaryDeviceIdentifier()
	if DH.count > 0 {
		deviceID = common.NewCompanionDeviceIdentifier(helpers.Board.WithIndex(DH.count).AsPlaceholder())
	}
	taskID := common.NewTaskIdentifier(CrosFwProvision).AddDeviceId(deviceID)
	containerBuilders := []*builders.ContainerBuilder{}

	// cros-dut isn't started when provisioning an AlOS device. The al-provision-filter
	// replaces the entire list of containers to start and cros-dut gets dropped.
	// cros-fw-provision requires cros-dut, so we add the container when
	// targeting Android.
	if test_plan.IsAlRun(req) {
		// The containerMetadata isn't populated for AL runs, so the filter needs
		// to manually look up the path to each container.
		var crosDutPath, crosFwProvisionPath string
		// We don't have an easy way to mock out commonlib.ProcessContainerPath, so
		// just skip testing this.
		if commonParams.AuthHelper != nil {
			var e error
			crosDutPath, e = commonlib.ProcessContainerPath(ctx, commonParams, "", "cros-dut")
			if e != nil {
				return fmt.Errorf("Error looking up cros-dut container path: %w", e)
			}

			crosFwProvisionPath, e = commonlib.ProcessContainerPath(ctx, commonParams, "", "cros-fw-provision")
			if e != nil {
				return fmt.Errorf("Error looking up cros-fw-provision container path: %w", e)
			}
		}
		crosDutContainer := helpers.NewCrosDutContainer(deviceID)
		crosDutContainer.ContainerImagePath = crosDutPath
		containerBuilders = append(containerBuilders, crosDutContainer)

		crosFwProvisionContainer := newFirmwareProvisionContainer(taskID)
		crosFwProvisionContainer.ContainerImagePath = crosFwProvisionPath
		containerBuilders = append(containerBuilders, crosFwProvisionContainer)
	} else {
		containerBuilders = append(containerBuilders, newFirmwareProvisionContainer(taskID))
	}

	return helpers.GenerateProvisionRequest(
		req, taskID, deviceID,
		containerBuilders,
		DH.newFirmwareInstallRequest(test_plan.IsAlRun(req)),
	)
}

// newFirmwareInstallRequest creates placeholder firmware configs
// based on the provided firmware specs.
func (DH *DynamicFirmwareProvisionHelper) newFirmwareInstallRequest(isAndroid bool) *interfaces.ProvisionTaskInstallRequest {
	var ro *gobuildapi.FirmwarePayload
	var rw *gobuildapi.FirmwarePayload
	var ecro *gobuildapi.FirmwarePayload
	var ecrw *gobuildapi.FirmwarePayload

	if DH.specs.Ro != "" {
		ro = newFirmwarePayload(FirmwareRo.WithIndex(DH.count).AsPlaceholder())
		ecro = ro
	}

	if DH.specs.Rw != "" {
		rw = newFirmwarePayload(FirmwareRw.WithIndex(DH.count).AsPlaceholder())
		ecrw = rw
	}

	if DH.specs.ECRO != "" {
		ecro = newFirmwarePayload(FirmwareECRO.WithIndex(DH.count).AsPlaceholder())
	}

	if DH.specs.ECRW != "" {
		ecrw = newFirmwarePayload(FirmwareECRW.WithIndex(DH.count).AsPlaceholder())
	}

	var os = api.FirmwareProvisionInstallMetadata_CHROMEOS
	if isAndroid {
		os = api.FirmwareProvisionInstallMetadata_ANDROID
	}

	fwProvisionMetadata, _ := anypb.New(&api.FirmwareProvisionInstallMetadata{
		FirmwareConfig: &gobuildapi.FirmwareConfig{
			MainRoPayload: ro,
			EcRoPayload:   ecro,
			MainRwPayload: rw,
			EcRwPayload:   ecrw,
		},
		Os: os,
	})
	return &interfaces.ProvisionTaskInstallRequest{
		StaticMetadata: fwProvisionMetadata,
	}
}

// newFirmwareProvisionContainer creates a default generic container
// to run the cros-fw-provision container.
func newFirmwareProvisionContainer(taskID *common.TaskIdentifier) *builders.ContainerBuilder {
	container := builders.NewContainerBuilder(
		taskID.Id, CrosFwProvision, "",
		CrosFwProvisionArtifactDir,
		fmt.Sprintf("%s %s", CrosFwProvision, CrosFwProvisionArgs),
	)
	return container
}

// newFirmwarePayload builds up a basic firmware payload object.
func newFirmwarePayload(path string) *gobuildapi.FirmwarePayload {
	return &gobuildapi.FirmwarePayload{
		FirmwareImage: &gobuildapi.FirmwarePayload_FirmwareImagePath{
			FirmwareImagePath: &goconfig.StoragePath{
				HostType: goconfig.StoragePath_GS,
				Path:     path,
			},
		},
	}
}
