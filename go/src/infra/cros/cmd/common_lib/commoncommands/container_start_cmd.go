// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	ctpv2_data "go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// ContainerStartCmd represents gcloud auth cmd.
type ContainerStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// container info data associated with this cmd.
	containerInfo *ctpv2_data.ContainerInfo

	// Deps
	ContainerRequest *testapi.ContainerRequest
	ContainerImage   string

	// Updates
	Endpoint          *labapi.IpEndpoint
	ContainerInstance interfaces.ContainerInterface

	// For internal use only
	// skip starting the container (skips execute)
	SkipStartingContainer bool

	// For BQ logging
	Req        *testapi.InternalTestplan
	BQClient   *bigquery.Client
	BuildState *build.State
}

// Instantiate extracts initial state info from the state keeper.
func (cmd *ContainerStartCmd) Instantiate(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.instantiateWithHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.instantiateWithHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	case *ctpv2_data.FilterStateKeeper:
		err = cmd.instantiateWithFilterStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error while instantiating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ContainerStartCmd) instantiateWithHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) (err error) {
	// Catch panics from bad cast.
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	if sk.ContainerQueue.Len() < 1 {
		return fmt.Errorf("cmd %q missing dependency: ContainerRequest", cmd.GetCommandType())
	}
	cmd.ContainerRequest = sk.ContainerQueue.Remove(sk.ContainerQueue.Front()).(*testapi.ContainerRequest)

	return nil
}

func (cmd *ContainerStartCmd) instantiateWithFilterStateKeeper(
	ctx context.Context,
	sk *ctpv2_data.FilterStateKeeper) (err error) {
	// No implementation needed for this. But since abstract implementation is
	// already overridden, we need to return nil to avoid runtime error.
	return nil
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ContainerStartCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *ContainerStartCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ContainerStartCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.ContainerRequest == nil {
		return fmt.Errorf("cmd %q missing dependency: ContainerRequest", cmd.GetCommandType())
	}

	if err := common.InjectDependencies(cmd.ContainerRequest.Container, sk.Injectables, cmd.ContainerRequest.DynamicDeps); err != nil {
		logging.Infof(ctx, "Warning: cmd %q failed to inject some dependencies, %s", cmd.GetCommandType(), err)
	}

	if cmd.ContainerRequest.ContainerImagePath != "" {
		cmd.ContainerImage = cmd.ContainerRequest.ContainerImagePath
	} else {
		containerImage, err := common.GetContainerImageFromMap(cmd.ContainerRequest.ContainerImageKey, sk.ContainerImages)
		if err != nil {
			return fmt.Errorf("cmd %q missing dependency: ContainerImage, %w", cmd.GetCommandType(), err)
		}
		cmd.ContainerImage = containerImage
	}
	cmd.BuildState = sk.BuildState

	return nil
}

func (cmd *ContainerStartCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.Endpoint != nil && cmd.ContainerRequest.DynamicIdentifier != "" {
		err := sk.Injectables.Set(cmd.ContainerRequest.DynamicIdentifier, cmd.Endpoint)
		if err != nil {
			logging.Infof(ctx, "Warning: Failed to set container endpoint for %s, %s", cmd.ContainerRequest.DynamicIdentifier, err)
		}
	}

	if cmd.ContainerInstance != nil && cmd.ContainerRequest.DynamicIdentifier != "" {
		sk.ContainerInstances[cmd.ContainerRequest.ContainerImageKey] = cmd.ContainerInstance
	}

	return nil
}

func NewContainerStartCmd(executor interfaces.ExecutorInterface) *ContainerStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ContainerStartCmdType, executor)
	cmd := &ContainerStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
