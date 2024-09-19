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

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/components/cft/adb"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
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
	containerName := cft.ADBName(dut)
	req := &api.StartTemplatedContainerRequest{
		Name:           containerName,
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/adb-base:otabekCLv2",
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
					AdditionalVolumes: []string{
						"/creds:/creds",
					},
					DockerArtifactDir: "/tmp/adb",
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
	adbClient, err := adb.ServiceClient(ctx, ctrInfo, dut)
	if err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	err = adb.ToScope(ctx, dut, adbClient)
	return errors.Annotate(err, "start adb container").Err()
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
	client, err := adb.FromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "adb command").Err()
	}
	// Minus 5 seconds as we expect 5 seconds to get container info.
	timeout := info.GetExecTimeout() - (5 * time.Second)
	argsMap := info.GetActionArgs(ctx)
	command := argsMap.AsString(ctx, "command", "")
	commandArgs := argsMap.AsStringSlice(ctx, "args", []string{})
	_, err = adb.ExecCommand(ctx, client, timeout, command, commandArgs...)
	return errors.Annotate(err, "adb command").Err()
}

func adbConnectExec(ctx context.Context, info *execs.ExecInfo) error {
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("adb connect: dut is not provided").Err()
	}
	client, err := adb.FromScope(ctx, dut)
	if err != nil {
		return errors.Annotate(err, "adb connect").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	// Set 10 seconds so in total is 60 seconds, but mostly will run faster.
	timeout := argsMap.AsDuration(ctx, "timeout", 10, time.Second)
	if _, err := adb.ExecCommand(ctx, client, timeout, "devices"); err != nil {
		log.Debugf(ctx, "adb devices error: %s", err)
	}
	if _, err := adb.ExecCommand(ctx, client, timeout, "kill-server"); err != nil {
		log.Debugf(ctx, "adb devices error: %s", err)
	}
	if _, err := adb.ExecCommand(ctx, client, timeout, "start-server"); err != nil {
		log.Debugf(ctx, "adb devices error: %s", err)
	}
	if _, err := adb.ExecCommand(ctx, client, timeout, "connect", dut.Name); err != nil {
		return errors.Annotate(err, "adb connect").Err()
	}
	if _, err := adb.ExecCommand(ctx, client, timeout, "root"); err != nil {
		return errors.Annotate(err, "adb connect").Err()
	}
	if _, err := adb.ExecCommand(ctx, client, timeout, "devices"); err != nil {
		log.Debugf(ctx, "adb devices error: %s", err)
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
