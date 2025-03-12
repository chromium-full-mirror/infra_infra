// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"errors"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// CheckKvmEnabled is the commands interface struct.
type CheckKvmEnabled struct {
	ctx context.Context
	cs  *service.CrOSService
}

// NewCheckKvmEnabled is the commands interface to CheckKvmEnabled
func NewCheckKvmEnabled(ctx context.Context, cs *service.CrOSService) *CheckKvmEnabled {
	return &CheckKvmEnabled{
		ctx: ctx,
		cs:  cs,
	}
}

// Execute is the executor for the command. Will check KVM is enabled by validating against device path for KVM.
func (c *CheckKvmEnabled) Execute(log *log.Logger) error {
	log.Printf("RUNNING CheckKvmEnabled Execute")

	if strings.Contains(c.cs.MachineMetadata.Board, "labstation") {
		log.Printf("CheckKvmEnabled skipping labstation boards")
	} else {
		exist, err := c.cs.Connection.PathExists(c.ctx, common_utils.KvmDevicePath)
		if err != nil {
			return err
		}
		if !exist {
			return errors.New("KVM device path does not exist")
		}
	}

	log.Printf("RUNNING CheckKvmEnabled Success")

	return nil
}

// Revert interface command. None needed as nothing has happened yet.
func (c *CheckKvmEnabled) Revert() error {
	// Thought this method has side effects to the service it does not to the OS,
	// as such Revert here is unneeded
	return nil
}

// GetErrorMessage provides the failed to check KVM enabled string.
func (c *CheckKvmEnabled) GetErrorMessage() string {
	return "failed check KVM enabled"
}

// GetStatus provides API Error reason.
func (c *CheckKvmEnabled) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
