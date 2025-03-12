// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package state_machine - Pre check for State Machine. Responsible for checking image status and possibly skipping provision.
package state_machine

import (
	"context"
	"fmt"
	"log"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/state-machine/commands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

// FoilPreInitState can be thought of as the constructor state, which initializes
// variables in FoilService
type FoilPreInitState struct {
	service    *service.FoilService
	shouldSkip bool
}

// NewFoilPreInitState provides an interface to FoilPreInitState.
func NewFoilPreInitState(service *service.FoilService) common_utils.ServiceState {
	return FoilPreInitState{
		service: service,
	}
}

// Execute executes the steps needed to support FoilPreInitState. Check if the chromeOS target == current, if so skip install.
func (s FoilPreInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute FoilPreInitState")
	comms := []common_utils.CommandInterface{
		commands.NewAdbRootSetup(ctx, s.service),
		commands.NewGetEnginePidSetup(ctx, s.service),
		commands.NewGetVersionSetup(ctx, s.service),
		commands.NewCheckForSkipCommand(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			return common_utils.WrapStringInAny(comm.GetErrorMessage()), comm.GetStatus(), fmt.Errorf("%s: %w", comm.GetErrorMessage(), err)
		}
	}

	log.Printf("DONE WITH PRE_INIT")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

// Next provides the interface if the install flag is set.
func (s FoilPreInitState) Next() common_utils.ServiceState {
	// NOTE: currently a device can be in the right build; but in a broken state (even though it can be reached through the shell)
	// Thus for now: we must always re-provision.
	// if s.service.SkipUpdate == true {
	// 	return nil
	// }
	return FoilInstallState{
		service: s.service,
	}

}

// Name of the step.
func (s FoilPreInitState) Name() string {
	return "Foil Init"
}
