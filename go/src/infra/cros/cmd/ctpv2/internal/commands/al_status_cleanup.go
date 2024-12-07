// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/ctpv2/data"
)

type AlStatusCleanUpCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	BuildState *build.State

	// Deps
	AlStateInfo *data.AlStateInfo

	ExecutionError error
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *AlStatusCleanUpCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *AlStatusCleanUpCmd) UpdateStateKeeper(ctx context.Context, ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateScheduleStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *AlStatusCleanUpCmd) updateScheduleStateKeeper(ctx context.Context, sk *data.FilterStateKeeper) error {
	sk.AlStateInfo = cmd.AlStateInfo

	sk.ExecutionError = cmd.ExecutionError

	return nil
}

func (cmd *AlStatusCleanUpCmd) extractDepsFromFilterStateKeeper(ctx context.Context, sk *data.FilterStateKeeper) error {
	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	if sk.BuildState == nil {
		return fmt.Errorf("cmd %q missing dependency: BuildState", cmd.GetCommandType())
	}

	if sk.ExecutionError == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: ExecutionError", cmd.GetCommandType())
	}

	cmd.BuildState = sk.BuildState
	cmd.AlStateInfo = sk.AlStateInfo

	cmd.ExecutionError = sk.ExecutionError
	return nil
}

func (cmd *AlStatusCleanUpCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "AL Status Clean up")
	defer func() { step.End(err) }()

	if cmd.AlStateInfo == nil {
		logging.Infof(ctx, "AlStateInfo needs to be populated for proper cleanup execution of AL work")
		return nil
	}

	if cmd.ExecutionError == nil {
		cmd.ExecutionError = fmt.Errorf("undefined error occurred during suite execution")
	}

	service, err := androidapi.NewAndroidBuildService(ctx, androidapi.SERVICEACCOUNT, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}

	logging.Infof(ctx, "Successfully made Android Build Service, closing work tree")

	err = cmd.AlStateInfo.CloseWUTree(ctx, service, cmd.ExecutionError.Error(), "CTP Infra Failure")
	if err != nil {
		logging.Errorf(ctx, "error while closing WU tree: %w", err)

		// Ignore update failures if being run inside of a LED run.
		if !common.IsLedRun(cmd.BuildState.Build().GetBuilder()) {
			return err
		}
	}

	return nil
}

// NewAlStatusCleanUpCmd returns a new AlStatusUpdateCmd
func NewAlStatusCleanUpCmd() *AlStatusCleanUpCmd {
	abstractCmd := interfaces.NewAbstractCmd(AlStatusCleanUpCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &AlStatusCleanUpCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
