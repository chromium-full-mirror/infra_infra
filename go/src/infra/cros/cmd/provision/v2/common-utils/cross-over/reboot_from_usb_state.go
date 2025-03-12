// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DLC constants and helpers
package cross_over

import (
	"context"
	"log"
	"time"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

const waitForPowerOff = 30 * time.Second

// RebootFromUSBState sets the DUT power off and bring it back up in rec mode.
type RebootFromUSBState struct {
	params *CrossOverParameters
}

func NewRebootFromUSBState(params *CrossOverParameters) common_utils.ServiceState {
	return &RebootFromUSBState{
		params: params,
	}
}

func (s RebootFromUSBState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("Executing " + s.Name())
	if err := callServodRetry(ctx, log, "power_state", "off", s.params); err != nil {
		setPDRole(ctx, log, "src", s.params)
		return common_utils.WrapStringInAny("INFRA: unable to set turn off USB"), api.InstallResponse_STATUS_PROVISIONING_FAILED, err
	}
	log.Printf("\nWaiting for the power state to turn off.")
	// TODO: Instead of fixed wait time, use a poll based status checker. Refer WaitForPowerStates and GetECSystemPowerState in firmware code.
	time.Sleep(waitForPowerOff)
	if err := callServodRetry(ctx, log, "power_state", "rec", s.params); err != nil {
		setPDRole(ctx, log, "src", s.params)
		// A hard failure isn't necessary at this point, as subsequent checks will identify servod call failures and system state problems.
		log.Printf("Letting passthrough from failed rec mode boot: %s", err)
	}

	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s RebootFromUSBState) Next() common_utils.ServiceState {
	return NewFullOSImageState(s.params)
}

func (s RebootFromUSBState) Name() string {
	return "Reboot from USB State"
}
