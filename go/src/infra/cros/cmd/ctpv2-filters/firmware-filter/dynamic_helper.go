// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	goconfig "go.chromium.org/chromiumos/config/go"
	gobuildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/builders"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/helpers"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/interfaces"
)

const (
	// Firmware request information.
	CrosFwProvision            = "cros-fw-provision"
	CrosFwProvisionArtifactDir = "/tmp/cros-fw-provision"
	CrosFwProvisionArgs        = "server -port 0"
)

var (
	// Firmware lookup keys.
	FirmwareRo = helpers.LookupKey("apFwRo")
	FirmwareRw = helpers.LookupKey("apFwRw")
)

// FirmwareProvisionLookupValues stores the relevant firmware
// values that need to be stored within the lookup tables.
type FirmwareProvisionLookupValues struct {
	Ro string
	Rw string
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

	DH.count += 1
}

// GenerateProvisionRequest creates a dynamic firmware request for chromeos devices.
func (DH *DynamicFirmwareProvisionHelper) GenerateProvisionRequest(req *api.InternalTestplan, log *log.Logger) error {
	deviceID := common.NewPrimaryDeviceIdentifier()
	if DH.count > 0 {
		deviceID = common.NewCompanionDeviceIdentifier(helpers.Board.WithIndex(DH.count).AsPlaceholder())
	}
	taskID := common.NewTaskIdentifier(CrosFwProvision).AddDeviceId(deviceID)
	containerBuilders := []*builders.ContainerBuilder{
		newFirmwareProvisionContainer(taskID),
	}

	return helpers.GenerateProvisionRequest(
		req, taskID, deviceID,
		containerBuilders,
		DH.newFirmwareInstallRequest(),
	)
}

// newFirmwareInstallRequest creates placeholder firmware configs
// based on the provided firmware specs.
func (DH *DynamicFirmwareProvisionHelper) newFirmwareInstallRequest() *interfaces.ProvisionTaskInstallRequest {
	var ro *gobuildapi.FirmwarePayload
	var rw *gobuildapi.FirmwarePayload

	if DH.specs.Ro != "" {
		ro = newFirmwarePayload(FirmwareRo.WithIndex(DH.count).AsPlaceholder())
	}

	if DH.specs.Rw != "" {
		rw = newFirmwarePayload(FirmwareRw.WithIndex(DH.count).AsPlaceholder())
	}

	fwProvisionMetadata, _ := anypb.New(&api.FirmwareProvisionInstallMetadata{
		FirmwareConfig: &gobuildapi.FirmwareConfig{
			MainRoPayload: ro,
			EcRoPayload:   ro,
			MainRwPayload: rw,
		},
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
