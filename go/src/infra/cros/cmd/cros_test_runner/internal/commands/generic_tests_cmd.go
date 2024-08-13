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
	"infra/cros/cmd/common_lib/common_commands"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
)

// GenericTestsCmd represents gcloud auth cmd.
type GenericTestsCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	TestRequest *api.TestTask
	Identifier  string

	// Updates
	TestResponses *testapi.CrosTestResponse
}

// Instantiate extracts initial state info from the state keeper.
func (cmd *GenericTestsCmd) Instantiate(
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

func (cmd *GenericTestsCmd) instantiateWithHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) (err error) {

	if err := common_commands.Instantiate_PopFromQueue(sk.TestQueue, func(element any) {
		cmd.TestRequest = element.(*api.TestTask)
	}); err != nil {
		return fmt.Errorf("cmd %s missing dependency: TestRequest, %s", cmd.GetCommandType(), err)
	}

	return nil
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *GenericTestsCmd) ExtractDependencies(ctx context.Context,
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
func (cmd *GenericTestsCmd) UpdateStateKeeper(
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

func (cmd *GenericTestsCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.TestRequest == nil {
		return fmt.Errorf("cmd %q missing dependency: TestRequest", cmd.GetCommandType())
	}

	if err := common.InjectDependencies(cmd.TestRequest, sk.Injectables, cmd.TestRequest.DynamicDeps); err != nil {
		logging.Infof(ctx, "Warning: cmd %q failed to inject some dependencies, %s", cmd.GetCommandType(), err)
	}

	cmd.Identifier = cmd.TestRequest.GetDynamicIdentifier()
	if cmd.Identifier == "" {
		logging.Infof(ctx, "Warning: cmd %q missing preferred dependency: DynamicIdentifier (required for dynamic referencing)", cmd.GetCommandType())
	}

	return nil
}

func (cmd *GenericTestsCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	taskIdentifier := common.NewTaskIdentifier(cmd.TestRequest.DynamicIdentifier)
	if cmd.TestResponses != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRpcResponse("runTests"), cmd.TestResponses); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRpcResponse("runTests"))
		}
		sk.TestResponses = cmd.TestResponses
		rdbTestResult, err := constructTestResultFromStateKeeper(ctx, sk)
		if err != nil {
			return errors.Annotate(err, "Cmd %q failed to construct update: TestResultForRdb", cmd.GetCommandType()).Err()
		}
		sk.TestResultForRdb = rdbTestResult
		if err := sk.Injectables.Set(taskIdentifier.GetRpcResponse("rdbTestResult"), sk.TestResultForRdb); err != nil {
			logging.Infof(ctx, "Warning: failed to set %s into the InjectableStorage, %s", taskIdentifier.GetRpcResponse("rdbTestResult"), err)
		}
	}

	// Upload request objects to storage
	if cmd.TestRequest.TestRequest != nil {
		if err := sk.Injectables.Set(taskIdentifier.GetRpcRequest("test"), cmd.TestRequest.TestRequest); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the Injectables Storage, %s", string(cmd.GetCommandType()), taskIdentifier.GetRpcRequest("test"))
		}
	}

	return nil
}

func NewGenericTestsCmd(executor interfaces.ExecutorInterface) *GenericTestsCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(GenericTestsCmdType, executor)
	cmd := &GenericTestsCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
