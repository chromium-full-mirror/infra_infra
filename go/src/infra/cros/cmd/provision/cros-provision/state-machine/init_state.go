// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// First step of the CrOSInstall State Machine. Responsible for initialization.
package state_machine

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/service"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/state-machine/commands"
)

// CrosInitState can be thought of as the constructor state, which initializes
// variables in CrOSService
type CrOSInitState struct {
	service *service.CrOSService
}

func NewCrOSInitState(service *service.CrOSService) common_utils.ServiceState {
	return CrOSInitState{
		service: service,
	}
}

func (s CrOSInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute CrOSInitState")
	comms := []common_utils.CommandInterface{
		commands.NewCreateProvisionMarkerCommand(ctx, s.service),
		commands.NewGetRootInfoCommand(ctx, s.service),
		commands.NewCheckKvmEnabled(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			return nil, comm.GetStatus(), fmt.Errorf("%s, %w", comm.GetErrorMessage(), err)
		}
	}
	log.Printf("State: CrOSInitState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s CrOSInitState) Next() common_utils.ServiceState {
	return CrOSInstallState(s)
}

func (s CrOSInitState) Name() string {
	return "CrOS Init"
}
