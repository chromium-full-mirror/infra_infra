// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	adbTool "infra/cros/recovery/internal/adb"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/components/cft/adb"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
)

func startADBContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start adb container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("start adb container: dut is not provided").Err()
	}
	networkName := cft.NetworkName(dut)
	if _, err := ctrInfo.GetNetwork(ctx, networkName); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	containerTag := argsMap.AsString(ctx, "container_tag", "prod_adb-base")
	volumes := argsMap.AsStringSlice(ctx, "container_volumes", []string{"/creds:/creds"})
	artifactDir := argsMap.AsString(ctx, "artifact_dir", "/tmp/base-adb")
	containerImage, err := ctrInfo.GenerateContainerImagePath(ctx, cft.ADBBase, containerTag)
	if err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	containerName := cft.ADBName(dut)
	req := &api.StartTemplatedContainerRequest{
		Name:           containerName,
		ContainerImage: containerImage,
		Template: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "base-adb",
					BinaryArgs: []string{
						"server",
						"-port",
						"80",
						"-device",
						dut.Name,
					},
					AdditionalVolumes: volumes,
					DockerArtifactDir: artifactDir,
				},
			},
		},
		Network: networkName,
		// ArtifactDir: c.artifactsDir, defined below in the call.
	}
	if _, err := ctrInfo.CreateContainer(ctx, req); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	log.Infof(ctx, "Container %q started!", req.Name)
	client, err := adb.ServiceClient(ctx, ctrInfo, dut)
	if err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	if err := cft.ClientToScope(ctx, dut, client, containerName); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	if err := cft.AddressToScope(ctx, ctrInfo, containerName); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	return nil
}

func stopADBContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("stop adb container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("stop adb container: dut is not provided").Err()
	}
	containerName := cft.ADBName(dut)
	if err := ctrInfo.StopContainer(ctx, containerName); err != nil {
		return errors.Annotate(err, "stop adb container").Err()
	}
	log.Infof(ctx, "Container %q stopped!", containerName)
	return nil
}

// adbCommandExec execs custom command with arguments.
func adbCommandExec(ctx context.Context, info *execs.ExecInfo) error {
	var client api.ADBServiceClient
	if !adbTool.UseLocal(ctx) {
		var err error
		client, err = cft.ADBClientFromScope(ctx, info.GetDut())
		if err != nil {
			return errors.Annotate(err, "adb command").Err()
		}
	}
	// Minus 5 seconds as we expect 5 seconds to get container info.
	timeout := info.GetExecTimeout() - (5 * time.Second)
	argsMap := info.GetActionArgs(ctx)
	command := argsMap.AsString(ctx, "command", "")
	commandArgs := argsMap.AsStringSlice(ctx, "args", []string{})
	_, err := adb.ExecCommand(ctx, client, timeout, command, commandArgs...)
	return errors.Annotate(err, "adb command").Err()
}

func adbConnectExec(ctx context.Context, info *execs.ExecInfo) error {
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("adb connect: dut is not provided").Err()
	}
	var client api.ADBServiceClient
	if !adbTool.UseLocal(ctx) {
		var err error
		client, err = cft.ADBClientFromScope(ctx, info.GetDut())
		if err != nil {
			return errors.Annotate(err, "adb connect").Err()
		}
	}
	adbPort := adbTool.Port(ctx)
	argsMap := info.GetActionArgs(ctx)
	retryCount := argsMap.AsInt(ctx, "retry_count", 1)
	retryinterval := argsMap.AsDuration(ctx, "retry_interval", 1, time.Second)
	// Set 10 seconds so in total is 60 seconds, but mostly will run faster.
	timeout := argsMap.AsDuration(ctx, "timeout", 2, time.Second)
	deviceName := fmt.Sprintf("%s:%d", dut.Name, adbPort)
	if argsMap.AsBool(ctx, "skip_when_connected", true) {
		log.Infof(ctx, "Check if %q is already connected!", dut.Name)
		if res, err := adb.ExecCommand(ctx, client, timeout, "devices"); err != nil {
			log.Debugf(ctx, "Device is not connected yet: %s", err)
		} else if out := string(res.GetStdout()); out != "" && strings.Contains(out, deviceName) {
			log.Debugf(ctx, "Device is listed, so not need to connect")
			return nil
		} else {
			log.Debugf(ctx, "Device is not connected!")
		}
	}
	if !adbTool.UseLocal(ctx) {
		if _, err := adb.ExecCommand(ctx, client, timeout, "kill-server"); err != nil {
			log.Debugf(ctx, "adb devices error: %s", err)
		}
		if _, err := adb.ExecCommand(ctx, client, timeout, "start-server"); err != nil {
			log.Debugf(ctx, "adb devices error: %s", err)
		}
	}
	connect := func() error {
		log.Infof(ctx, "Try to connect to %q by adb", dut.Name)
		if _, err := adb.ExecCommand(ctx, client, timeout, "connect", deviceName); err != nil {
			return errors.Annotate(err, "fail to connect").Err()
		}
		if _, err := adb.ExecCommand(ctx, client, timeout, "root"); err != nil {
			return errors.Annotate(err, "fail to root service, event when expected").Err()
		}
		if res, err := adb.ExecCommand(ctx, client, timeout, "devices"); err != nil {
			return errors.Annotate(err, "fail to read adb devices, after connection").Err()
		} else if out := string(res.GetStdout()); out != "" {
			if !strings.Contains(out, deviceName) {
				return errors.Reason("fail to find connected device %q in list of devices", deviceName).Err()
			}
		} else {
			return errors.Reason("fail to read adb devices, after connection").Err()
		}
		return nil
	}
	if retryErr := retry.LimitCount(ctx, retryCount, retryinterval, connect, "adb connect"); retryErr != nil {
		return errors.Annotate(retryErr, "adb connect").Err()
	}
	return nil
}

func readAndroidVersionExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetDut().Name)
	argsMap := info.GetActionArgs(ctx)
	timeout := argsMap.AsDuration(ctx, "timeout", 10, time.Second)
	if out, err := run(ctx, timeout, "getprop", "ro.build.version.release"); err != nil {
		return errors.Annotate(err, "read android version").Err()
	} else {
		log.Infof(ctx, "ro.build.version.release: %s", out)
	}
	if out, err := run(ctx, timeout, "getprop", "ro.build.version.sdk"); err != nil {
		return errors.Annotate(err, "read android version").Err()
	} else {
		log.Infof(ctx, "ro.build.version.release: %s", out)
	}
	return nil
}

// makeAwakeAlwaysExec sets flag to keep android awake always.
func makeAwakeAlwaysExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	timeout := argsMap.AsDuration(ctx, "timeout", 10, time.Second)
	run := info.NewRunner(info.GetDut().Name)
	_, err := run(ctx, timeout, "settings", "put", "global", "stay_on_while_plugged_in", "3")
	return errors.Annotate(err, "make awake always").Err()
}

func init() {
	execs.Register("ctr_start_adb_container", startADBContainerExec)
	execs.Register("ctr_stop_adb_container", stopADBContainerExec)
	execs.Register("ctr_adb_command", adbCommandExec)
	execs.Register("ctr_adb_connect", adbConnectExec)
	execs.Register("ctr_read_android_version", readAndroidVersionExec)
	execs.Register("ctr_make_awake_always", makeAwakeAlwaysExec)
}
