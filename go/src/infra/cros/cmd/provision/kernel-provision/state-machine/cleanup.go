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

// KernelProvisionCleanUpState represents the clean up tasks needed to be done
// upon completion of a kernel provision.
type KernelProvisionCleanUpState struct {
	service *service.KernelProvisionService
}

func NewKernelProvisionCleanUpState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &KernelProvisionCleanUpState{service: service}
}

func (s *KernelProvisionCleanUpState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	if err := s.resetGBBFlags(ctx, log); err != nil {
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

// resetGBBFlags sets the DUT's GBB flags back to their original value.
func (s *KernelProvisionCleanUpState) resetGBBFlags(ctx context.Context, log *log.Logger) error {
	if s.service.OriginalGBBFlags == "" {
		return fmt.Errorf("original gbb flags not found during kernel-provision cleanup")
	}
	if err := gbb.SetGBBFlags(ctx, log, s.service.DUT.GetChromeos().GetSsh().GetAddress(), s.service.OriginalGBBFlags); err != nil {
		return fmt.Errorf("setting gbb flags back to original value %s: %w", s.service.OriginalGBBFlags, err)
	}
	if newFlags, err := gbb.GetGBBFlags(ctx, log, s.service.DUT.GetChromeos().GetSsh().GetAddress()); err != nil {
		return fmt.Errorf("checking gbb flags after resetting: %w", err)
	} else if newFlags != s.service.OriginalGBBFlags {
		return fmt.Errorf("got gbb flags %s after attempting to reset to original value %s", newFlags, s.service.OriginalGBBFlags)
	}
	return nil
}

func (s *KernelProvisionCleanUpState) Next() commonutils.ServiceState {
	return nil
}

func (s *KernelProvisionCleanUpState) Name() string {
	return "Kernel-Provision Clean Up"
}

func (s *KernelProvisionCleanUpState) Retry() bool {
	return false
}
