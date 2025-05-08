// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"google.golang.org/protobuf/proto"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// FilterExecutionCmd represents test execution cmd.
type FilterExecutionCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	InputTestPlan           *testapi.InternalTestplan
	ContainerInfo           *data.ContainerInfo
	ContainerRequestChannel chan commontypes.ContainerManagementRequest
	ContainerLogsChannel    chan *commontypes.ContainerLogInfo

	// Updates
	OutputTestPlan *testapi.InternalTestplan

	BuildState *build.State

	// Logging
	BQClient *bigquery.Client
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *FilterExecutionCmd) ExtractDependencies(
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
func (cmd *FilterExecutionCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateFilterStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}
	// set cmd to nil, to release memory
	cmd = nil

	return nil
}

func (cmd *FilterExecutionCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.ContainerInfoQueue.Len() < 1 {
		return fmt.Errorf("cmd %q missing dependency: ContainerInfo", cmd.GetCommandType())
	}

	cmd.ContainerInfo = sk.ContainerInfoQueue.Remove(sk.ContainerInfoQueue.Front()).(*data.ContainerInfo)

	if len(sk.TestPlanStates) == 0 {
		if sk.InitialInternalTestPlan != nil {
			// Set the first state from initial test plan
			sk.TestPlanStates = append(sk.TestPlanStates, sk.InitialInternalTestPlan)
			// Set the cmd input test plan
			cmd.InputTestPlan = proto.Clone(sk.InitialInternalTestPlan).(*testapi.InternalTestplan)
		} else {
			return fmt.Errorf("cmd %q missing dependency: InputTestPlan", cmd.GetCommandType())
		}
	} else {
		// Get the last test plan state and set it as input test plan for current filter
		cmd.InputTestPlan = proto.Clone(sk.TestPlanStates[len(sk.TestPlanStates)-1]).(*testapi.InternalTestplan)
	}

	if sk.BQClient != nil {
		cmd.BQClient = sk.BQClient
	}
	cmd.BuildState = sk.BuildState

	cmd.ContainerRequestChannel = sk.ContainerRequestChannel
	cmd.ContainerLogsChannel = sk.ContainerLogsChannel
	// TODO (azrahman): remove these custom test plans call once ttcp filter stablized.
	// Only to be used to test ttcp filter through led.
	// if cmd.ContainerInfo.GetKey() == "ttcp-demo" {
	// 	cmd.InputTestPlan = addNTests(5)
	// }

	// if cmd.ContainerInfo.GetKey() == "ttcp-demo" {
	// 	cmd.InputTestPlan = addCustomTests()
	// }

	return nil
}

func (cmd *FilterExecutionCmd) updateFilterStateKeeper(ctx context.Context, sk *data.FilterStateKeeper) error {
	if err := common.ValidateTestPlans(cmd.InputTestPlan, cmd.OutputTestPlan); err != nil {
		return fmt.Errorf("cmd %q failed with test plan validation: %w", cmd.GetCommandType(), err)
	}

	// Add the validated output testplan to test plan states.
	sk.TestPlanStates = []*testapi.InternalTestplan{cmd.OutputTestPlan}
	return nil
}

func NewFilterExecutionCmd(executor interfaces.ExecutorInterface) *FilterExecutionCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(FilterExecutionCmdType, executor)
	cmd := &FilterExecutionCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
