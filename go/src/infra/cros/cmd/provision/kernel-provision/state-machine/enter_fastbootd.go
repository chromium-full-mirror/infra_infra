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

// EnterFastbootdState represents the tasks needed to reboot the DUT into `fastbootd` mode.
type EnterFastbootdState struct {
	service *service.KernelProvisionService
}

func NewEnterFastbootdState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &EnterFastbootdState{service: service}
}

func (s *EnterFastbootdState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	// TODO: b/396483984 - Enter fastbootd state.
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *EnterFastbootdState) Next() commonutils.ServiceState {
	return NewKernelProvisionProvisionState(s.service)
}

func (s *EnterFastbootdState) Name() string {
	return "Kernel-Provision Enter Fastbootd"
}

func (s *EnterFastbootdState) Retry() bool {
	return false
}
