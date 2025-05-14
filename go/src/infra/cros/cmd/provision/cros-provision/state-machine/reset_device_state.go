// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package state_machine - Pre check for CrOSInstall State Machine. Responsible for checking image status and possibly skipping provision.
package state_machine

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/service"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/state-machine/commands"
)

// CrOSResetDeviceState can be thought of as the constructor state, which initializes
// variables in CrOSService
type CrOSResetDeviceState struct {
	service *service.CrOSService
}

// NewCrOSResetDeviceState provides an interface to CrOSResetDeviceState.
func NewCrOSResetDeviceState(service *service.CrOSService) commonutils.ServiceState {
	return CrOSResetDeviceState{
		service: service,
	}
}

// Execute executes the steps needed to support CrOSResetDeviceState.
func (s CrOSResetDeviceState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute CrOSResetDeviceState")
	comms := []commonutils.CommandInterface{
		commands.NewStopSystemDaemonsCommand(ctx, s.service),
		commands.NewProvisionStatefulCommand(ctx, s.service),
		commands.NewClearTPMCommand(ctx, s.service),
		commands.NewRebootCommand(ctx, s.service),
		commands.NewEnableChargeLimitCommand(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			return nil, comm.GetStatus(), fmt.Errorf("%s, %w", comm.GetErrorMessage(), err)
		}
	}

	log.Printf("State: CrOSResetDeviceState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

// Nothing to do for Next.
func (s CrOSResetDeviceState) Next() commonutils.ServiceState {
	return nil
}

// Name of the step.
func (s CrOSResetDeviceState) Name() string {
	return "CrOS Init"
}

func (s CrOSResetDeviceState) Retry() bool {
	return false
}
