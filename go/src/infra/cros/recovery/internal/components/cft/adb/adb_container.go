// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package adb contains methods to work with an ADB-base container.
package adb

import (
	"context"
	"os"
	"strconv"
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
	res, err := adbClient.ExecCommand(ctx, &api.ADBCommandRequest{
		Command: command,
		Args:    args,
	})
	if res != nil {
		log.Debugf(ctx, "STDOUT: %s", res.GetStdout())
		log.Debugf(ctx, "STDERR: %s", res.GetStderr())
		log.Debugf(ctx, "EXITCODE: %d", res.GetExitCode())
	}
	if err != nil {
		err = errors.Reason("failed execute command %q, finished with error: %d", fullCmd, err).Err()
	}
	return res, errors.Annotate(err, "exec adb command %q", fullCmd).Err()
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

// Port provides port number for ADB connect.
func Port(ctx context.Context) int {
	val := strings.TrimSpace(os.Getenv("ADB_CONNECTION_PORT"))
	if val != "" {
		if port, err := strconv.Atoi(val); err != nil {
			log.Infof(ctx, "Fail to parse ADB port from environment: %q, will use default port 22", val)
		} else {
			return port
		}
	}
	// Default ADB port for connection.
	return 22
}
