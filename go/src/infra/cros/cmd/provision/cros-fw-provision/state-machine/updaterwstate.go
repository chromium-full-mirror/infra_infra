// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// First step of FirmwareService State Machine. Installs RW firmware.
package state_machine

import (
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	firmwareservice "go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/service"
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
			return nil, api.InstallResponse_STATUS_DOWNLOADING_FIRMWARE_FAILED, err
		}
		futilityImageArgs = []string{fmt.Sprint("--image=", mainRwPath)}
	}

	// Get EC Image
	ecRwMetadata, ok := s.service.GetImageMetadata(s.service.GetEcRwPath())
	if ok && s.service.GetMainRwPath() != s.service.GetEcRwPath() {
		log.Printf("[FW Provisioning: Update RW] extracting EC-RW image to flash\n")
		ecRwPath, err = firmwareservice.PickAndExtractECImage(ctx, s.service.DUTServer, ecRwMetadata, s.service.GetEcRwPath(), s.service)
		if err != nil {
			return nil, api.InstallResponse_STATUS_DOWNLOADING_FIRMWARE_FAILED, err
		}

		newMainPath, err := firmwareservice.SwapECRWImage(ctx, s.service.DUTServer, mainRwPath, ecRwPath)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
		if newMainPath != mainRwPath {
			mainRwPath = newMainPath
			futilityImageArgs = []string{fmt.Sprint("--image=", mainRwPath)}
		}
	}

	log.Printf("[FW Provisioning: Update RW] checking versions")
	if err := s.service.ExtractFirmwareVersions(ctx, true /* WP */, futilityImageArgs, mainRwPath); err != nil {
		return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
	}
	versions, err := s.service.ActiveFirmwareVersions(ctx)
	if err != nil {
		return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
	}
	expected := &firmwareservice.FirmwareVersions{}
	expected.AP.Versions.RW = s.service.ExpectedVersions.AP.Versions.RW
	expected.EC.Versions.RW = s.service.ExpectedVersions.EC.Versions.RW
	expected.EC.Versions.RWHash = s.service.ExpectedVersions.EC.Versions.RWHash
	ok, err = s.service.CompareVersions(ctx, versions, expected)
	if err != nil {
		return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
	}
	if ok {
		log.Printf("[FW Provisioning: Update RW] Existing version is correct, skipping flashing")
		return nil, api.InstallResponse_STATUS_SUCCESS, nil
	}

	log.Printf("[FW Provisioning: Update RW] flashing RW firmware with futility\n")
	err = s.service.FlashWithFutility(ctx, true /* WP */, futilityImageArgs, mainRwPath, "")
	if err != nil {
		return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
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
