// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/components/cft/crosdut"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

func startCrosDutContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start cros-dut container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("start cros-dut container: dut is not provided").Err()
	}
	networkName := cft.NetworkName(dut)
	if _, err := ctrInfo.GetNetwork(ctx, networkName); err != nil {
		return errors.Annotate(err, "start cros-dut container").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	containerTag := argsMap.AsString(ctx, "container_tag", "prod_cros-dut")
	volumes := argsMap.AsStringSlice(ctx, "container_volumes", []string{"/creds:/creds"})
	artifactDir := argsMap.AsString(ctx, "artifact_dir", "/tmp/cros-dut")
	containerImage, err := ctrInfo.GenerateContainerImagePath(ctx, cft.CrosDUT, containerTag)
	if err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	containerName := cft.CrosDUTName(dut)
	req := &api.StartTemplatedContainerRequest{
		Name:           containerName,
		ContainerImage: containerImage,
		Template: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "cros-dut",
					BinaryArgs: []string{
						"-dut_address",
						dut.Name,
						"-cache_address",
						// TODO(b/376048814): get correct cache address
						"192.168.100.1:8082",
						"-port",
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
		return errors.Annotate(err, "start cros-dut container").Err()
	}
	log.Infof(ctx, "Container %q started!", req.Name)
	client, err := crosdut.ServiceClient(ctx, ctrInfo, dut)
	if err != nil {
		return errors.Annotate(err, "start cros-dut container").Err()
	}
	if err := cft.ClientToScope(ctx, dut, client, containerName); err != nil {
		return errors.Annotate(err, "start cros-dut container").Err()
	}
	if err := cft.AddressToScope(ctx, ctrInfo, containerName); err != nil {
		return errors.Annotate(err, "start cros-dut container").Err()
	}
	return nil
}

func stopCrosDutContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("stop cros-dut container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("stop cros-dut container: dut is not provided").Err()
	}
	containerName := cft.CrosDUTName(dut)
	if err := ctrInfo.StopContainer(ctx, containerName); err != nil {
		return errors.Annotate(err, "stop cros-dut container").Err()
	}
	log.Infof(ctx, "Container %q stopped!", containerName)
	return nil
}

func init() {
	execs.Register("ctr_start_cros_dut_container", startCrosDutContainerExec)
	execs.Register("ctr_stop_cros_dut_container", stopCrosDutContainerExec)
}
