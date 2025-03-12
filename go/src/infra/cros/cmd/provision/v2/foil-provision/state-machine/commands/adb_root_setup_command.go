// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"
	"time"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/chromiumos/test/util/adb"
)

const (
	retryTimeout = 15 * time.Second
)

type AdbRoot struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
}

func NewAdbRootSetup(ctx context.Context, cs *service.FoilService) *AdbRoot {
	return &AdbRoot{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *AdbRoot) Execute(log *log.Logger) error {
	log.Printf("Start AdbRoot Execute, with reboot")

	addr := c.cs.DutIp
	adb.RetrySetupAdb(log, addr, retryTimeout)

	_, err := adb.AdbCmd([]string{"-s", adb.FmtAddr(addr), "root"}, log)
	if err != nil {
		return err
	}
	return adb.RetrySetupAdb(log, addr, retryTimeout)
}
func (c *AdbRoot) Revert() error {
	return nil
}

func (c *AdbRoot) GetErrorMessage() string {
	return "failed to establish adb Root."
}

func (c *AdbRoot) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED
}
