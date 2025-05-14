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

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// KernelProvisionProvisionState represents the tasks needed to perform a kernel
// provision.
type KernelProvisionProvisionState struct {
	service *service.KernelProvisionService
}

func NewKernelProvisionProvisionState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &KernelProvisionProvisionState{service: service}
}

func (s *KernelProvisionProvisionState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	for _, pi := range s.service.KPRequest.GetPartitionImages() {
		// s.LocalArtifactPaths should have been populated during DownloadArtifactsState.
		_, ok := s.service.LocalArtifactPaths[pi.GetImagePath().GetPath()]
		if !ok {
			return nil, api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED, fmt.Errorf("no local path found for partition image %+v; only have %+v", pi, s.service.LocalArtifactPaths)
		}
		// TODO: b/396483984 - Flash this prebuilt onto DUT via fastbootd
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *KernelProvisionProvisionState) Next() commonutils.ServiceState {
	return NewExitFastbootdState(s.service)
}

func (s *KernelProvisionProvisionState) Name() string {
	return "Kernel-Provision Provision"
}

func (s KernelProvisionProvisionState) Retry() bool {
	return false
}
