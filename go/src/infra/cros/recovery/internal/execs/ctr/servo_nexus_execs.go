// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"
	"os"
	"path/filepath"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/components/cft/servonexus"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/execs/servo"
	"infra/cros/recovery/internal/log"
)

// startServoNexusContainerExec starts servo-nexius container.
func startServoNexusContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start servo-nexus: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("start servo-nexus: dut is not provided").Err()
	}
	networkName := cft.NetworkName(dut)
	if _, err := ctrInfo.GetNetwork(ctx, networkName); err != nil {
		return errors.Annotate(err, "start servo-nexus container").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	containerRepo := argsMap.AsString(ctx, "container_repo", "us-docker.pkg.dev/cros-registry/test-services/servo-nexus")
	containerTag := argsMap.AsString(ctx, "container_tag", "prod") //oct_18_b
	volumes := argsMap.AsStringSlice(ctx, "container_volumes", []string{"/creds:/creds"})
	artifactDir := argsMap.AsString(ctx, "artifact_dir", "/tmp/servod")
	containerImage := containerRepo + ":" + containerTag
	containerName := cft.ServoNexusName(dut)
	req := &api.StartTemplatedContainerRequest{
		Name:           containerName,
		ContainerImage: containerImage,
		Template: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "cros-servod",
					BinaryArgs: []string{
						"server",
						"-server_port",
						"0",
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
		return errors.Annotate(err, "start servo-nexus container").Err()
	}
	log.Infof(ctx, "Container %q started!", req.Name)
	client, err := servonexus.ServiceClient(ctx, ctrInfo, dut)
	if err != nil {
		return errors.Annotate(err, "start servo-nexus container").Err()
	}
	err = cft.ClientToScope(ctx, dut, client, containerName)
	return errors.Annotate(err, "start servo-nexus container").Err()
}

func stopServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("stop servo-nexus container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("stop servo-nexus container: dut is not provided").Err()
	}
	containerName := cft.ServoNexusName(dut)
	if err := ctrInfo.StopContainer(ctx, containerName); err != nil {
		return errors.Annotate(err, "stop servo-nexus container").Err()
	}
	log.Infof(ctx, "Container %q stopped!", containerName)
	return nil
}

func startServodByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("start servod by servo-nexus: client is not found").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	useRecoveryMode := actionArgs.AsBool(ctx, "recovery_mode", false)
	if !useRecoveryMode {
		// The request to use recovery mode can be specified by presence of a specific file.
		logRoot := info.GetLogRoot()
		flagPath := filepath.Join(logRoot, servo.ServodUseRecoveryModeFlag)
		// If the call fail we think that file is not exist.
		// The call cannot fail as part of permission issue as file is created under the same user.
		if _, err := os.Stat(flagPath); err == nil {
			useRecoveryMode = true
		}
	}
	if err := servonexus.StartServod(ctx, client, info.GetDut(), useRecoveryMode); err != nil {
		return errors.Annotate(err, "start servod by servo-nexus").Err()
	}
	log.Debugf(ctx, "Start servod by servo-nexus: success!")
	return nil
}

func stopServodByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("start servod by servo-nexus: client is not found").Err()
	}
	if err := servonexus.StopServod(ctx, client, info.GetDut()); err != nil {
		return errors.Annotate(err, "start servod by servo-nexus").Err()
	}
	log.Debugf(ctx, "Start servod by servo-nexus: success!")
	return nil
}

func callServodByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("call get servod by servo-nexus: client is not found").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	method := actionArgs.AsString(ctx, "method", "")
	command := actionArgs.AsString(ctx, "command", "")
	value := actionArgs.AsString(ctx, "value", "")
	var res *xmlrpc.Value
	if value == "" {
		res, err = servonexus.CallServod(ctx, client, info.GetDut(), method, command)
	} else {
		res, err = servonexus.CallServod(ctx, client, info.GetDut(), method, command, value)
	}
	if err != nil {
		return errors.Annotate(err, "call get servod by servo-nexus").Err()
	}
	log.Debugf(ctx, "Get servod %q: %s", command, res)
	return nil
}

func callServodGetByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("call get servod by servo-nexus: client is not found").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	command := actionArgs.AsString(ctx, "command", "")
	res, err := servonexus.CallServod(ctx, client, info.GetDut(), "get", command)
	if err != nil {
		return errors.Annotate(err, "call get servod by servo-nexus").Err()
	}
	log.Debugf(ctx, "Get servod %q: %s", command, res)
	return nil
}

func callServodSetByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("call set servod by servo-nexus: client is not found").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	command := actionArgs.AsString(ctx, "command", "")
	value := actionArgs.AsString(ctx, "value", "")
	if _, err := servonexus.CallServod(ctx, client, info.GetDut(), "set", command, value); err != nil {
		return errors.Annotate(err, "call set servod by servo-nexus").Err()
	}
	return nil
}

func callServodDocByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("call doc servod by servo-nexus: client is not found").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	command := actionArgs.AsString(ctx, "command", "")
	res, err := servonexus.CallServod(ctx, client, info.GetDut(), "doc", command)
	if err != nil {
		return errors.Annotate(err, "call doc servod by servo-nexus").Err()
	}
	log.Debugf(ctx, "Doc servod %q: %s", command, res.GetString_())
	return nil
}

func callServodHwinitByServoNexusExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.ServoClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("call doc servod by servo-nexus: client is not found").Err()
	}
	if _, err := servonexus.CallServod(ctx, client, info.GetDut(), "hwinit", true); err != nil {
		return errors.Annotate(err, "call doc servod by servo-nexus").Err()
	}
	log.Debugf(ctx, "Doc servod 'hwinit' with out issues!")
	return nil
}

func init() {
	execs.Register("ctr_servo_nexus_start_container", startServoNexusContainerExec)
	execs.Register("ctr_servo_nexus_stop_container", stopServoNexusExec)
	execs.Register("ctr_servo_nexus_start_servod", startServodByServoNexusExec)
	execs.Register("ctr_servo_nexus_stop_servod", stopServodByServoNexusExec)
	execs.Register("ctr_servo_nexus_servod_call", callServodByServoNexusExec)
	execs.Register("ctr_servo_nexus_servod_get", callServodGetByServoNexusExec)
	execs.Register("ctr_servo_nexus_servod_set", callServodSetByServoNexusExec)
	execs.Register("ctr_servo_nexus_servod_doc", callServodDocByServoNexusExec)
	execs.Register("ctr_servo_nexus_servod_hwinit", callServodHwinitByServoNexusExec)
}
