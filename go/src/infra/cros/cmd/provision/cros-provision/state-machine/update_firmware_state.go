// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Third step of CrOSInstall State Machine. Responsible for update firmware
package state_machine

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/service"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/state-machine/commands"
)

type CrosUpdateFirmwareState struct {
	service *service.CrOSService
}

func (s CrosUpdateFirmwareState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	if !s.service.UpdateFirmware {
		log.Printf("State: Skip CrosUpdateFirmwareState by request")
		return nil, api.InstallResponse_STATUS_SUCCESS, nil
	}
	// Some type of build(e.g. public build) doesn't have built-in firmware updater, so skip the firmware update in this case.
	checkUpdaterComm := commands.NewCheckFirmwareUpdaterCommand(ctx, s.service)
	if err := checkUpdaterComm.Execute(log); err != nil {
		return nil, checkUpdaterComm.GetStatus(), fmt.Errorf("%s, %w", checkUpdaterComm.GetErrorMessage(), err)
	}
	if !checkUpdaterComm.UpdaterExist {
		log.Printf("State: Skip CrosUpdateFirmwareState as firmware updater does not exist on the build")
		return nil, api.InstallResponse_STATUS_SUCCESS, nil
	}

	log.Printf("State: Execute CrosUpdateFirmwareState")

	comms := []commonutils.CommandInterface{
		commands.NewWaitForDutToStabilizeCommand(ctx, s.service),
		commands.NewRunFirmwareUpdaterCommand(ctx, s.service),
	}
	checkFirmwareSlotComm := commands.NewCheckFirmwareSlotCommand(ctx, s.service)
	comms = append(comms, checkFirmwareSlotComm)

	for i, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			for ; i >= 0; i-- {
				log.Printf("CrosUpdateFirmwareState REVERT CALLED")
				if innerErr := comm.Revert(); innerErr != nil {
					return nil, comm.GetStatus(), fmt.Errorf("failure while reverting, %w: %w", err, innerErr)
				}
			}
			return nil, comm.GetStatus(), fmt.Errorf("%s, %w", comm.GetErrorMessage(), err)
		}
	}
	// Reboot if firmware slot changed.
	if checkFirmwareSlotComm.RebootRequired {
		// Post firmware update reboot could take longer time, so give it 300 seconds timeout here.
		rebootComm := commands.NewRebootWithTimeoutCommand(300*time.Second, ctx, s.service)
		if err := rebootComm.Execute(log); err != nil {
			return nil, rebootComm.GetStatus(), fmt.Errorf("%s, %w", rebootComm.GetErrorMessage(), err)
		}
	} else {
		log.Printf("no firmware slot change detected, skip post firmware update reboot.")
	}
	verifyFirmwareComm := commands.NewVerifyFirmwareCommand(ctx, s.service)
	if err := verifyFirmwareComm.Execute(log); err != nil {
		return nil, verifyFirmwareComm.GetStatus(), fmt.Errorf("%s, %w", verifyFirmwareComm.GetErrorMessage(), err)
	}

	log.Printf("State: CrosUpdateFirmwareState Completed")

	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s CrosUpdateFirmwareState) Next() commonutils.ServiceState {
	return CrOSPostInstallState(s)
}

func (s CrosUpdateFirmwareState) Name() string {
	return "CrOS Update Firmware"
}

func (s CrosUpdateFirmwareState) Retry() bool {
	return false
}
