// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package adb contains methods to work with an ADB-base container.
package adb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

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

// Exec execs command by adb service and provide error if adb command failed.
// Hide complexity of which client need to be used.
func Exec(ctx context.Context, dut *tlw.Dut, timeout time.Duration, command string, args ...string) (ADBResponse, error) {
	client, err := adbClient(ctx, dut)
	if err != nil {
		return adb.EmptyResult(), errors.Annotate(err, "adb exec").Err()
	}
	res, err := run(ctx, client, timeout, command, args...)
	if err != nil {
		return res, errors.Annotate(err, "adb exec command").Err()
	}
	if res.GetExitCode() != 0 {
		return res, errors.Reason("exec adb command: failed with exitcode: %d", res.GetExitCode()).Err()
	}
	return res, nil
}

// Root runs root command for a DUT.
func Root(ctx context.Context, dut *tlw.Dut, timeout time.Duration) (ADBResponse, error) {
	client, err := adbClient(ctx, dut)
	if err != nil {
		return adb.EmptyResult(), errors.Annotate(err, "adb root").Err()
	}
	command := "-s"
	args := []string{deviceName(ctx, dut), "root"}
	res, err := run(ctx, client, timeout, command, args...)
	if err != nil {
		return res, errors.Annotate(err, "adb root").Err()
	}
	if res.GetExitCode() != 0 {
		return res, errors.Reason("adb root: failed with exitcode: %d", res.GetExitCode()).Err()
	}
	return res, nil
}

// Connect runs connect command for a DUT.
func Connect(ctx context.Context, dut *tlw.Dut, timeout time.Duration) (ADBResponse, error) {
	client, err := adbClient(ctx, dut)
	if err != nil {
		return adb.EmptyResult(), errors.Annotate(err, "adb connect").Err()
	}
	command := "connect"
	args := []string{deviceName(ctx, dut)}
	res, err := run(ctx, client, timeout, command, args...)
	if err != nil {
		return res, errors.Annotate(err, "adb connect").Err()
	}
	if res.GetExitCode() != 0 {
		return res, errors.Reason("adb connect: failed with exitcode: %d", res.GetExitCode()).Err()
	}
	return res, nil
}

// Disconnect runs disconnect command for a DUT.
func Disconnect(ctx context.Context, dut *tlw.Dut, timeout time.Duration) (ADBResponse, error) {
	client, err := adbClient(ctx, dut)
	if err != nil {
		return adb.EmptyResult(), errors.Annotate(err, "adb disconnect").Err()
	}
	command := "disconnect"
	args := []string{deviceName(ctx, dut)}
	res, err := run(ctx, client, timeout, command, args...)
	if err != nil {
		return res, errors.Annotate(err, "adb disconnect").Err()
	}
	if res.GetExitCode() != 0 {
		return res, errors.Reason("adb disconnect: failed with exitcode: %d", res.GetExitCode()).Err()
	}
	return res, nil
}

func IsConnected(ctx context.Context, dut *tlw.Dut, timeout time.Duration) bool {
	res, err := Devices(ctx, dut, timeout)
	if err != nil {
		log.Debugf(ctx, "Fail to read adb devices, assume device isn't listed yet: %s", err)
		return false
	}
	deviceName := deviceName(ctx, dut)
	if out := string(res.GetStdout()); out != "" && strings.Contains(out, deviceName) {
		log.Debugf(ctx, "Device is listed.")
		return true
	}
	log.Debugf(ctx, "Device isn't listed yet: %s", err)
	return false
}

// Devices runs devices command.
func Devices(ctx context.Context, dut *tlw.Dut, timeout time.Duration) (ADBResponse, error) {
	client, err := adbClient(ctx, dut)
	if err != nil {
		return adb.EmptyResult(), errors.Annotate(err, "adb devices").Err()
	}
	command := "devices"
	res, err := run(ctx, client, timeout, command)
	if err != nil {
		return res, errors.Annotate(err, "adb devices").Err()
	}
	if res.GetExitCode() != 0 {
		return res, errors.Reason("adb devices: failed with exitcode: %d", res.GetExitCode()).Err()
	}
	return res, nil
}

// Shell runs shell command by base-adb service and provide error if adb command failed.
func Shell(ctx context.Context, dut *tlw.Dut, timeout time.Duration, command string, args ...string) (ADBResponse, error) {
	client, err := adbClient(ctx, dut)
	if err != nil {
		return adb.EmptyResult(), errors.Annotate(err, "adb shell").Err()
	}
	// Example adb -s device1:5555 shell xyz
	// Where `-s` is the commend and rest will be args.
	newCommand := "-s"
	newArgs := []string{deviceName(ctx, dut), "shell", command}
	newArgs = append(newArgs, args...)
	res, err := run(ctx, client, timeout, newCommand, newArgs...)
	if err != nil {
		return res, errors.Annotate(err, "adb shell").Err()
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
func run(ctx context.Context, adbClient api.ADBServiceClient, timeout time.Duration, command string, args ...string) (ADBResponse, error) {
	if command == "" {
		return nil, errors.Reason("exec adb command: command is empty").Err()
	}
	fullCmd := "adb " + command
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
		log.Debugf(ctx, "EXITCODE: %v", response.GetExitCode())
	}
	if err != nil {
		err = errors.Reason("failed execute command %q, finished with error: %d", fullCmd, err).Err()
	}
	return response, errors.Annotate(err, "exec adb command %q", fullCmd).Err()
}

func adbClient(ctx context.Context, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if !adb.UseLocal(ctx) {
		return cft.ADBClientFromScope(ctx, dut)
	}
	// For local test we do not provide a client.
	return nil, nil
}

func deviceName(ctx context.Context, dut *tlw.Dut) string {
	return dut.Name + fmt.Sprintf(":%d", adb.Port(ctx))
}
