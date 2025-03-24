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

	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// KernelProvisionCleanUpState represents the clean up tasks needed to be done
// upon completion of a kernel provision.
type KernelProvisionCleanUpState struct {
	service *service.KernelProvisionService
}

func NewKernelProvisionCleanUpState(service *service.KernelProvisionService) common_utils.ServiceState {
	return &KernelProvisionCleanUpState{service: service}
}

func (s *KernelProvisionCleanUpState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *KernelProvisionCleanUpState) Next() common_utils.ServiceState {
	return nil
}

func (s *KernelProvisionCleanUpState) Name() string {
	return "Kernel-Provision Clean Up"
}
