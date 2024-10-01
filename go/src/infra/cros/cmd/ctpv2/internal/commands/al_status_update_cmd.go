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

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/outputprops"
	"infra/cros/cmd/ctpv2/data"
)

// AlStatusUpdateCmd represents al state update cmd.
type AlStatusUpdateCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

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

	return nil
}

func (cmd *AlStatusUpdateCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	cmd.AlStateInfo = sk.AlStateInfo

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

// Execute executes the command.
func (cmd *AlStatusUpdateCmd) Execute(ctx context.Context) error {
	var err error
	// Skip the step completely for now if the event state is nil
	// TODO (azrahman:atp): undo this once output props support is added here
	if cmd.AlStateInfo == nil || cmd.AlStateInfo.CurrentTestJobEvent == nil {
		return nil
	}

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
