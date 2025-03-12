// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package state_machine - Pre check for CrOSInstall State Machine. Responsible for checking image status and possibly skipping provision.
package state_machine

import (
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/state-machine/commands"
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

// CrOSPreInitState can be thought of as the constructor state, which initializes
// variables in CrOSService
type CrOSPreInitState struct {
	service    *service.CrOSService
}

// NewCrOSPreInitState provides an interface to CrOSPreInitState.
func NewCrOSPreInitState(service *service.CrOSService) common_utils.ServiceState {
	return CrOSPreInitState{
		service: service,
	}
}

// Execute executes the steps needed to support CrOSPreInitState. Xheck if the chromeOS target == current, if so skip install.
func (s CrOSPreInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute CrOSPreInitState")
	comms := []common_utils.CommandInterface{
		commands.NewGetBoardCommand(ctx, s.service),
		commands.NewGetVersionCommand(ctx, s.service),
		commands.NewCheckInstallNeeded(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			return nil, comm.GetStatus(), fmt.Errorf("%s, %s", comm.GetErrorMessage(), err)
		}
	}

	log.Printf("State: CrOSPreInitState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

// Next provides the interface to the CrosInitState if the install flag is set.
func (s CrOSPreInitState) Next() common_utils.ServiceState {
	if s.service.UpdateCros {
		return CrOSInitState(s)
	} else if s.service.QuickResetDevice {
		return CrOSResetDeviceState(s)
	}
	return nil
}

// Name of the step.
func (s CrOSPreInitState) Name() string {
	return "CrOS Init"
}
