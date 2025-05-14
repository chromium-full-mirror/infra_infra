// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Fourth step in the CrOSInstall State Machine. Currently a noop, to be implemented
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

type CrOSVerifyState struct {
	service *service.CrOSService
}

func (s CrOSVerifyState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute CrOSVerfiyState")
	comms := []commonutils.CommandInterface{
		commands.NewGetVersionCommand(ctx, s.service),
		commands.NewCheckVersionMatches(ctx, s.service),
		commands.NewEnableChargeLimitCommand(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			return nil, comm.GetStatus(), fmt.Errorf("%s, %w", comm.GetErrorMessage(), err)
		}
	}

	log.Printf("State: CrOSVerfiyState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s CrOSVerifyState) Next() commonutils.ServiceState {
	return CrOSProvisionDLCState(s)
}

func (s CrOSVerifyState) Name() string {
	return "CrOS Verify"
}

func (s CrOSVerifyState) Retry() bool {
	return false
}
