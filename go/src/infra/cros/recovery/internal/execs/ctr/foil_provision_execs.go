// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"

	lab_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/components/cft/foilprovision"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/version"
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
	containerTag := argsMap.AsString(ctx, "container_tag", "prod")
	volumes := argsMap.AsStringSlice(ctx, "container_volumes", []string{"/creds:/creds"})
	artifactDir := argsMap.AsString(ctx, "artifact_dir", "/tmp/provisionservice")
	containerImage, err := ctrInfo.GenerateContainerImagePath(ctx, cft.FoilProvision, containerTag)
	if err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
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
	if err := cft.ClientToScope(ctx, dut, client, containerName); err != nil {
		return errors.Annotate(err, "start foil-provision container").Err()
	}
	if err := cft.AddressToScope(ctx, ctrInfo, containerName); err != nil {
		return errors.Annotate(err, "start foil-provision container").Err()
	}
	return nil
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

func setupFoilProvisionServiceExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.FoilProvisionClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("setup foil-provision service: client is not found").Err()
	}
	dut := info.GetDut()
	if dut.GetChromeos() == nil {
		return errors.Reason("setup foil-provision service: dut is not detected").Err()
	}
	var servoNexusAddr, cachingAddress *lab_api.IpEndpoint
	argsMap := info.GetActionArgs(ctx)
	// All provisions required USB-drive.
	if argsMap.AsBool(ctx, "provide_servo_nexus", true) {
		if addr, err := cft.ServoServiceAddressFromScope(ctx, dut); err != nil {
			return errors.Annotate(err, "start servo-nexus container").Err()
		} else {
			servoNexusAddr = addr
		}
	}
	if addr, err := cft.CacheServiceAddressFromScope(ctx); err != nil {
		return errors.Annotate(err, "setup foil-provision service").Err()
	} else {
		cachingAddress = addr
	}
	if err := foilprovision.Setup(ctx, client, dut, toLabDut(dut, cachingAddress), servoNexusAddr); err != nil {
		return errors.Annotate(err, "setup foil-provision service").Err()
	}
	log.Debugf(ctx, "Foil-provision setup pass for %q!", info.GetDut().Name)
	return nil
}

func installFoilProvisionExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.FoilProvisionClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Reason("install foil-provision service: client is not found").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("install foil-provision service: dut is not detected").Err()
	}
	sv, err := version.ByResource(ctx, version.AndroidOSType, dut, dut.Name)
	if err != nil {
		return errors.Annotate(err, "install foil-provision service").Err()
	}
	log.Debugf(ctx, "Foil-provision uses image: %q", sv.GetOsImagePath())
	argsMap := info.GetActionArgs(ctx)
	preventReboot := argsMap.AsBool(ctx, "prevent_reboot", false)
	imagePath := &lab_go.StoragePath{
		HostType: lab_go.StoragePath_GS,
		Path:     sv.GetOsImagePath(),
	}
	if err := foilprovision.Install(ctx, client, imagePath, preventReboot); err != nil {
		return errors.Annotate(err, "install foil-provision service").Err()
	}
	log.Debugf(ctx, "Foil-provision install pass for %q!", info.GetDut().Name)
	return nil
}

func init() {
	execs.Register("ctr_start_foil_provision_container", startFoilProvisionContainerExec)
	execs.Register("ctr_stop_foil_provision_container", stopFoilProvisionContainerExec)
	execs.Register("ctr_foil_provision_setup_service", setupFoilProvisionServiceExec)
	execs.Register("ctr_foil_provision_install", installFoilProvisionExec)
}
