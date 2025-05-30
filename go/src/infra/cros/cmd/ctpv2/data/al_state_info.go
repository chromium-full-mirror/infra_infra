// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// AlStateInfo captures the state info for Al runs
type AlStateInfo struct {
	IsAlRun     bool
	DoneTesting bool
	TreeClosed  bool

	// Input test job that should not change during execution and only be used as reader
	InputTestJob *common.TestJobMessage

	// Current state that will be changed by cmds during execution
	CurrentTestJob           *common.TestJobMessage
	CurrentTestJobEvent      *common.TestJobEventMessage
	TestJobEventPubSubClient *pubsub.Client

	// WorkUnitTrees is a map that points to the head of each ATP request's
	// beginning node.
	//
	// NOTE: For the time being this map will only contain one tree until we
	// begin to support multiple ATP requests per CTP build.
	WorkUnitTrees map[string]*androidapi.WorkUnitTree

	// ATP is the instantiated ATP service API that we will reuse throughout the
	// build.
	ATP *androidapi.Service

	BuildID string

	// Fields for invocation generation logic.

	GenerateInvocation bool
	WorkUnitsOnly      bool
	ATPWorkUnit        *androidbuildinternal.WorkUnit
	ATPInvocation      *androidbuildinternal.Invocation

	Top *androidapi.WorkUnitNode
}

// GetWorkUnitTree fetches the ATP work unit tree if one exists and is being
// tracked.
func (a *AlStateInfo) GetWorkUnitTree() *androidapi.WorkUnitTree {
	if a == nil {
		return nil
	}

	if a.WorkUnitTrees == nil {
		return nil
	}

	if tree, ok := a.WorkUnitTrees["test"]; ok {
		return tree
	}

	return nil
}

// updateAllNodes recursively iterates through the tree updating the status of
// each node from the bottom layers upward.
//
// NOTE: This leverages the fact that we set the state for the attempt nodes
// inside the schedule_tasks command.
func updateAllNodes(ctx context.Context, service *androidapi.Service, head *androidapi.WorkUnitNode, errorMsg, errorName string) error {
	children := head.GetChildren()
	// Once we've reached the attempt layer update the WU and return.
	if children.Len() == 0 {
		if strings.EqualFold(head.GetWorkUnit().State, androidapi.WorkUnitRunning.String()) {
			head.GetWorkUnit().State = androidapi.WorkUnitError.String()
			head.GetWorkUnit().DebugInfo = &androidbuildinternal.DebugInfo{
				ErrorCode:    1,
				ErrorMessage: errorMsg,
				ErrorName:    errorName,
			}

		}

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
		err := updateAllNodes(ctx, service, child, errorMsg, errorName)
		if err != nil {
			return err
		}

		if !strings.EqualFold(child.GetWorkUnit().State, androidapi.WorkUnitCompleted.String()) {
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
			ErrorMessage: "Not all children work units passed",
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
func (a *AlStateInfo) sealInvocation(ctx context.Context, tree *androidapi.WorkUnitTree, service *androidapi.Service) error {
	if a.ATPWorkUnit == nil {
		return nil
	}

	var err error

	// Refresh the work unit in case we are not using the most up-to-date
	// revision.
	a.ATPWorkUnit, err = service.WorkUnitService.Get(a.ATPWorkUnit.Id)
	if err != nil {
		return errors.Annotate(err, "error while refreshing ATP WorkUnit").Err()
	}

	// Set the top level WU to the same status as the tree's head.
	a.ATPWorkUnit.State = tree.Head.GetWorkUnit().State
	logging.Infof(ctx, "updating Work Unit %s-%s to state %s\n", a.ATPWorkUnit.Name, a.ATPWorkUnit.Id, a.ATPWorkUnit.State)

	_, err = service.WorkUnitService.Update(a.ATPWorkUnit.Id, a.ATPWorkUnit)
	if err != nil {
		return errors.Annotate(err, "error while updating ATP WorkUnit").Err()
	}

	// Refresh the invocation in case we are not using the most up-to-date
	// revision.
	a.ATPInvocation, err = service.InvocationService.Get(ctx, a.ATPInvocation.InvocationId)
	if err != nil {
		return errors.Annotate(err, "error while refreshing ATP Invocation").Err()
	}

	// Set the invocation to the same status as the tree's head.
	a.ATPInvocation.SchedulerState = tree.Head.GetWorkUnit().State
	logging.Infof(ctx, "updating Invocation %s to state %s\n", a.ATPInvocation.InvocationId, a.ATPInvocation.SchedulerState)

	_, err = service.InvocationService.Update(a.ATPInvocation.InvocationId, a.ATPInvocation)
	if err != nil {
		return errors.Annotate(err, "error while updating ATP Invocation").Err()
	}
	return nil
}

func (a *AlStateInfo) CloseWUTree(ctx context.Context, service *androidapi.Service, errorMsg, errorName string) error {
	tree := a.GetWorkUnitTree()
	if tree == nil {
		return fmt.Errorf("work unit tree was removed unexpectedly")
	}

	if tree.Head == nil {
		return fmt.Errorf("work unit tree has a nil head")
	}

	invocationID := tree.Head.GetWorkUnit().InvocationId
	inv, err := service.InvocationService.Get(ctx, invocationID)
	if err != nil {
		return err
	}

	if common.InvocationSealed(inv) {
		logging.Infof(ctx, "Cannot close tree on sealed invocation %s. Invocation State: %s", invocationID, inv.SchedulerState)
		return nil
	}

	logging.Infof(ctx, "Got work tree head %s, updating node statuses", tree.Head.GetWorkUnit().Id)
	// Update the status of each WU.
	wuErr := updateAllNodes(ctx, service, tree.Head, errorMsg, errorName)
	if wuErr != nil {
		logging.Warningf(ctx, "Failed updating nodes: %v", wuErr)
	}

	// If we generated the invocation and the starting ATP WorkUnit then close
	// out the work unit and invocation to fully seal the run.
	//
	// NOTE: ATP would normally handle this but because we are handling the
	// creation of the invocation we are now in charge.
	var sealInvErr error
	if a.ATPWorkUnit != nil {
		sealInvErr = a.sealInvocation(ctx, tree, service)
		if sealInvErr != nil {
			logging.Warningf(ctx, "Failed to seal invocation: %v", sealInvErr)
		}
	}

	return errors.Join(wuErr, sealInvErr)
}
