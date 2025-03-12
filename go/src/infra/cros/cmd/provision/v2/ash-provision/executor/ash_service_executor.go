// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executor

import (
	"errors"

	"go.chromium.org/chromiumos/test/provision/v2/ash-provision/service"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"

	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	state_machine "go.chromium.org/chromiumos/test/provision/v2/ash-provision/state-machine"
)

type AShProvisionExecutor struct {
}

func (c *AShProvisionExecutor) GetFirstState(dut *lab_api.Dut, dutClient api.DutServiceClient, _ string, req *api.InstallRequest) (common_utils.ServiceState, error) {
	cs, err := service.NewAShService(dut, dutClient, req)
	if err != nil {
		return nil, err
	}
	return state_machine.NewAShInitState(cs), nil
}

// Validate ensures the ProvisionStartupRequest meets specified requirements.
func (c *AShProvisionExecutor) Validate(req *api.ProvisionStartupRequest) error {
	if req.GetDut() == nil || req.GetDut().GetId().GetValue() == "" {
		return errors.New("dut id is not specified in input file")
	}
	if req.GetDutServer() == nil || req.DutServer.GetAddress() == "" || req.DutServer.GetPort() <= 0 {
		return errors.New("dut server address is no specified or incorrect in input file")
	}
	return nil
}
