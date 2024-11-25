// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
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

	// CTPRequest
	CtpRequest *testapi.CTPRequest
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
		err = cmd.updateScheduleStateKeeper(sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *AlStatusUpdateCmd) updateScheduleStateKeeper(sk *data.FilterStateKeeper) error {
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

	if sk.CtpReq == nil {
		return fmt.Errorf("cmd %q missing dependency: CtpRequest", cmd.GetCommandType())
	}

	cmd.AlStateInfo = sk.AlStateInfo
	cmd.BuildState = sk.BuildState
	cmd.BuildsMap = sk.BuildsMap
	cmd.CtpRequest = sk.CtpReq

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

	logging.Infof(ctx, "TOP Parent %s-%s#%d: %+v\n", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetIndex(), head)

	// Generate and insert the Run Node into the WU tree.
	runNode, err := androidapi.NewWorkUnitNode(head.GetWorkUnit().Id, head.GetWorkUnit().InvocationId, androidapi.WULayerRun, head, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}
	logging.Infof(ctx, "NEW RUN Node %s-%s#%d: %+v\n", runNode.GetWorkUnit().Id, runNode.GetWorkUnit().Name, runNode.GetIndex(), runNode)

	for key := range cmd.BuildsMap {
		// NOTE: Shards are unique for a given tree/run. When we migrate to
		// multiple runs this will not collide since they'll be in separate
		// trees.
		if _, ok := tree.ShardsByKey[key]; !ok {
			logging.Infof(ctx, "Run Parent %s-%s#%d: %+v\n", runNode.GetWorkUnit().Id, runNode.GetWorkUnit().Name, runNode.GetIndex(), head)

			// Generate and insert the Run Node into the WU tree.
			shardNode, err := androidapi.NewWorkUnitNode(runNode.GetWorkUnit().Id, runNode.GetWorkUnit().InvocationId, androidapi.WULayerShard, runNode, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
			if err != nil {
				return err
			}

			logging.Infof(ctx, "NEW SHARD Node %s-%s#%d: %+v\n", shardNode.GetWorkUnit().Id, shardNode.GetWorkUnit().Name, shardNode.GetIndex(), shardNode)
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

		if strings.ToLower(child.GetWorkUnit().State) != strings.ToLower(androidapi.WorkUnitCompleted.String()) {
			logging.Infof(ctx, "%s-%s: child %s-%s in state %s, allPassed set to FALSE\n", head.GetWorkUnit().Id, head.GetWorkUnit().Name, child.GetWorkUnit().Id, child.GetWorkUnit().Name, child.GetWorkUnit().State)
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
		logging.Infof(ctx, "WU %s-%s set as %s, all children passed\n", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetWorkUnit().State)
	} else {
		head.GetWorkUnit().State = common.TaskErrorState
		logging.Infof(ctx, "WU %s-%s set as %s, not all children passed\n", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetWorkUnit().State)
		head.GetWorkUnit().DebugInfo = &androidbuildinternal.DebugInfo{
			ErrorCode:    1,
			ErrorMessage: "Not all children WUs passed",
			ErrorName:    "Failed Children",
		}
		logging.Infof(ctx, "WU %s-%s completed testing in %s status\n", head.GetWorkUnit().Id, head.GetWorkUnit().Name, head.GetWorkUnit().State)
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

// sealInvocation updates the top level Work Unit and Invocation once before
// sealing them with a terminal state status.
func (cmd *AlStatusUpdateCmd) sealInvocation(ctx context.Context, tree *androidapi.WorkUnitTree, service *androidapi.Service) error {
	var err error

	// Refresh the work unit in case we are not using the most up-to-date
	// revision.
	cmd.AlStateInfo.ATPWorkUnit, err = service.WorkUnitService.Get(cmd.AlStateInfo.ATPWorkUnit.Id)
	if err != nil {
		return errors.Annotate(err, "error while refreshing ATP WorkUnit").Err()
	}

	// Set the top level WU to the same status as the tree's head.
	cmd.AlStateInfo.ATPWorkUnit.State = tree.Head.GetWorkUnit().State
	logging.Infof(ctx, "updating Work Unit %s-%s to state %s\n", cmd.AlStateInfo.ATPWorkUnit.Name, cmd.AlStateInfo.ATPWorkUnit.Id, cmd.AlStateInfo.ATPWorkUnit.State)

	_, err = service.WorkUnitService.Update(cmd.AlStateInfo.ATPWorkUnit.Id, cmd.AlStateInfo.ATPWorkUnit)
	if err != nil {
		return errors.Annotate(err, "error while updating ATP WorkUnit").Err()
	}

	// Refresh the invocation in case we are not using the most up-to-date
	// revision.
	cmd.AlStateInfo.ATPInvocation, err = service.InvocationService.Get(cmd.AlStateInfo.ATPInvocation.InvocationId)
	if err != nil {
		return errors.Annotate(err, "error while refreshing ATP Invocation").Err()
	}

	// Set the invocation to the same status as the tree's head.
	cmd.AlStateInfo.ATPInvocation.SchedulerState = tree.Head.GetWorkUnit().State
	logging.Infof(ctx, "updating Invocation %s to state %s\n", cmd.AlStateInfo.ATPInvocation.InvocationId, cmd.AlStateInfo.ATPInvocation.SchedulerState)

	_, err = service.InvocationService.Update(cmd.AlStateInfo.ATPInvocation.InvocationId, cmd.AlStateInfo.ATPInvocation)
	if err != nil {
		return errors.Annotate(err, "error while updating ATP Invocation").Err()
	}
	return nil
}

func (cmd *AlStatusUpdateCmd) closeWUTree(ctx context.Context, service *androidapi.Service) error {
	tree := cmd.getWorkUnitTree()
	if tree == nil {
		return fmt.Errorf("wu tree was removed unexpectedly")
	}

	// Update the status of each WU.
	err := updateAllNodes(ctx, service, tree.Head)
	if err != nil {
		return err
	}

	// If we generated the invocation and the starting ATP WorkUnit then close
	// out the work unit and invocation to fully seal the run.
	//
	// NOTE: ATP would normally handle this but because we are handing the
	// creation of the invocation we now in charge.
	if cmd.AlStateInfo.ATPWorkUnit != nil {
		return cmd.sealInvocation(ctx, tree, service)
	}

	return nil
}

// getALInvocationInformation fetches the required Invocation generation from
// the build request. If this information is missing then we cannot generate an
// invocation.
func (cmd *AlStatusUpdateCmd) getALInvocationInformation() (string, string, string) {
	var buildID, buildTarget, runTarget string
	for _, item := range cmd.BuildsMap {
		// Avoid nil pointer in the loop.
		if item.SuiteInfo == nil {
			break
		}

		// Exit if we've found our results
		if buildID != "" && buildTarget != "" {
			break
		}

		for _, unit := range item.SuiteInfo.GetSuiteMetadata().GetSchedulingUnits() {
			if buildID != "" && buildTarget != "" {
				break
			}

			for _, pair := range unit.GetPrimaryTarget().GetSwReq().GetKeyValues() {
				if buildID != "" && buildTarget != "" {
					break
				}

				if pair.GetKey() == "al_build_id" {
					buildID = pair.GetValue()
				} else if pair.GetKey() == "al_build_target" {
					buildTarget = pair.GetValue()
				}
			}

			runTarget = unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget()
		}
	}

	return buildID, buildTarget, runTarget
}

func (cmd *AlStatusUpdateCmd) generateInvocation(ctx context.Context, step *build.Step) error {
	if !cmd.AlStateInfo.GenerateInvocation {
		return nil
	}

	// Only generate the invocation if we have the requisite information.
	buildID, buildTarget, runTarget := cmd.getALInvocationInformation()
	isReady := buildID != "" && buildTarget != "" && runTarget != ""
	if !isReady {
		return nil
	}

	service, err := androidapi.NewAndroidBuildService(ctx, androidapi.SERVICEACCOUNT, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}
	cmd.AlStateInfo.ATP = service

	inv, err := service.InvocationService.Insert(&androidbuildinternal.Invocation{
		PrimaryBuild: &androidbuildinternal.BuildDescriptor{
			BuildId:     buildID,
			BuildTarget: buildTarget},
		Properties: []*androidbuildinternal.Property{
			{
				Name:  "cluster_id",
				Value: "skylab",
			},
			{
				Name:  "postsubmit_failure_status",
				Value: "info",
			},
			{
				Name:  "presubmit_failure_status",
				Value: "disabled",
			},
			{
				Name:  "run_target",
				Value: runTarget,
			},
		},
		Scheduler:      "CTP",
		SchedulerState: androidapi.InvocationRunning.String(),
		Trigger:        buildTarget,
	})
	if err != nil {
		return err
	}
	invocationID := inv.InvocationId
	logging.Infof(ctx, "generated invocationID: %s\n", invocationID)

	// Generate the top of the tree node to begin the ATP WU tree.
	ctp, err := androidapi.NewWorkUnitNode("", invocationID, androidapi.WULayerCTP, nil, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}

	// Add the generated artifacts to AlStateInfo so that we can close them out
	// at the end of the run.
	cmd.AlStateInfo.ATPWorkUnit = ctp.GetWorkUnit()
	cmd.AlStateInfo.ATPInvocation = inv

	parentWUID := ctp.GetWorkUnit().Id
	logging.Infof(ctx, "generated parentWUID: %s\n", parentWUID)

	// Generate the top of the tree node to begin the ATP WU tree.
	top, err := androidapi.NewWorkUnitNode(parentWUID, invocationID, androidapi.WULayerTestJob, nil, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
	if err != nil {
		return err
	}

	if tree, ok := cmd.AlStateInfo.WorkUnitTrees["test"]; ok {
		tree.Head = top
		tree.ShardsByKey = map[string]*androidapi.WorkUnitNode{}

	} else {
		return fmt.Errorf("a work unit tree was never instantiated")
	}

	logging.Infof(ctx, "TOP Node %s: %+v\n", top.GetWorkUnit().Id, top)

	// Once we have generated the invocation, CTP node, and TEST_JOB NODE then
	// we can unset this field so that we do not create another Invocation.
	cmd.AlStateInfo.GenerateInvocation = false

	return nil
}

func (cmd *AlStatusUpdateCmd) metadataArgExists(flag, value string) bool {
	if cmd.CtpRequest == nil {
		return false
	}

	emArgs := cmd.CtpRequest.GetSuiteRequest().GetTestSuite().GetExecutionMetadata().GetArgs()
	for _, arg := range emArgs {
		if arg.GetFlag() == flag && arg.GetValue() == value {
			return true
		}
	}
	return false
}

func (cmd *AlStatusUpdateCmd) updateInvocationProperties(ctx context.Context, service *androidapi.Service) error {
	var props []*androidbuildinternal.Property

	if cmd.metadataArgExists(common.InvocationDataFlag, common.CbIngestionValue) {
		cbProp := &androidbuildinternal.Property{Name: common.CbPropName, Value: "yes"}
		cbMetricsProp := &androidbuildinternal.Property{Name: common.CbMetricsPropName, Value: "yes"}
		props = append(props, cbProp, cbMetricsProp)
	}

	if cmd.BuildState != nil {
		ancestorIDs := cmd.BuildState.Build().GetAncestorIds()
		if len(ancestorIDs) > 0 {
			ancestors := make([]string, 0, len(ancestorIDs))
			for _, ancID := range ancestorIDs {
				ancestors = append(ancestors, strconv.Itoa(int(ancID)))
			}

			ancestorsProp := &androidbuildinternal.Property{
				Name:  common.AncestorsPropName,
				Value: strings.Join(ancestors, ","),
			}
			props = append(props, ancestorsProp)
		}
	}

	tree := cmd.getWorkUnitTree()
	if tree == nil || tree.Head == nil {
		logging.Infof(ctx, "No workunit tree present. Skipping update.")
	}

	invocationID := tree.Head.GetWorkUnit().InvocationId
	if invocationID == "" || len(props) == 0 {
		logging.Infof(ctx, "No invocation id or new property found. Skipping update.")
		return nil
	}

	inv, err := service.InvocationService.Get(invocationID)
	if err != nil {
		return err
	}

	inv.Properties = append(inv.Properties, props...)
	_, err = service.InvocationService.Update(invocationID, inv)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "Added properties to invocation: %v", props)
	return nil
}

// Execute executes the command.
func (cmd *AlStatusUpdateCmd) Execute(ctx context.Context) error {
	var err error
	// Skip the step completely for now if the event state is nil. If we are
	// manually creating an invocation then allow this to go through.
	// TODO (azrahman:atp): undo this once output props support is added here
	if cmd.AlStateInfo == nil {
		return nil
	} else if cmd.AlStateInfo.CurrentTestJobEvent == nil && !cmd.AlStateInfo.WorkUnitsOnly {
		return nil
	}

	step, ctx := build.StartStep(ctx, "Al Status Update")
	defer func() { step.End(err) }()
	logging.Infof(ctx, "Al Status Update")

	// ********** WORK UNIT MAINTENANCE **********
	err = cmd.generateInvocation(ctx, step)
	if err != nil {
		return err
	}

	err = cmd.initRunAndShards(ctx)
	if err != nil {
		logging.Infof(ctx, "error while initing run layer: %s", err.Error())
		if !common.IsLedRun(cmd.BuildState.Build().GetBuilder()) {
			return err
		}
	}

	if cmd.AlStateInfo.DoneTesting {
		service, err := androidapi.NewAndroidBuildService(ctx, androidapi.SERVICEACCOUNT, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
		if err != nil {
			return err
		}
		err = cmd.closeWUTree(ctx, service)
		if err != nil {
			logging.Errorf(ctx, "error while closing WU tree: %w", err)

			// Ignore update failures if being run inside of a LED run.
			if !common.IsLedRun(cmd.BuildState.Build().GetBuilder()) {
				return err
			}
		}

		err = cmd.updateInvocationProperties(ctx, service)
		if err != nil {
			logging.Errorf(ctx, "error while updating Invocation: %w", err)

			// Ignore update failures if being run inside of a LED run.
			if !common.IsLedRun(cmd.BuildState.Build().GetBuilder()) {
				return err
			}
		}
	}

	if cmd.AlStateInfo.WorkUnitsOnly {
		return nil
	}
	// ********** WORK UNIT MAINTENANCE **********

	currTestJobEvent := cmd.AlStateInfo.CurrentTestJobEvent

	// Publish to pub/sub
	common.WriteAnyObjectToStepLog(ctx, step, currTestJobEvent, "current test job event state")
	id, err := common.PublishToTestJobEventPubSub(ctx, cmd.AlStateInfo.TestJobEventPubSubClient, currTestJobEvent)
	if err != nil {
		common.WriteStringToStepLog(ctx, step, fmt.Sprintf("err while publishing with id %s: %s", id, err.Error()), "pubsub publish error")
		err = nil
	}

	updateItems := &outputprops.UpdateItems{}

	// Publish TestJobMsg
	if cmd.AlStateInfo.CurrentTestJobEvent.TestJob != nil {
		encodedTestJobMsg, err := common.EncodeAnyObj(cmd.AlStateInfo.CurrentTestJobEvent.TestJob)
		if err != nil {
			common.WriteStringToStepLog(ctx, step, fmt.Sprintf("err while encoding test job msg: %s", err.Error()), " testJobMsg encoding error")
			err = nil
		} else {
			common.WriteAnyObjectToStepLog(ctx, step, encodedTestJobMsg, "encoded testJobMsg")
			updateItems.EncodedTestJobMsg = encodedTestJobMsg
		}
	}

	// Publish TestJobEvent
	if cmd.AlStateInfo.CurrentTestJobEvent != nil {
		updateItems.TestJobEventMsgJson = cmd.AlStateInfo.CurrentTestJobEvent
		encodedTestJobEventMsg, err := common.EncodeAnyObj(cmd.AlStateInfo.CurrentTestJobEvent)
		if err != nil {
			common.WriteStringToStepLog(ctx, step, fmt.Sprintf("err while encoding test job event msg: %s", err.Error()), "testJobEventMsg encoding error")
			err = nil
		} else {
			common.WriteAnyObjectToStepLog(ctx, step, encodedTestJobEventMsg, "encoded testJobEventMsg")
			updateItems.EncodedTestJobEventMsg = encodedTestJobEventMsg
		}
	}

	// Update output props
	if updateItems.EncodedTestJobEventMsg != "" || updateItems.EncodedTestJobMsg != "" {
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
