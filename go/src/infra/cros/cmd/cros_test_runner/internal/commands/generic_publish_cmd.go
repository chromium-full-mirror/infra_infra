// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/commoncommands"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
)

// GenericPublishCmd represents gcloud auth cmd.
type GenericPublishCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	PublishRequest *api.PublishTask
	Identifier     string

	// Updates
	PublishResp *testapi.PublishResponse
}

// Instantiate extracts initial state info from the state keeper.
func (cmd *GenericPublishCmd) Instantiate(
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

func (cmd *GenericPublishCmd) instantiateWithHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) (err error) {

	if err := commoncommands.InstantiatePopFromQueue(sk.PublishQueue, func(element any) {
		cmd.PublishRequest = element.(*api.PublishTask)
	}); err != nil {
		return fmt.Errorf("cmd %s missing dependency: PublishRequest, %s", cmd.GetCommandType(), err)
	}

	return nil
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *GenericPublishCmd) ExtractDependencies(ctx context.Context,
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

func (cmd *GenericPublishCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.PublishRequest == nil {
		return fmt.Errorf("cmd %q missing dependency: PublishRequest", cmd.GetCommandType())
	}

	if err := common.InjectDependencies(cmd.PublishRequest, sk.Injectables, cmd.PublishRequest.DynamicDeps); err != nil {
		logging.Infof(ctx, "Warning: cmd %q failed to inject some dependencies, %s", cmd.GetCommandType(), err)
	}

	cmd.Identifier = cmd.PublishRequest.GetDynamicIdentifier()
	if cmd.Identifier == "" {
		logging.Infof(ctx, "Warning: cmd %q missing preferred dependency: DynamicIdentifier (required for dynamic referencing)", cmd.GetCommandType())
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *GenericPublishCmd) UpdateStateKeeper(
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

func (cmd *GenericPublishCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	taskIdentifier := common.NewTaskIdentifier(cmd.PublishRequest.DynamicIdentifier)
	if cmd.PublishResp != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCResponse("publish"), cmd.PublishResp); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCResponse("publish"))
		}
	}

	// Upload request objects to storage
	if cmd.PublishRequest.PublishRequest != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRPCRequest("publish"), cmd.PublishRequest.PublishRequest); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRPCRequest("publish"))
		}
	}

	return nil
}

func NewGenericPublishCmd(executor interfaces.ExecutorInterface) *GenericPublishCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(GenericPublishCmdType, executor)
	cmd := &GenericPublishCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
