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

	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/adb"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// ADBResponse base interface describe ADB response.
type ADBResponse interface {
	GetStdout() []byte
	GetStderr() []byte
	GetExitCode() int32
}

// ExecCommand execs command by base-adb service and provide error if adb command failed.
func ExecCommand(ctx context.Context, adbClient api.ADBServiceClient, timeout time.Duration, command string, args ...string) (ADBResponse, error) {
	res, err := RunCommand(ctx, adbClient, timeout, command, args...)
	if err != nil {
		return res, errors.Annotate(err, "exec adb command").Err()
	}
	if res.GetExitCode() != 0 {
		return res, errors.Reason("exec adb command: failed with exitcode: %d", res.GetExitCode()).Err()
	}
	return res, nil
}

// RunCommand runs raw command by base-adb service and return result.
//
// Error is provided only if service fail to execute command or timeout.
// Command execution always represented as exec-code in response.
func RunCommand(ctx context.Context, adbClient api.ADBServiceClient, timeout time.Duration, command string, args ...string) (ADBResponse, error) {
	if command == "" {
		return nil, errors.Reason("exec adb command: command is empty").Err()
	}
	fullCmd := command
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	log.Debugf(ctx, "Prepare to run adb command: %q", fullCmd)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var err error
	var response ADBResponse
	if adbClient != nil {
		response, err = adbClient.ExecCommand(ctx, &api.ADBCommandRequest{
			Command: command,
			Args:    args,
		})
	} else {
		newArgs := []string{command}
		newArgs = append(newArgs, args...)
		response, err = adb.Run(ctx, newArgs...)
	}
	if response != nil {
		log.Debugf(ctx, "STDOUT: %s", response.GetStdout())
		log.Debugf(ctx, "STDERR: %s", response.GetStderr())
		log.Debugf(ctx, "EXITCODE: %d", response.GetExitCode())
	}
	if err != nil {
		err = errors.Reason("failed execute command %q, finished with error: %d", fullCmd, err).Err()
	}
	return response, errors.Annotate(err, "exec adb command %q", fullCmd).Err()
}

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("adb service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("adb service client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.ADBName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "adb service client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "adb service client").Err()
	}
	client := api.NewADBServiceClient(conn)
	if client == nil {
		return nil, errors.Reason("adb service client: fail to create client").Err()
	}
	return client, nil
}
