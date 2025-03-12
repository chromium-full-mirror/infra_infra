// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Executor defines a state initializer for each state. Usable by server to start.
// Under normal conditions this would be part of service, but go being go, it
// would create an impossible cycle
package executor

import (
	"context"
	"errors"
	"fmt"
	"log"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	cross_over "go.chromium.org/chromiumos/test/provision/v2/common-utils/cross-over"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	state_machine "go.chromium.org/chromiumos/test/provision/v2/cros-provision/state-machine"

	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
)

type CrOSProvisionExecutor struct {
	Logger *log.Logger
}

func (c *CrOSProvisionExecutor) GetFirstState(dut *lab_api.Dut, dutClient api.DutServiceClient, servoNexusAddr string, req *api.InstallRequest) (common_utils.ServiceState, error) {
	cs, err := service.NewCrOSService(dut, dutClient, req)
	if err != nil {
		return nil, err
	}
	crosProvisionState := state_machine.NewCrOSPreInitState(cs)
	if !cross_over.IsCROS(context.Background(), c.Logger, dutClient) {
		c.Logger.Println("DUT not on CROS, will perform crossOver(ANDROID->CROS) provisioning.")
		if servoNexusAddr == "" {
			return nil, fmt.Errorf("servoNexusAdd required for crossover provision")
		}
		conn, err := grpc.Dial(servoNexusAddr, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		servoNexusClient := api.NewServodServiceClient(conn)
		params := &cross_over.CrossOverParameters{
			DutClient:          dutClient,
			Dut:                dut,
			TargetImagePath:    req.GetImagePath(),
			PostProvisionState: crosProvisionState,
			ServoNexusClient:   servoNexusClient,
			PartnerMetadata:    req.GetPartnerMetadata(),
		}
		return cross_over.NewCrossOverInitState(params), nil
	}
	c.Logger.Println("CROS Detected, will perform CROS -> CROS provisioning.")
	return crosProvisionState, nil
}

// Validate ensures the ProvisionStartupRequest meets specified requirements.
func (c *CrOSProvisionExecutor) Validate(req *api.ProvisionStartupRequest) error {
	if req.GetDut() == nil || req.GetDut().GetId().GetValue() == "" {
		return errors.New("dut id is not specified in input file")
	}
	if req.GetDutServer() == nil || req.DutServer.GetAddress() == "" || req.DutServer.GetPort() <= 0 {
		return errors.New("dut server address is no specified or incorrect in input file")
	}
	return nil
}
