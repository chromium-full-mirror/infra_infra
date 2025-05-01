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
}

func NewKernelProvisionService(kp *api.KernelPrebuilts, dut *labapi.Dut) *KernelProvisionService {
	return &KernelProvisionService{KPRequest: kp, DUT: dut}
}
