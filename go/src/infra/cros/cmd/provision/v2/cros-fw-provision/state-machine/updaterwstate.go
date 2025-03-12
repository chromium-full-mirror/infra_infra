// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// First step of FirmwareService State Machine. Installs RW firmware.
package state_machine

import (
	"context"
	"fmt"
	"log"

	firmwareservice "go.chromium.org/chromiumos/test/provision/v2/cros-fw-provision/service"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// FirmwareUpdateRwState updates firmware with write protection disabled.
type FirmwareUpdateRwState struct {
	service *firmwareservice.FirmwareService
}

// Execute flashes firmware using futility with write-protection enabled.
func (s FirmwareUpdateRwState) Execute(ctx context.Context, log *log.Logger) (*api.FirmwareProvisionResponse, api.InstallResponse_Status, error) {
	// form futility command args based on the request
	var futilityImageArgs []string
	var mainRwPath, ecRwPath string
	var err error

	// Get AP Image
	mainRwMetadata, ok := s.service.GetImageMetadata(s.service.GetMainRwPath())
	if ok {
		log.Printf("[FW Provisioning: Update RW] extracting AP image to flash\n")
		mainRwPath, err = firmwareservice.PickAndExtractMainImage(ctx, s.service.DUTServer, mainRwMetadata, s.service.GetMainRwPath(), s.service)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		futilityImageArgs = []string{fmt.Sprint("--image=", mainRwPath)}
	}

	// Get EC Image
	ecRwMetadata, ok := s.service.GetImageMetadata(s.service.GetEcRwPath())
	if ok {
		log.Printf("[FW Provisioning: Update RW] extracting EC-RW image to flash\n")
		ecRwPath, err = firmwareservice.PickAndExtractECImage(ctx, s.service.DUTServer, ecRwMetadata, s.service.GetEcRwPath(), s.service)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}

		newMainPath, err := firmwareservice.SwapECRWImage(ctx, s.service.DUTServer, mainRwPath, ecRwPath)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		if newMainPath != mainRwPath {
			mainRwPath = newMainPath
			futilityImageArgs = []string{fmt.Sprint("--image=", mainRwPath)}
		}
	}

	// TODO: Call fws.ExtractFirmwareVersions & fws.ActiveFirmwareVersions, and if they match, then skip flashing.
	// TODO: Remove fws.ExtractFirmwareVersions from fws.FlashWithFutility
	log.Printf("[FW Provisioning: Update RW] flashing RW firmware with futility\n")
	err = s.service.FlashWithFutility(ctx, true /* WP */, futilityImageArgs, mainRwPath, "")
	if err != nil {
		return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
	}

	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s FirmwareUpdateRwState) Next() ServiceState {
	return FirmwarePostInstallState(s)
}

const UpdateRwStateName = "Firmware Update RW"

func (s FirmwareUpdateRwState) Name() string {
	return UpdateRwStateName
}
