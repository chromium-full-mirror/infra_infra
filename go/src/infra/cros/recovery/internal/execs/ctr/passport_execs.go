// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"
	"strings"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/lab/api/passport"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/dev"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/components/pasit"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/localproxy"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// passportAddressNotInScopeExec verifies that the container is known to the current scope.
func passportAddressNotInScopeExec(ctx context.Context, info *execs.ExecInfo) error {
	if _, err := cft.AddressFromScope(ctx, cft.PassportName(info.GetDut())); err != nil {
		return nil
	}
	return errors.Reason("cros-passport container address not in scope: Container address was found in scope").Err()
}

// passportStartContainerExec starts cros-passport container using ctr.
func passportStartContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	address := info.GetChromeos().GetPasit().GetHostname()
	if dev.IsActive(ctx) {
		address = localproxy.BuildAddr(address)
	}

	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("start cros-passport: dut is not provided").Err()
	}

	var conn *grpc.ClientConn
	var err error
	if !strings.HasPrefix(address, "satlab") || dev.IsActive(ctx) {
		// If not satlab or we are in dev mode -> just connect directly.
		conn, err = common.ConnectWithService(ctx, address)
		if err != nil {
			return errors.Annotate(err, "connect to existing container: get client %q", address).Err()
		}
	} else {
		// If host is a satlab -> launch using ctr.
		ctrInfo, ok := ctr.Get(ctx)
		if !ok {
			return errors.Reason("start cros-passport: ctr is not started").Err()
		}
		networkName := cft.NetworkName(dut)
		if _, err := ctrInfo.GetNetwork(ctx, networkName); err != nil {
			return errors.Annotate(err, "start cros-passport container").Err()
		}
		argsMap := info.GetActionArgs(ctx)
		containerTag := argsMap.AsString(ctx, "container_tag", "prod_cros-passport")
		volumes := argsMap.AsStringSlice(ctx, "container_volumes", []string{"/dev:/dev"})
		artifactDir := argsMap.AsString(ctx, "artifact_dir", "/tmp/cros-passport")
		containerImage, err := ctrInfo.GenerateContainerImagePath(ctx, cft.Passport, containerTag)
		if err != nil {
			return errors.Annotate(err, "start cros-passport container").Err()
		}
		containerName := cft.PassportName(dut)
		req := &api.StartTemplatedContainerRequest{
			Name:           containerName,
			ContainerImage: containerImage,
			Template: &api.Template{
				Container: &api.Template_Generic{
					Generic: &api.GenericTemplate{
						BinaryName:        "cros-passport",
						AdditionalVolumes: volumes,
						DockerArtifactDir: artifactDir,
					},
				},
			},
			Network: networkName,
		}
		if _, err := ctrInfo.CreateContainer(ctx, req); err != nil {
			return errors.Annotate(err, "start cros-passport container").Err()
		}
		log.Infof(ctx, "Container %q started!", req.Name)

		conn, err = pasit.ContainerClient(ctx, ctrInfo, dut)
		if err != nil {
			return err
		}
	}

	if err := pasit.InitClients(ctx, conn, dut); err != nil {
		return errors.Annotate(err, "start cros-passport container").Err()
	}

	if err := cft.ServiceAddressToScope(ctx, cft.PassportName(dut), address); err != nil {
		return errors.Annotate(err, "start cros-passport container").Err()
	}

	return nil
}

// pasportStopContainerExec stops the running Passport container.
func pasportStopContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("stop cros-passport container: ctr is not started").Err()
	}
	containerName := cft.PassportName(info.GetDut())
	if err := ctrInfo.StopContainer(ctx, containerName); err != nil {
		return errors.Annotate(err, "stop cros-passport container").Err()
	}
	log.Infof(ctx, "Container %q stopped!", containerName)
	return nil
}

// passportResetResetSwitchesExec resets all switches connected to the PassPort Host.
func passportResetResetSwitchesExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.PassportSwitchClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "reset cros-passport switches: get switch client").Err()
	}

	if _, err := client.ResetAllSwitches(ctx, &passport.ResetAllSwitchesRequest{}); err != nil {
		return errors.Annotate(err, "reset cros-passport switches: call ResetAllSwitches").Err()
	}

	return nil
}

func init() {
	execs.Register("ctr_passport_address_not_in_scope", passportAddressNotInScopeExec)
	execs.Register("ctr_passport_start", passportStartContainerExec)
	execs.Register("ctr_passport_reset_switches", passportResetResetSwitchesExec)
	execs.Register("ctr_passport_stop", pasportStopContainerExec)
}
