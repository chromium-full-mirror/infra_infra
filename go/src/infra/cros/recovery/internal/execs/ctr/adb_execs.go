// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	cftadb "go.chromium.org/infra/cros/recovery/internal/components/cft/adb"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/adb"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/android"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
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
	client, err := cftadb.ServiceClient(ctx, ctrInfo, dut)
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
	// Minus 5 seconds as we expect 5 seconds to get container info.
	timeout := info.GetExecTimeout() - (5 * time.Second)
	argsMap := info.GetActionArgs(ctx)
	command := argsMap.AsString(ctx, "command", "")
	commandArgs := argsMap.AsStringSlice(ctx, "args", []string{})
	_, err := adb.Exec(ctx, info.GetDut(), timeout, command, commandArgs...)
	return errors.WrapIf(err, "adb command")
}

func adbConnectExec(ctx context.Context, info *execs.ExecInfo) error {
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("adb connect: dut is not provided").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	retryCount := argsMap.AsInt(ctx, "retry_count", 1)
	retryinterval := argsMap.AsDuration(ctx, "retry_interval", 1, time.Second)
	timeout := argsMap.AsDuration(ctx, "timeout", 2, time.Second)
	skipWhenConnected := argsMap.AsBool(ctx, "skip_when_connected", true)
	reconnect := !skipWhenConnected
	if err := android.ADBConnect(ctx, retryCount, retryinterval, reconnect, timeout, dut); err != nil {
		return errors.Annotate(err, "adb connect").Err()
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
	// go/android-dev/reference/android/provider/Settings.Global#STAY_ON_WHILE_PLUGGED_IN
	// The value 7 indicates that the screen will stay on at all times when plugged in.
	_, err := run(ctx, timeout, "settings", "put", "global", "stay_on_while_plugged_in", "7")
	return errors.WrapIf(err, "make awake always")
}

func init() {
	execs.Register("ctr_start_adb_container", startADBContainerExec)
	execs.Register("ctr_stop_adb_container", stopADBContainerExec)
	execs.Register("ctr_adb_command", adbCommandExec)
	execs.Register("ctr_adb_connect", adbConnectExec)
	execs.Register("ctr_read_android_version", readAndroidVersionExec)
	execs.Register("ctr_make_awake_always", makeAwakeAlwaysExec)
}
