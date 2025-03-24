// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service contains the service definition of the kernel-provision
// container.
package service

// KernelProvisionService represents the overall kernel provisioning service.
// This struct should contain all necessary values that needed to be accessed by
// any of the state machines states.
type KernelProvisionService struct {
	// Populate with the values needed for the Kernel-provision.
}

func NewKernelProvisionService() (*KernelProvisionService, error) {
	return &KernelProvisionService{}, nil
}
