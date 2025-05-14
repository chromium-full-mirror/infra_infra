// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package statemachine contains the individual states representing the kernel
// provision state machine.
package statemachine

import (
	"context"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// ExitFastbootdState represents the tasks needed to reboot the DUT from fastbootd back into normal mode.
type ExitFastbootdState struct {
	service *service.KernelProvisionService
}

func NewExitFastbootdState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &ExitFastbootdState{service: service}
}

func (s *ExitFastbootdState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	// TODO: b/396483984 - Exit fastbootd state.
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *ExitFastbootdState) Next() commonutils.ServiceState {
	return NewKernelProvisionCleanUpState(s.service)
}

func (s *ExitFastbootdState) Name() string {
	return "Kernel-Provision Exit Fastbootd"
}

func (s *ExitFastbootdState) Retry() bool {
	return false
}
