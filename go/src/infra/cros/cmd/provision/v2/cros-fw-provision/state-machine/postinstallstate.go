// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Last step of FirmwareService State Machine.
// Cleans up temporary folders and reboots the DUT.
package state_machine

import (
	"context"
	"log"

	"github.com/pkg/errors"

	"go.chromium.org/chromiumos/config/go/test/api"

	firmwareservice "go.chromium.org/infra/cros/cmd/provision/v2/cros-fw-provision/service"
)

// FirmwarePostInstallState cleans up temporary folders and reboots the DUT.
type FirmwarePostInstallState struct {
	service *firmwareservice.FirmwareService
}

// Execute deletes all folders with firmware image archives.
func (s FirmwarePostInstallState) Execute(ctx context.Context, log *log.Logger) (*api.FirmwareProvisionResponse, api.InstallResponse_Status, error) {
	fwMetadata := s.service.GetVersions()
	s.service.DeleteArchiveDirectories()
	if s.service.RestartRequired {
		err := s.service.RestartDut(ctx, false)
		if err != nil {
			return fwMetadata, api.InstallResponse_STATUS_DUT_UNREACHABLE_POST_FIRMWARE_UPDATE, firmwareservice.UnreachablePostProvisionErr(err)
		}
	}

	versions, err := s.service.ActiveFirmwareVersions(ctx)
	if err != nil {
		return nil, api.InstallResponse_STATUS_DUT_UNREACHABLE_POST_FIRMWARE_UPDATE, firmwareservice.FirmwareMismatchPostProvisionErr(err)
	}
	log.Printf("[FW Provisioning: Post Install] want firmware version ap:%+v ec:%+v\n", s.service.ExpectedVersions.AP.Versions, s.service.ExpectedVersions.EC.Versions)
	log.Printf("[FW Provisioning: Post Install]  got firmware version ap:%+v ec:%+v\n", versions.AP.Versions, versions.EC.Versions)
	if fwMetadata.ApRoVersion == "" {
		fwMetadata.ApRoVersion = versions.AP.Versions.RO
	}
	if fwMetadata.ApRwVersion == "" {
		fwMetadata.ApRwVersion = versions.AP.Versions.RW
	}
	if fwMetadata.EcRoVersion == "" {
		fwMetadata.EcRoVersion = versions.EC.Versions.RO
	}
	if fwMetadata.EcRwVersion == "" {
		fwMetadata.EcRwVersion = versions.EC.Versions.RW
	}

	if s.service.ExpectedVersions.AP.Versions.RO != "" && s.service.ExpectedVersions.AP.Versions.RO != versions.AP.Versions.RO {
		return fwMetadata, api.InstallResponse_STATUS_FIRMWARE_MISMATCH_POST_FIRMWARE_UPDATE,
			firmwareservice.FirmwareMismatchPostProvisionErr(errors.Errorf(
				"incorrect ap ro version got %q, want %q", versions.AP.Versions.RO, s.service.ExpectedVersions.AP.Versions.RO))
	}
	if s.service.ExpectedVersions.EC.Versions.RO != "" && s.service.ExpectedVersions.EC.Versions.RO != versions.EC.Versions.RO {
		return fwMetadata, api.InstallResponse_STATUS_FIRMWARE_MISMATCH_POST_FIRMWARE_UPDATE,
			firmwareservice.FirmwareMismatchPostProvisionErr(errors.Errorf(
				"incorrect ec ro version got %q, want %q", versions.EC.Versions.RO, s.service.ExpectedVersions.EC.Versions.RO))
	}
	if s.service.ExpectedVersions.AP.Versions.RW != "" && s.service.ExpectedVersions.AP.Versions.RW != versions.AP.Versions.RW {
		return fwMetadata, api.InstallResponse_STATUS_FIRMWARE_MISMATCH_POST_FIRMWARE_UPDATE,
			firmwareservice.FirmwareMismatchPostProvisionErr(errors.Errorf(
				"incorrect ap rw version got %q, want %q", versions.AP.Versions.RW, s.service.ExpectedVersions.AP.Versions.RW))
	}
	if s.service.ExpectedVersions.EC.Versions.RWHash != "" && s.service.ExpectedVersions.EC.Versions.RWHash != versions.EC.Versions.RWHash {
		return fwMetadata, api.InstallResponse_STATUS_FIRMWARE_MISMATCH_POST_FIRMWARE_UPDATE,
			firmwareservice.FirmwareMismatchPostProvisionErr(errors.Errorf(
				"incorrect ec rw hash got %q, want %q", versions.EC.Versions.RWHash, s.service.ExpectedVersions.EC.Versions.RWHash))
	}
	// If the hash is present, then don't check the RW version at all, because it's unreliable as it comes from the ec.bin, but the actual binary comes from the AP RW cbfs.
	if s.service.ExpectedVersions.EC.Versions.RWHash == "" && s.service.ExpectedVersions.EC.Versions.RW != "" && s.service.ExpectedVersions.EC.Versions.RW != versions.EC.Versions.RW {
		return fwMetadata, api.InstallResponse_STATUS_FIRMWARE_MISMATCH_POST_FIRMWARE_UPDATE,
			firmwareservice.FirmwareMismatchPostProvisionErr(errors.Errorf(
				"incorrect ec rw version got %q, want %q", versions.EC.Versions.RW, s.service.ExpectedVersions.EC.Versions.RW))
	}
	// Since the EC RWHash was verified, overwrite the expected EC RW version with the actual version, just in case we had the wrong one.
	fwMetadata.EcRwVersion = versions.EC.Versions.RW

	return fwMetadata, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s FirmwarePostInstallState) Next() ServiceState {
	return nil
}

const PostInstallStateName = "Post Install (cleanup/reboot)"

func (s FirmwarePostInstallState) Name() string {
	return PostInstallStateName
}
