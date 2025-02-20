// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	ctpv2_data "go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// GcloudAuthCmd represents gcloud auth cmd.
type GcloudAuthCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	DockerKeyFileLocation string

	// Config auth mode
	UseDockerKeyDirectly bool
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *GcloudAuthCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromLocalTestStateKeeper(ctx, sk)
	case *ctpv2_data.PrePostFilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *GcloudAuthCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if sk.DockerKeyFileLocation == "" {
		return fmt.Errorf("cmd %q missing dependency: DockerKeyFileLocation", cmd.GetCommandType())
	}
	cmd.DockerKeyFileLocation = sk.DockerKeyFileLocation

	return nil
}

func (cmd *GcloudAuthCmd) extractDepsFromLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if sk.DockerKeyFileLocation == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: DockerKeyFileLocation", cmd.GetCommandType())
	}
	cmd.DockerKeyFileLocation = sk.DockerKeyFileLocation
	cmd.UseDockerKeyDirectly = sk.UseDockerKeyDirectly
	if sk.UseDockerKeyDirectly {
		logging.Infof(ctx, "Info: using service account key directly for docker login")
	}

	return nil
}

func (cmd *GcloudAuthCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *ctpv2_data.PrePostFilterStateKeeper) error {

	if sk.DockerKeyFileLocation == "" {
		return fmt.Errorf("cmd %q missing dependency: DockerKeyFileLocation", cmd.GetCommandType())
	}
	cmd.DockerKeyFileLocation = sk.DockerKeyFileLocation

	return nil
}

func NewGcloudAuthCmd(executor interfaces.ExecutorInterface) *GcloudAuthCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(GcloudAuthCmdType, executor)
	cmd := &GcloudAuthCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
