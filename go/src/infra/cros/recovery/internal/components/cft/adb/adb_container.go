// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package adb contains methods to work with an ADB-base container.
package adb

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

// ADBResponse base interface describe ADB response.
type ADBResponse interface {
	GetStdout() []byte
	GetStderr() []byte
}

// ExecCommand execs a raw command by ADB.
func ExecCommand(ctx context.Context, adbClient api.ADBServiceClient, timeout time.Duration, command string, args ...string) (response ADBResponse, rErr error) {
	if command == "" {
		return nil, errors.Reason("exec adb command: command is empty").Err()
	}
	fullCmd := command
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	log.Infof(ctx, "Prepare to run adb command %q ...", fullCmd)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := adbClient.ExecCommand(ctx, &api.ADBCommandRequest{
		Command: command,
		Args:    args,
	})
	if res != nil {
		log.Infof(ctx, "STDOUT: %s", response.GetStdout())
		log.Infof(ctx, "STDERR: %s", response.GetStderr())
	}
	return res, errors.Annotate(err, "exec adb command %q", fullCmd).Err()
}

// ShellCommand execs a shell command by ADB.
func ShellCommand(ctx context.Context, adbClient api.ADBServiceClient, timeout time.Duration, args ...string) (ADBResponse, error) {
	if len(args) == 0 {
		return nil, errors.Reason("shell adb command: no commands for execution").Err()
	}
	res, err := ExecCommand(ctx, adbClient, timeout, "shell", args...)
	return res, errors.Annotate(err, "shell adb command").Err()
}

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("adb service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("adb service client: ctr client is not provided").Err()
	}
	adbContainer, err := ctrInfo.GetContainer(ctx, cft.ADBName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "adb service client").Err()
	}
	conn, err := adbContainer.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "adb service client").Err()
	}
	adbClient := api.NewADBServiceClient(conn)
	if adbClient == nil {
		return nil, errors.Reason("adb service client: fail to create client").Err()
	}
	return adbClient, nil
}
