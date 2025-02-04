// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/commoncommands"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
)

// GenericPostProcessCmd represents post process service commands.
type GenericPostProcessCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	PostProcessRequest *testapi.PostTestTask
	Identifier         string

	// Updates
	StartUpResp       *testapi.PostTestStartUpResponse
	RunActivitiesResp *testapi.RunActivitiesResponse
}

// Instantiate extracts initial state info from the state keeper.
func (cmd *GenericPostProcessCmd) Instantiate(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.instantiateWithHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.instantiateWithHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type: %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error while instantiating for command: %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *GenericPostProcessCmd) instantiateWithHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) (err error) {
	if err := commoncommands.InstantiatePopFromQueue(sk.PostTestQueue, func(element any) {
		cmd.PostProcessRequest = element.(*testapi.PostTestTask)
	}); err != nil {
		return fmt.Errorf("cmd: %s missing dependency: PostProcessRequest for err: %w", cmd.GetCommandType(), err)
	}

	return nil
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *GenericPostProcessCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	default:
		return fmt.Errorf("StateKeeper: '%T' is not supported by cmd type: %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command: %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *GenericPostProcessCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {
	if cmd.PostProcessRequest == nil {
		return fmt.Errorf("cmd: %q missing dependency: PostProcessRequest", cmd.GetCommandType())
	}

	if err := common.InjectDependencies(cmd.PostProcessRequest, sk.Injectables, cmd.PostProcessRequest.DynamicDeps); err != nil {
		logging.Warningf(ctx, "Warning: cmd: %q failed to inject some dependencies, %s", cmd.GetCommandType(), err)
	}

	cmd.Identifier = cmd.PostProcessRequest.GetDynamicIdentifier()
	if cmd.Identifier == "" {
		logging.Warningf(ctx, "Warning: cmd: %q missing preferred dependency: DynamicIdentifier (required for dynamic referencing)", cmd.GetCommandType())
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *GenericPostProcessCmd) UpdateStateKeeper(
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
		return errors.Annotate(err, "error during updating for command: %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *GenericPostProcessCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {
	taskIdentifier := common.NewTaskIdentifier(cmd.PostProcessRequest.DynamicIdentifier)
	warningLogFormat := "Warning: cmd: %s failed to set: %s in the Injectables Storage"
	if cmd.StartUpResp != nil {
		startUpRespID := taskIdentifier.GetRPCResponse("startUp")
		if err := sk.Injectables.Set(startUpRespID, cmd.StartUpResp); err != nil {
			logging.Infof(ctx, warningLogFormat, string(cmd.GetCommandType()), startUpRespID)
		}
	}
	if cmd.RunActivitiesResp != nil {
		runActivitiesRespID := taskIdentifier.GetRPCResponse("runActivities")
		if err := sk.Injectables.Set(runActivitiesRespID, cmd.RunActivitiesResp); err != nil {
			logging.Infof(ctx, warningLogFormat, string(cmd.GetCommandType()), runActivitiesRespID)
		}
	}

	// Upload request objects to storage
	if cmd.PostProcessRequest.StartUpRequest != nil {
		startUpReqID := taskIdentifier.GetRPCRequest("startUp")
		if err := sk.Injectables.Set(startUpReqID, cmd.PostProcessRequest.StartUpRequest); err != nil {
			logging.Infof(ctx, warningLogFormat, string(cmd.GetCommandType()), startUpReqID)
		}
	}
	if cmd.PostProcessRequest.RunActivitiesRequest != nil {
		runActivitiesReqID := taskIdentifier.GetRPCRequest("runActivities")
		if err := sk.Injectables.Set(runActivitiesReqID, cmd.PostProcessRequest.RunActivitiesRequest); err != nil {
			logging.Infof(ctx, warningLogFormat, string(cmd.GetCommandType()), runActivitiesReqID)
		}
	}

	return nil
}

// NewGenericPostProcessCmd creates a new generic post process command.
func NewGenericPostProcessCmd(executor interfaces.ExecutorInterface) *GenericPostProcessCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(GenericPostProcessCmdType, executor)
	cmd := &GenericPostProcessCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
