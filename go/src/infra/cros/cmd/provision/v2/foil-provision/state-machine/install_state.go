// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"fmt"
	"log"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	cross_over "go.chromium.org/chromiumos/test/provision/v2/common-utils/cross-over"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/state-machine/commands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"

	"google.golang.org/grpc"
)

// FoilInstallState can be thought of as the constructor state, which initializes
// variables in FoilService
type FoilInstallState struct {
	service *service.FoilService
}

func NewFoilInstallState(service *service.FoilService) common_utils.ServiceState {
	return FoilInstallState{
		service: service,
	}
}

func (s FoilInstallState) setupForCrosover(ctx context.Context, log *log.Logger, prevErr string) error {
	if s.service.ServoNexusAddr == "" {
		log.Println("unable to crosover after OTA due to no servo")
		return fmt.Errorf("servoNexusAdd required for crossover provision")
	}
	conn, err := grpc.Dial(s.service.ServoNexusAddr, grpc.WithInsecure())
	if err != nil {
		log.Println("unable to crosover after OTA due to error on dial", err)
		return fmt.Errorf("unable to connect to servoGRpc")

	}
	servoNexusClient := api.NewServodServiceClient(conn)
	s.service.Params = &cross_over.CrossOverParameters{
		Dut:              s.service.Dut,
		TargetImagePath:  s.service.Req.GetImagePath(),
		ServoNexusClient: servoNexusClient,
		PrevError:        prevErr,
		PartnerMetadata:  s.service.Req.GetPartnerMetadata(),
	}
	return nil
}

func (s FoilInstallState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("State: Execute FoilInstallState")
	comms := []common_utils.CommandInterface{

		commands.NewSetSelinuxCommandSetup(ctx, s.service),
		commands.NewInstall(ctx, s.service),
		commands.NewRebootCommand(ctx, s.service),
	}

	for _, comm := range comms {
		err := comm.Execute(log)
		if err != nil {
			err = s.setupForCrosover(ctx, log, comm.GetErrorMessage())
			if err != nil {
				log.Println("Unable to setup for crossover provision after OTA failed. Failing.")
				return common_utils.WrapStringInAny(comm.GetErrorMessage()), comm.GetStatus(), fmt.Errorf("%s, %s", comm.GetErrorMessage(), err)
			}
			s.service.CrossOver = true
			break
		}
	}

	if s.service.CrossOver {
		log.Println("State: FoilInstallState FAILED. Will try Flash.")
		// Intentionally not returning an err; as this will prevent next() from being called;
		return nil, api.InstallResponse_STATUS_SUCCESS, nil

	} else {
		log.Printf("State: FoilInstallState Completed")
		return nil, api.InstallResponse_STATUS_SUCCESS, nil

	}
}

func (s FoilInstallState) Next() common_utils.ServiceState {
	if s.service.CrossOver {
		return cross_over.NewCrossOverInitState(s.service.Params)
	} else {
		return FoilPostState{
			service: s.service,
		}
	}
}

func (s FoilInstallState) Name() string {
	return "Foil Install"
}
