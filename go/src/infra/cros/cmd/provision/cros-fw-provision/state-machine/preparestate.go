// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	firmwareservice "go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/service"
)

type FirmwarePrepareState struct {
	service *firmwareservice.FirmwareService
}

func NewFirmwarePrepareState(service *firmwareservice.FirmwareService) ServiceState {
	return FirmwarePrepareState{
		service: service,
	}
}

// FirmwarePrepareState downloads and extracts every image from the request.
// The already downloaded images will not be downloaded and extracted again.
func (s FirmwarePrepareState) Execute(ctx context.Context, log *log.Logger) (*api.FirmwareProvisionResponse, api.InstallResponse_Status, error) {
	firmwareImageDestination := "DUT"
	if s.service.IsServoUsed() {
		firmwareImageDestination = "ServoHost"
	}
	log.Printf("[FW Provisioning: Prepare FW] preparing for %v\n", firmwareImageDestination)
	if err := s.service.WaitForReconnect(ctx); err != nil {
		return nil, api.InstallResponse_STATUS_DUT_UNREACHABLE_PRE_PROVISION, err
	}

	// Android doesn't have crosid or config.yaml
	if s.service.IsAndroid() {
		if err := s.service.ReadFRID(ctx); err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
	} else {
		if err := s.service.ReadConfigYAML(ctx); err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
	}
	if mainRw := s.service.GetMainRwPath(); len(mainRw) > 0 {
		if err := s.service.DownloadAndProcess(ctx, mainRw); err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
	}
	if mainRo := s.service.GetMainRoPath(); len(mainRo) > 0 {
		if err := s.service.DownloadAndProcess(ctx, mainRo); err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
	}
	if ecRoPath := s.service.GetEcRoPath(); len(ecRoPath) > 0 {
		if err := s.service.DownloadAndProcess(ctx, ecRoPath); err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
	}
	if ecRwPath := s.service.GetEcRwPath(); len(ecRwPath) > 0 {
		if err := s.service.DownloadAndProcess(ctx, ecRwPath); err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, err
		}
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s FirmwarePrepareState) Next() ServiceState {
	if s.service.UpdateRo() {
		return FirmwareUpdateRoState(s)
	} else {
		return FirmwareUpdateRwState(s)
	}
}

const PrepareStateName = "Firmware Prepare (download/extract archives)"

func (s FirmwarePrepareState) Name() string {
	return PrepareStateName
}
