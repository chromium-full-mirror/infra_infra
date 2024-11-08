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
	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/outputprops"
	"infra/cros/cmd/ctpv2/data"
)

// AlStatusUpdateCmd represents al state update cmd.
type AlStatusUpdateCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	BuildState *build.State

	BuildsMap map[string]*data.BuildRequest

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
	cmd.BuildsMap = sk.BuildsMap

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

func (cmd *AlStatusUpdateCmd) getWorkUnitTree() *androidapi.WorkUnitTree {
	if cmd.AlStateInfo == nil {
		return nil
	}

	if cmd.AlStateInfo.WorkUnitTrees == nil {
		return nil
	}

	if tree, ok := cmd.AlStateInfo.WorkUnitTrees["test"]; ok {
		return tree
	}

	return nil
}

func (cmd *AlStatusUpdateCmd) initRunAndShards(ctx context.Context) error {
	if cmd.BuildsMap == nil {
		return nil
	}

	tree := cmd.getWorkUnitTree()
	if tree == nil {
		return fmt.Errorf("WU tree was not initialized")
	}

	head := tree.Head

	runs, err := head.FetchRunLayer()
	if err != nil {
		return err
	}

	// Only run this command once.
	if len(runs) > 0 {
		return nil
	}

	fmt.Printf("TOP Parent %s-%s#%d: %+v\n", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetIndex(), head)

	// Generate and insert the Run Node into the WU tree.
	runNode, err := androidapi.NewWorkUnitNode(head.GetWorkUnit().Id, head.GetWorkUnit().InvocationId, androidapi.WULayerRun, head, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}
	fmt.Printf("NEW RUN Node %s-%s#%d: %+v\n", runNode.GetWorkUnit().Id, runNode.GetWorkUnit().Name, runNode.GetIndex(), runNode)

	for key := range cmd.BuildsMap {
		// NOTE: Shards are unique for a given tree/run. When we migrate to
		// multiple runs this will not collide since they'll be in separate
		// trees.
		if _, ok := tree.ShardsByKey[key]; !ok {
			fmt.Printf("Run Parent %s-%s#%d: %+v\n", runNode.GetWorkUnit().Id, runNode.GetWorkUnit().Name, runNode.GetIndex(), head)

			// Generate and insert the Run Node into the WU tree.
			shardNode, err := androidapi.NewWorkUnitNode(runNode.GetWorkUnit().Id, runNode.GetWorkUnit().InvocationId, androidapi.WULayerShard, runNode, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
			if err != nil {
				return err
			}

			fmt.Printf("NEW SHARD Node %s-%s#%d: %+v\n", shardNode.GetWorkUnit().Id, shardNode.GetWorkUnit().Name, shardNode.GetIndex(), shardNode)
			tree.ShardsByKey[key] = shardNode
		}
	}

	return nil
}

// updateAllNodes recursively iterates through the tree updating the status of
// each node from the bottom layers upward.
//
// NOTE: This leverages the fact that we set the state for the attempt nodes
// inside the schedule_tasks command.
func updateAllNodes(ctx context.Context, service *androidapi.Service, head *androidapi.WorkUnitNode) error {
	children := head.GetChildren()
	// Once we've reached the attempt layer update the WU and return.
	if children.Len() == 0 {
		newWU, err := service.WorkUnitService.Update(head.GetWorkUnit().Id, head.GetWorkUnit())
		if err != nil {
			return err
		}
		head.SetWorkUnit(newWU)

		return nil
	}

	// Determine if all child nodes passed.
	allPassed := true
	for _, child := range children {
		// Recursively call on all children so they can update their statuses
		err := updateAllNodes(ctx, service, child)
		if err != nil {
			return err
		}

		if strings.ToLower(child.GetWorkUnit().State) != strings.ToLower(common.TaskCompletedState) {
			fmt.Printf("%s-%s: child %s-%s in state %s, allPassed set to FALSE", head.GetWorkUnit().Id, head.GetWorkUnit().Name, child.GetWorkUnit().Id, child.GetWorkUnit().Name, child.GetWorkUnit().State)
			allPassed = false
		}
	}

	// If the WU changed in anyway inside TestRunner then our current WU
	// is going to be outdated. This will refresh the CTP WU so that we
	// can make updates without conflict.
	refreshedWU, err := head.Service.Get(head.GetWorkUnit().Id)
	if err != nil {
		return err
	}
	head.SetWorkUnit(refreshedWU)

	// Update the state of the current node based on the child nodes.
	if allPassed {
		head.GetWorkUnit().State = common.TaskCompletedState
		fmt.Printf("WU %s-%s set as %s, all children passed", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetWorkUnit().State)
	} else {
		head.GetWorkUnit().State = common.TaskErrorState
		fmt.Printf("WU %s-%s set as %s, not all children passed", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetWorkUnit().State)
		head.GetWorkUnit().DebugInfo = &androidbuildinternal.DebugInfo{
			ErrorCode:    1,
			ErrorMessage: "Not all children WUs passed",
			ErrorName:    "Failed Children",
		}
		fmt.Printf("WU %s-%s completed testing in %s status", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetWorkUnit().State)
	}

	// Send the WU to the ATP API to be updated.
	newWU, err := service.WorkUnitService.Update(head.GetWorkUnit().Id, head.GetWorkUnit())
	if err != nil {
		return err
	}

	// Insert the returned (updated) WU into the current node.
	head.SetWorkUnit(newWU)

	return nil
}

func (cmd *AlStatusUpdateCmd) closeWUTree(ctx context.Context) error {
	tree := cmd.getWorkUnitTree()
	if tree == nil {
		return fmt.Errorf("wu tree was removed unexpectedly")
	}

	service, err := androidapi.NewAndroidBuildService(context.Background(), androidapi.SERVICEACCOUNT, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}

	// Update the status of each WU.
	err = updateAllNodes(ctx, service, tree.Head)
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

	step, ctx := build.StartStep(ctx, "Al Status Update")
	defer func() { step.End(err) }()

	// WORK UNIT MAINTENANCE
	err = cmd.initRunAndShards(ctx)
	if err != nil {
		fmt.Printf("error while initing run layer: %s", err.Error())
		if !common.IsLedRun(cmd.BuildState.Build().GetBuilder()) {
			return err
		}
	}

	if cmd.AlStateInfo.DoneTesting {
		err = cmd.closeWUTree(ctx)
		if err != nil {
			logging.Errorf(ctx, "error while closing WU tree: %s", err.Error())

			// If this is being ran inside of a LED run then ignore the update
			// failures.
			if !common.IsLedRun(cmd.BuildState.Build().GetBuilder()) {
				return err
			}
		}
	}
	// WORK UNIT MAINTENANCE END

	currTestJobEvent := cmd.AlStateInfo.CurrentTestJobEvent

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
