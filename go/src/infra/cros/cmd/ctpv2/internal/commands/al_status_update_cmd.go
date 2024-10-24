// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/outputprops"
	"infra/cros/cmd/ctpv2/data"
)

// AlStatusUpdateCmd represents al state update cmd.
type AlStatusUpdateCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	BuildState *build.State

	// Deps
	AlStateInfo *data.AlStateInfo
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *AlStatusUpdateCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PrePostFilterStateKeeper:
		err = cmd.extractDepsFromPrePostStateKeeper(ctx, sk)
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *AlStatusUpdateCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

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

func (cmd *AlStatusUpdateCmd) updateScheduleStateKeeper(ctx context.Context, sk *data.FilterStateKeeper) error {
	sk.AlStateInfo = cmd.AlStateInfo

	return nil
}

func (cmd *AlStatusUpdateCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	if sk.BuildState == nil {
		return fmt.Errorf("cmd %q missing dependency: BuildState", cmd.GetCommandType())
	}

	cmd.AlStateInfo = sk.AlStateInfo
	cmd.BuildState = sk.BuildState

	return nil
}

func (cmd *AlStatusUpdateCmd) extractDepsFromPrePostStateKeeper(
	ctx context.Context,
	sk *data.PrePostFilterStateKeeper) error {

	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	cmd.AlStateInfo = sk.AlStateInfo

	return nil
}

func (cmd *AlStatusUpdateCmd) initRunLayer() error {
	top := cmd.AlStateInfo.WorkUnitTrees["test"]
	if top == nil {
		return fmt.Errorf("WU tree was not initialized")
	}

	runs, err := top.FetchRunLayer()
	if err != nil {
		return err
	}

	if len(runs) == 0 {
		fmt.Printf("top Parent %s-%s#%d: %+v\n", top.GetWorkUnit().Id, top.GetWorkUnit().Name, top.GetIndex(), top)

		// Generate and insert the Run Node into the WU tree.
		runNode, err := androidapi.NewWorkUnitNode(top.GetWorkUnit().Id, top.GetWorkUnit().InvocationId, androidapi.Run, top, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
		if err != nil {
			return err
		}

		fmt.Printf("NEW RUN Node %s-%s#%d: %+v\n", runNode.GetWorkUnit().Id, runNode.GetWorkUnit().Name, runNode.GetIndex(), runNode)
	}

	return nil
}

func patchRunsAndTestJob(service *androidapi.Service, top *androidapi.WorkUnitNode) error {
	runNodes, err := top.FetchRunLayer()
	if err != nil {
		return err
	}

	allRunsSuccessful := true
	for _, run := range runNodes {
		allSuccess := true
		for _, attempt := range run.GetChildren() {
			if strings.ToLower(attempt.GetWorkUnit().State) != "completed" {
				allSuccess = false
				break
			}
		}

		if !allSuccess {
			allRunsSuccessful = false
			run.GetWorkUnit().State = "ERROR"
		} else {
			run.GetWorkUnit().State = "COMPLETED"
		}

		patchedRunWU, err := service.WorkUnitService.Patch(run.GetWorkUnit().Id, run.GetWorkUnit())
		if err != nil {
			return err
		}

		run.SetWorkUnit(patchedRunWU)
	}

	if !allRunsSuccessful {
		top.GetWorkUnit().State = "ERROR"
	} else {
		top.GetWorkUnit().State = "COMPLETED"
	}

	patchedTopWU, err := service.WorkUnitService.Patch(top.GetWorkUnit().Id, top.GetWorkUnit())
	if err != nil {
		return err
	}

	top.SetWorkUnit(patchedTopWU)

	return nil
}

func (cmd *AlStatusUpdateCmd) closeWUTree() error {
	top := cmd.AlStateInfo.WorkUnitTrees["test"]
	if top == nil {
		return fmt.Errorf("wu tree was removed unexpectedly")
	}

	// TODO(b/372507028): Pass this in rather than create a new one each time
	service, err := androidapi.NewAndroidBuildService(context.Background(), androidapi.SERVICEACCOUNT, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}

	err = patchRunsAndTestJob(service, top)
	if err != nil {
		return err
	}

	return nil
}

// Execute executes the command.
func (cmd *AlStatusUpdateCmd) Execute(ctx context.Context) error {
	var err error
	// Skip the step completely for now if the event state is nil
	// TODO (azrahman:atp): undo this once output props support is added here
	if cmd.AlStateInfo == nil || cmd.AlStateInfo.CurrentTestJobEvent == nil {
		return nil
	}

	// WORK UNIT MAINTENANCE
	err = cmd.initRunLayer()
	if err != nil {
		return err
	}

	if cmd.AlStateInfo.DoneTesting {
		err = cmd.closeWUTree()
		if err != nil {
			return err
		}
	}
	// WORK UNIT MAINTENANCE

	currTestJobEvent := cmd.AlStateInfo.CurrentTestJobEvent

	step, ctx := build.StartStep(ctx, "Al Status Update")
	defer func() { step.End(err) }()

	// Publish to pub/sub
	common.WriteAnyObjectToStepLog(ctx, step, currTestJobEvent, "current test job event state")
	id, err := common.PublishToTestJobEventPubSub(ctx, cmd.AlStateInfo.TestJobEventPubSubClient, currTestJobEvent)
	if err != nil {
		common.WriteStringToStepLog(ctx, step, fmt.Sprintf("err while publishing with id %s: %s", id, err.Error()), "pubsub publish error")
		err = nil
	}

	// Update output props
	if cmd.AlStateInfo.CurrentTestJobEvent.TestJob != nil {
		updateItems := &outputprops.UpdateItems{TestJobMsgJson: cmd.AlStateInfo.CurrentTestJobEvent.TestJob}
		encodedTestJobMsg, err := common.EncodeAnyObj(cmd.AlStateInfo.CurrentTestJobEvent.TestJob)
		if err != nil {
			common.WriteStringToStepLog(ctx, step, fmt.Sprintf("err while encoding test job msg: %s", err.Error()), "encoding error")
			err = nil
		} else {
			common.WriteAnyObjectToStepLog(ctx, step, encodedTestJobMsg, "encoded msg")
			updateItems.EncodedTestJobMsg = encodedTestJobMsg
		}

		// DO NOT CHANGE `TestJobInfo` key until multiple request support is added for ATP flow.
		// It will be changed to SuiteName when that support will be introduced. Currently, ATP depends on this key.
		outputprops.CTPv2AtpUpdate.SetOutput(ctx, outputprops.SummaryMap{"TestJobInfo": updateItems})
	}

	return err
}

// NewAlStatusUpdateCmd returns a new AlStatusUpdateCmd
func NewAlStatusUpdateCmd() *AlStatusUpdateCmd {
	abstractCmd := interfaces.NewAbstractCmd(AlStatusUpdateCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &AlStatusUpdateCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
