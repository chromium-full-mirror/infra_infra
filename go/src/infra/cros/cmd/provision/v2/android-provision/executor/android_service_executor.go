// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executor

import (
	"errors"

	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/chromiumos/test/provision/v2/android-provision/service"
	state_machine "go.chromium.org/chromiumos/test/provision/v2/android-provision/state-machine"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
)

type AndroidProvisionExecutor struct {
}

func (c *AndroidProvisionExecutor) GetFirstState(dut *lab_api.Dut, dutClient api.DutServiceClient, servoNexusAddr string, req *api.InstallRequest) (common_utils.ServiceState, error) {
	svc, err := service.NewAndroidService(dut, dutClient, req)
	if err != nil {
		return nil, err
	}
	return state_machine.NewPrepareState(svc), nil
}

// Validate ensures the ProvisionStartupRequest meets specified requirements.
func (c *AndroidProvisionExecutor) Validate(req *api.ProvisionStartupRequest) error {
	if req.GetDut() == nil || req.GetDut().GetId().GetValue() == "" {
		return errors.New("dut id is not specified in input file")
	}
	if req.GetDut().GetAndroid() == nil {
		return errors.New("android dut is not specified in input file")
	}
	if req.GetDut().GetAndroid().GetSerialNumber() == "" {
		return errors.New("android dut serial number is missing from input file")
	}
	if req.GetDut().GetAndroid().GetAssociatedHostname() == nil || req.GetDut().GetAndroid().GetAssociatedHostname().GetAddress() == "" {
		return errors.New("associated host of android dut is not specified in input file")
	}
	if req.GetDutServer() == nil || req.GetDutServer().GetAddress() == "" || req.DutServer.GetPort() <= 0 {
		return errors.New("dut server address is not specified or incorrect in input file")
	}
	return nil
}
