// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// First step of the State Machine. Responsible for initialization.
package state_machine

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/foil-provision/service"
	"go.chromium.org/infra/cros/cmd/provision/foil-provision/state-machine/commands"
)

// FoilPostState can be thought of as the constructor state, which initializes
// variables in FoilService
type FoilPostState struct {
	service *service.FoilService
}

func NewFoilPostState(service *service.FoilService) commonutils.ServiceState {
	return FoilPostState{
		service: service,
	}
}

func (s FoilPostState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute FoilPostState")
	comms := []commonutils.CommandInterface{
		commands.NewGetVersionSetup(ctx, s.service),
		commands.NewVerifyInstallCommand(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			return commonutils.WrapStringInAny(comm.GetErrorMessage()), comm.GetStatus(), fmt.Errorf("%s, %s", comm.GetErrorMessage(), err)
		}
	}
	log.Printf("State: FoilPostState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s FoilPostState) Next() commonutils.ServiceState {
	return nil
}

func (s FoilPostState) Name() string {
	return "Foil Post Instsll"
}
