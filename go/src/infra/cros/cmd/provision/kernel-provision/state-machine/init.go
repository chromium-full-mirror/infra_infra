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

// KernelProvisionInitState represents the initialization task needed to ready a
// dut for a kernel provision.
type KernelProvisionInitState struct {
	service *service.KernelProvisionService
}

func NewKernelProvisionInitState(service *service.KernelProvisionService) common_utils.ServiceState {
	return &KernelProvisionInitState{service: service}
}

func (s *KernelProvisionInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *KernelProvisionInitState) Next() common_utils.ServiceState {
	return NewDownloadArtifactsState(s.service)
}

func (s *KernelProvisionInitState) Name() string {
	return "Kernel-Provision Init"
}
