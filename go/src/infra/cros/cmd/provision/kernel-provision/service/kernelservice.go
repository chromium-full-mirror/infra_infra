// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service contains the service definition of the kernel-provision
// container.
package service

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// KernelProvisionService represents the overall kernel provisioning service.
// This struct should contain all necessary values that needed to be accessed by
// any of the state machine's states.
type KernelProvisionService struct {
	KPRequest *api.KernelPrebuilts
	DUT       *labapi.Dut
	// LocalArtifactPaths contains the local paths to the downloaded kernel prebuilt artifacts that will be flashed.
	// The keys are the full Android Build paths (i.e., partition_image.image_path.Path),
	// and the values are absolute local paths to the downloaded files..
	LocalArtifactPaths map[string]string
}

func NewKernelProvisionService(kp *api.KernelPrebuilts, dut *labapi.Dut) *KernelProvisionService {
	return &KernelProvisionService{
		KPRequest:          kp,
		DUT:                dut,
		LocalArtifactPaths: map[string]string{},
	}
}
