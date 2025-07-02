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

// KernelProvisionInitState represents the initialization task needed to ready a
// dut for a kernel provision.
type KernelProvisionInitState struct {
	service *service.KernelProvisionService
}

func NewKernelProvisionInitState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &KernelProvisionInitState{service: service}
}

func (s *KernelProvisionInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	if err := s.storeOriginalGBBFlags(ctx, log); err != nil {
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

// storeOriginalGBBFlags gets the DUT's GBB flags, and stores them in the KernelProvisionService.
func (s *KernelProvisionInitState) storeOriginalGBBFlags(ctx context.Context, log *log.Logger) error {
	gbbFlags, err := gbb.GetGBBFlags(ctx, log, s.service.DUT.GetChromeos().GetSsh().GetAddress())
	if err != nil {
		return fmt.Errorf("getting gbb flags: %w", err)
	}
	log.Printf("GBB flags before kernel provisioning: %s", gbbFlags)
	s.service.OriginalGBBFlags = gbbFlags
	return nil
}

func (s *KernelProvisionInitState) Next() commonutils.ServiceState {
	return NewDownloadArtifactsState(s.service)
}

func (s *KernelProvisionInitState) Name() string {
	return "Kernel-Provision Init"
}

func (s *KernelProvisionInitState) Retry() bool {
	return false
}
