// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service contains the service definition of the kernel-provision
// container.
package service

import (
	"go.chromium.org/chromiumos/config/go/test/api"
)

// KernelProvisionService represents the overall kernel provisioning service.
// This struct should contain all necessary values that needed to be accessed by
// any of the state machine's states.
type KernelProvisionService struct {
	kpRequest *api.KernelPrebuilts
}

func NewKernelProvisionService(kp *api.KernelPrebuilts) *KernelProvisionService {
	return &KernelProvisionService{kpRequest: kp}
}
