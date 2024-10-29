// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/components/cft/foilprovision"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

func startFoilProvisionContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start foil-provision container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("start foil-provision container: dut is not provided").Err()
	}
	networkName := cft.NetworkName(dut)
	if _, err := ctrInfo.GetNetwork(ctx, networkName); err != nil {
		return errors.Annotate(err, "start foil-provision container").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	containerRepo := argsMap.AsString(ctx, "container_repo", "us-docker.pkg.dev/cros-registry/test-services/foil-provision")
	containerTag := argsMap.AsString(ctx, "container_tag", "prod")
	volumes := argsMap.AsStringSlice(ctx, "container_volumes", []string{"/creds:/creds"})
	artifactDir := argsMap.AsString(ctx, "artifact_dir", "/tmp/provisionservice")
	containerImage := containerRepo + ":" + containerTag
	containerName := cft.FoilProvisionName(dut)
	req := &api.StartTemplatedContainerRequest{
		Name:           containerName,
		ContainerImage: containerImage,
		Template: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "foil-provision",
					BinaryArgs: []string{
						"server",
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
		return errors.Annotate(err, "start foil-provision container").Err()
	}
	log.Infof(ctx, "Container %q started!", req.Name)
	client, err := foilprovision.ServiceClient(ctx, ctrInfo, dut)
	if err != nil {
		return errors.Annotate(err, "start foil-provision container").Err()
	}
	err = cft.ClientToScope(ctx, dut, client, containerName)
	return errors.Annotate(err, "start foil-provision container").Err()
}

func stopFoilProvisionContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("stop foil-provision container: ctr is not started").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("stop foil-provision container: dut is not provided").Err()
	}
	containerName := cft.FoilProvisionName(dut)
	if err := ctrInfo.StopContainer(ctx, containerName); err != nil {
		return errors.Annotate(err, "stop foil-provision container").Err()
	}
	log.Infof(ctx, "Container %q stopped!", containerName)
	return nil
}

func init() {
	execs.Register("ctr_start_foil_provision_container", startFoilProvisionContainerExec)
	execs.Register("ctr_stop_foil_provision_container", stopFoilProvisionContainerExec)
}
