// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// BuildDutTopologyCmd represents load dut toplogy cmd.
type BuildDutTopologyCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	DutSshAddress      *labapi.IpEndpoint
	CacheServerAddress *labapi.IpEndpoint
	Board              string

	// Updates
	DutTopology *labapi.DutTopology
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *BuildDutTopologyCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromLocalTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *BuildDutTopologyCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.LocalTestStateKeeper:
		err = cmd.updateLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *BuildDutTopologyCmd) extractDepsFromLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if sk.Args.BuildBoard == "" {
		return fmt.Errorf("Cmd %q missing dependency: Board", cmd.GetCommandType())
	}
	cmd.Board = sk.Args.BuildBoard

	if sk.DutSshAddress == nil {
		return fmt.Errorf("Cmd %q missing dependency: DutSshAddress", cmd.GetCommandType())
	}
	cmd.DutSshAddress = sk.DutSshAddress

	if sk.DutCacheServerAddress == nil {
		return fmt.Errorf("Cmd %q missing dependency: CacheServerAddress", cmd.GetCommandType())
	}
	cmd.CacheServerAddress = sk.DutCacheServerAddress

	return nil
}

func (cmd *BuildDutTopologyCmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if cmd.DutTopology != nil {
		sk.DutTopology = cmd.DutTopology
	}

	return nil
}

func NewBuildDutTopologyCmd(executor interfaces.ExecutorInterface) *BuildDutTopologyCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(BuildDutTopologyCmdType, executor)
	cmd := &BuildDutTopologyCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
