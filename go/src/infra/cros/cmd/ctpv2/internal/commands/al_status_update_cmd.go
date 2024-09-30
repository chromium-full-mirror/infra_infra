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

	SuiteTestResults map[string]*data.TestResults
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
	cmd.SuiteTestResults = sk.SuiteTestResults

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

	common.WriteAnyObjectToStepLog(ctx, step, currTestJobEvent, "current test job event state")
	id, err := common.PublishToTestJobEventPubSub(ctx, cmd.AlStateInfo.TestJobEventPubSubClient, currTestJobEvent)
	if err != nil {
		common.WriteStringToStepLog(ctx, step, fmt.Sprintf("err while publishing with id %s: %s", id, err.Error()), "pubsub publish error")
		err = nil
	}

	// If there are results in the map (after the schedule tests step) then
	// write the result totals to the output props.
	if cmd.SuiteTestResults != nil {
		// Collect the results from the each the the test results
		summaryTotals := outputprops.SummaryMap{}

		for _, testResult := range cmd.SuiteTestResults {
			// TODO(juahurta): Only track the last attempt. In the future we do want
			// to count the results if the test was retried.
			key := testResult.Suite
			if _, ok := summaryTotals[key]; !ok {
				summaryTotals[key] = &outputprops.SummaryItem{
					TotalTestCount:          0,
					TotalFailedTestCount:    0,
					TotalFailedTestRunCount: 0,
				}
			}

			// Get and add the totals to the tracking map.
			totalTestCount, totalFailedTestCount, totalFailedTestRunCount := testResult.GetTestCounts()
			summaryTotals[key].TotalTestCount += totalTestCount
			summaryTotals[key].TotalFailedTestCount += totalFailedTestCount
			summaryTotals[key].TotalFailedTestRunCount += totalFailedTestRunCount
		}

		// Write the pass/fail summary map to the sub-build's output properties.
		outputprops.CTPv2PassFail.SetOutput(ctx, summaryTotals)
	}

	return err
}

// NewAlStatusUpdateCmd returns a new AlStatusUpdateCmd
func NewAlStatusUpdateCmd() *AlStatusUpdateCmd {
	abstractCmd := interfaces.NewAbstractCmd(AlStatusUpdateCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &AlStatusUpdateCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
