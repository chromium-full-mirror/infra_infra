// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package statemachine contains the individual states representing the kernel
// provision state machine.

package statemachine

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/gbb"
	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// EnableDevSwitchState represents the tasks needed to enable the dev switch on the DUT.
//
// Normally, AL lab devices are kept in "normal mode", i.e. the dev-switch is OFF.
// However, there is a bug where AL devices in fastbootd with the dev-switch OFF lack network connectivity. (b/400637437)
// Thus, when we boot the DUT into fastbootd mode in order to flash kernel partitions,
// we lose the ability to contact the DUT unless we first toggle the dev switch.
//
// TODO: b/400637437 - Remove this state once the fastbootd bug is resolved.
// (At that time we can also remove the GBB logic in the init/cleanup states.)
type EnableDevSwitchState struct {
	service *service.KernelProvisionService
}

func NewEnableDevSwitchState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &EnableDevSwitchState{service: service}
}

func (s *EnableDevSwitchState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	// 0x00000008 = VB2_GBB_FLAG_FORCE_DEV_SWITCH_ON.
	// 0x00000001 = VB2_GBB_FLAG_DEV_SCREEN_SHORT_DELAY (reduces timeout from 30 sec to 2 sec).
	// Thus, the total mask is +0x00000009.
	if err := gbb.SetGBBFlags(ctx, log, s.service.DUT.GetChromeos().GetSsh().GetAddress(), "+0x00000009"); err != nil {
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, fmt.Errorf("setting gbb mask +0x00000009: %w", err)
	}
	updatedGBBFlags, err := gbb.GetGBBFlags(ctx, log, s.service.DUT.GetChromeos().GetSsh().GetAddress())
	if err != nil {
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, fmt.Errorf("checking gbb flags after editing: %w", err)
	}
	log.Printf("Updated GBB flags: %s", updatedGBBFlags)
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *EnableDevSwitchState) Next() commonutils.ServiceState {
	return NewEnterFastbootdState(s.service)
}

func (s *EnableDevSwitchState) Name() string {
	return "Kernel-Provision Enable Dev Switch"
}

func (s *EnableDevSwitchState) Retry() bool {
	return false
}
