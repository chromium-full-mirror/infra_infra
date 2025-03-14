// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// First step of FirmwareService State Machine. Installs RW firmware.
package state_machine

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/pkg/errors"

	"go.chromium.org/chromiumos/config/go/test/api"
	firmwareservice "go.chromium.org/chromiumos/test/provision/v2/cros-fw-provision/service"
)

// FirmwareUpdateRoState updates firmware with write protection disabled.
type FirmwareUpdateRoState struct {
	service *firmwareservice.FirmwareService
}

// Execute flashes firmware with write-protection disabled using futility.
func (s FirmwareUpdateRoState) Execute(ctx context.Context, log *log.Logger) (*api.FirmwareProvisionResponse, api.InstallResponse_Status, error) {
	connection := s.service.GetConnectionToFlashingDevice()

	// form futility command args based on the request
	var futilityImageArgs []string
	// Detailed Request
	var mainRoPath, ecRoPath, ecRwPath string
	var err error
	board := s.service.GetBoard()
	ecRoMetadata, ok := s.service.GetImageMetadata(s.service.GetEcRoPath())
	// Skip EC provisioning on Wilco duts that don't have Chrome ECs. Since there are only 2,
	// hard code them here.
	if ok && board != "drallion" && board != "sarien" {
		// Check for RO_AT_BOOT before downloading images, as /var/tmp may be cleared on reboot.
		out, _, err := firmwareservice.RunDUTCommand(ctx, s.service.DUTServer, time.Minute, "ectool", []string{"flashprotect"}, nil)
		log.Printf("flashprotect: %s %v", out, err)
		roAtBootRe := regexp.MustCompile("Flash protect flags:.*ro_at_boot")
		if roAtBootRe.MatchString(out) {
			log.Printf("RO_AT_BOOT is not clear: Rebooting EC")
			// Ignore the err, because the ssh connection is expected to break
			firmwareservice.RunDUTCommand(ctx, s.service.DUTServer, time.Minute, "ectool", []string{"reboot_ec"}, nil)
			for range 20 {
				_, _, err := firmwareservice.RunDUTCommand(ctx, s.service.DUTServer, time.Minute, "true", nil, nil)
				if err == nil {
					break
				}
				log.Printf("%s", err)
				time.Sleep(10 * time.Second)
			}
			out, _, err := firmwareservice.RunDUTCommand(ctx, s.service.DUTServer, time.Minute, "ectool", []string{"flashprotect", "now", "disable"}, nil)
			log.Printf("flashprotect after reboot: %s %s", out, err)
		}
		log.Printf("[FW Provisioning: Update RO] extracting EC image to flash\n")

		_, _, err = firmwareservice.RunDUTCommand(ctx, s.service.DUTServer, time.Minute, "mkdir", []string{"-p", firmwareservice.Escape(ecRoMetadata.ArchiveDir)}, nil)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}

		ecRoPath, err = firmwareservice.PickAndExtractECImage(ctx, s.service.DUTServer, ecRoMetadata, s.service.GetEcRoPath(), s.service)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		if s.service.IsServoUsed() {
			log.Printf("[FW Provisioning: Update RO] separately flashing EC over Servo with flash_ec\n")
			// futility refuses to flash EC over servod as a separate image and only
			// accepts single image: http://shortn/_dtaO92HvqW. So, for servod, we
			// use flash_ec script that to flash the EC separately.
			flashECScript, err := firmwareservice.GetFlashECScript(ctx, connection, ecRoMetadata.ArchiveDir)
			if err != nil {
				return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
			}
			err = s.service.ProvisionWithFlashEC(ctx, ecRoPath, flashECScript)
			if err != nil {
				return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
			}
		} else {
			// For SSH, we can simply run `futility ... --ec-image=$EC_IMAGE ...`
			futilityImageArgs = append(futilityImageArgs, []string{fmt.Sprint("--ec_image=", ecRoPath)}...)
		}
	}

	mainRoMetadata, ok := s.service.GetImageMetadata(s.service.GetMainRoPath())
	if ok {
		log.Printf("[FW Provisioning: Update RO] extracting AP image to flash\n")
		mainRoPath, err = firmwareservice.PickAndExtractMainImage(ctx, s.service.DUTServer, mainRoMetadata, s.service.GetMainRoPath(), s.service)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		futilityImageArgs = append(futilityImageArgs, []string{fmt.Sprint("--image=", mainRoPath)}...)
		hasCSME, err := s.service.CheckForCSMESections(ctx, mainRoPath)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		if hasCSME {
			futilityImageArgs = append(futilityImageArgs, "--quirks", "unlock_csme")
		}
	}

	// If EC RW is present, but AP RW is not, and we were going to flash the AP anyway, then run swap_ec_rw on mainRoPath
	// Unless EC-RW is the same image as AP-RO
	ecRwMetadata, ok := s.service.GetImageMetadata(s.service.GetEcRwPath())
	if mainRoPath != "" && s.service.GetMainRwPath() == "" && ok && board != "drallion" && board != "sarien" &&
		s.service.GetMainRoPath() != s.service.GetEcRwPath() {
		log.Printf("[FW Provisioning: Update RO] extracting EC-RW image to flash\n")
		ecRwPath, err = firmwareservice.PickAndExtractECImage(ctx, s.service.DUTServer, ecRwMetadata, s.service.GetEcRwPath(), s.service)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		newMainPath, err := firmwareservice.SwapECRWImage(ctx, s.service.DUTServer, mainRoPath, ecRwPath)
		if err != nil {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
		}
		if newMainPath != mainRoPath {
			return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, errors.Errorf("newMainPath unexpected, got %q, want %q", newMainPath, mainRoPath)
		}
	}

	// TODO: Call fws.ExtractFirmwareVersions & fws.ActiveFirmwareVersions, and if they match, then skip flashing.
	// TODO: Remove fws.ExtractFirmwareVersions from fws.FlashWithFutility
	log.Printf("[FW Provisioning: Update RO] flashing RO/RW firmware with futility\n")
	err = s.service.FlashWithFutility(ctx, false /* WP */, futilityImageArgs, mainRoPath, ecRoPath)
	if err != nil {
		return nil, api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED, firmwareservice.UpdateFirmwareFailedErr(err)
	}

	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s FirmwareUpdateRoState) Next() ServiceState {
	if s.service.UpdateRw() {
		return FirmwareUpdateRwState(s)
	} else {
		return FirmwarePostInstallState(s)
	}
}

const UpdateRoStateName = "Firmware Update RO"

func (s FirmwareUpdateRoState) Name() string {
	return UpdateRoStateName
}
