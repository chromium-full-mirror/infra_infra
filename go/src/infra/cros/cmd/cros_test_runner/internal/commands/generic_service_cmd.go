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

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// GenericServiceCmd represents gcloud auth cmd.
type GenericServiceCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	GenericRequest *testapi.GenericTask
	Identifier     string

	// Updates
	StartResp *testapi.GenericStartResponse
	RunResp   *testapi.GenericRunResponse
	StopResp  *testapi.GenericStopResponse
}

// Instantiate extracts initial state info from the state keeper.
func (cmd *GenericServiceCmd) Instantiate(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.instantiateWithHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.instantiateWithHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error while instantiating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *GenericServiceCmd) instantiateWithHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) (err error) {

	if err := commoncommands.InstantiatePopFromQueue(sk.GenericQueue, func(element any) {
		cmd.GenericRequest = element.(*testapi.GenericTask)
	}); err != nil {
		return fmt.Errorf("cmd %s missing dependency: GenericRequest, %w", cmd.GetCommandType(), err)
	}

	return nil
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *GenericServiceCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *GenericServiceCmd) UpdateStateKeeper(
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

func (cmd *GenericServiceCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.GenericRequest == nil {
		return fmt.Errorf("cmd %q missing dependency: GenericRequest", cmd.GetCommandType())
	}

	if err := common.InjectDependencies(cmd.GenericRequest, sk.Injectables, cmd.GenericRequest.DynamicDeps); err != nil {
		logging.Infof(ctx, "Warning: cmd %q failed to inject some dependencies, %s", cmd.GetCommandType(), err)
	}

	cmd.Identifier = cmd.GenericRequest.GetDynamicIdentifier()
	if cmd.Identifier == "" {
		logging.Infof(ctx, "Warning: cmd %q missing preferred dependency: DynamicIdentifier (required for dynamic referencing)", cmd.GetCommandType())
	}

	return nil
}

func (cmd *GenericServiceCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	taskIdentifier := common.NewTaskIdentifier(cmd.GenericRequest.DynamicIdentifier)
	if cmd.StartResp != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCResponse("start"), cmd.StartResp); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCResponse("start"))
		}
	}
	if cmd.RunResp != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCResponse("run"), cmd.RunResp); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCResponse("run"))
		}
	}
	if cmd.StopResp != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCResponse("stop"), cmd.StopResp); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCResponse("stop"))
		}
	}

	// Upload request objects to storage
	if cmd.GenericRequest.StartRequest != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCRequest("start"), cmd.GenericRequest.StartRequest); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCRequest("start"))
		}
	}
	if cmd.GenericRequest.RunRequest != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCRequest("run"), cmd.GenericRequest.RunRequest); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCRequest("run"))
		}
	}
	if cmd.GenericRequest.StopRequest != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCRequest("stop"), cmd.GenericRequest.StopRequest); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCRequest("stop"))
		}
	}

	return nil
}

func NewGenericServiceCmd(executor interfaces.ExecutorInterface) *GenericServiceCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(GenericServiceCmdType, executor)
	cmd := &GenericServiceCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
