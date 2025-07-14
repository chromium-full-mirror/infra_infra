// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"slices"
	"strings"

	"go.chromium.org/luci/common/errors"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

type stateUpdater struct {
	ResourceName string
	Changes      []*ufspb.ChangeEvent
	Msgs         []*history.SnapshotMsgEntity
}

func (su *stateUpdater) logChanges(changes []*ufspb.ChangeEvent, msg *history.SnapshotMsgEntity) {
	su.Changes = append(su.Changes, changes...)
	if msg != nil {
		su.Msgs = append(su.Msgs, msg)
	}
}

// Delete a state record
//
// Can be used in a transaction
func (su *stateUpdater) deleteStateHelper(ctx context.Context) error {
	old, _ := state.GetStateRecord(ctx, su.ResourceName)
	state.DeleteStates(ctx, []string{su.ResourceName})
	su.logChanges(LogStateChanges(old, nil))
	return nil
}

// updateMachineLSEState updates the final state based on current state and
// other factors.
func (su *stateUpdater) updateMachineLSEState(ctx context.Context, machineLSE *ufspb.MachineLSE, originalState ufspb.State) error {
	oldRecord, _ := state.GetStateRecord(ctx, su.ResourceName)
	newRecord, err := resolveNewState(ctx, machineLSE, oldRecord)
	if err != nil {
		return errors.Fmt("decide machineLSE state: %w", err)
	}
	machineLSE.ResourceState = newRecord.GetState()
	if !needToUpdateStateRecord(oldRecord, newRecord) {
		return nil
	}
	state.DeleteStates(ctx, []string{su.ResourceName})
	// As we may use a queued state change request, the newRecord.User may not be
	// the current request user.
	newRecord.ResourceName = su.ResourceName
	if _, err := state.BatchUpdateStates(ctx, []*ufspb.StateRecord{newRecord}); err != nil {
		return err
	}
	su.logChanges(LogStateChanges(oldRecord, newRecord))
	return nil
}

// resolveNewState resolves the machineLSE state based on user requests and
// other factors.
// See go/ufs-browser-state-sync for the detail algorithm.
func resolveNewState(ctx context.Context, machineLSE *ufspb.MachineLSE, oldRecord *ufspb.StateRecord) (*ufspb.StateRecord, error) {
	user := util.CurrentUser(ctx)
	askingState := machineLSE.GetResourceState()
	// Currently only apply this rules to the Browser fleet.
	if util.GetNamespaceFromCtx(ctx) != util.BrowserNamespace {
		return &ufspb.StateRecord{User: user, State: askingState}, nil
	}
	if isHuman(ctx, user) {
		// Reset PendingRequests here to respect the human decision.
		return &ufspb.StateRecord{User: user, State: askingState}, nil
	}
	return resolveStateForServices(ctx, user, oldRecord, askingState), nil
}

func isHuman(ctx context.Context, email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	return parts[1] == "google.com"
}

// resolveStateForServices resolves the MachineLSE state for non-human users.
func resolveStateForServices(ctx context.Context, user string, originalRecord *ufspb.StateRecord, askingState ufspb.State) *ufspb.StateRecord {
	req, remainingReqs := getTopStateReq(ctx, user, askingState, originalRecord)

	return &ufspb.StateRecord{
		State:           req.State,
		User:            req.User,
		PendingRequests: remainingReqs,
	}
}

// needToUpdateStateRecord checks two StateRecord for changes matter which need
// to write back to datastore.
func needToUpdateStateRecord(old, new *ufspb.StateRecord) bool {
	switch {
	case old.GetState() != new.GetState():
		return true
	case old.GetUser() != new.GetUser():
		return true
	case !slices.Equal(old.GetPendingRequests(), new.GetPendingRequests()):
		return true
	}
	return false
}

func (su *stateUpdater) updateStateHelper(ctx context.Context, newS ufspb.State) error {
	old, _ := state.GetStateRecord(ctx, su.ResourceName)
	if old.GetState() == newS {
		return nil
	}
	state.DeleteStates(ctx, []string{su.ResourceName})
	newRecord := &ufspb.StateRecord{
		State:        newS,
		ResourceName: su.ResourceName,
		User:         util.CurrentUser(ctx),
	}
	if _, err := state.BatchUpdateStates(ctx, []*ufspb.StateRecord{newRecord}); err != nil {
		return err
	}
	su.logChanges(LogStateChanges(old, newRecord))
	return nil
}

func (su *stateUpdater) addLseStateHelper(ctx context.Context, lse *ufspb.MachineLSE, machine *ufspb.Machine) error {
	stateRecords := make([]*ufspb.StateRecord, 0)
	rn := util.AddPrefix(util.MachineCollection, machine.GetName())
	s := &ufspb.StateRecord{
		State:        machine.GetResourceState(),
		ResourceName: rn,
		User:         util.CurrentUser(ctx),
	}
	oldS, _ := state.GetStateRecord(ctx, rn)
	stateRecords = append(stateRecords, s)
	su.logChanges(LogStateChanges(oldS, s))
	for _, vm := range lse.GetChromeBrowserMachineLse().GetVms() {
		rn := util.AddPrefix(util.VMCollection, vm.GetName())
		s := &ufspb.StateRecord{
			State:        vm.GetResourceState(),
			ResourceName: rn,
			User:         util.CurrentUser(ctx),
		}
		oldS, _ := state.GetStateRecord(ctx, rn)
		stateRecords = append(stateRecords, s)
		su.logChanges(LogStateChanges(oldS, s))
	}
	newS := &ufspb.StateRecord{
		State:        lse.GetResourceState(),
		ResourceName: util.AddPrefix(util.HostCollection, lse.GetName()),
		User:         util.CurrentUser(ctx),
	}
	stateRecords = append(stateRecords, newS)
	su.logChanges(LogStateChanges(nil, newS))
	if _, err := state.BatchUpdateStates(ctx, stateRecords); err != nil {
		return err
	}
	return nil
}

func (su *stateUpdater) deleteLseStateHelper(ctx context.Context, lse *ufspb.MachineLSE, machine *ufspb.Machine) error {
	stateRecords := make([]*ufspb.StateRecord, 0)
	su.ResourceName = util.AddPrefix(util.HostCollection, lse.GetName())
	// Update attached machines' state to registered
	if machine != nil {
		rn := util.AddPrefix(util.MachineCollection, machine.GetName())
		s := &ufspb.StateRecord{
			State:        machine.GetResourceState(),
			ResourceName: rn,
			User:         util.CurrentUser(ctx),
		}
		oldS, _ := state.GetStateRecord(ctx, rn)
		stateRecords = append(stateRecords, s)
		su.logChanges(LogStateChanges(oldS, s))
		if _, err := state.BatchUpdateStates(ctx, stateRecords); err != nil {
			return err
		}
	}

	// Delete host & vm states
	toDeleteResources := make([]string, 0)
	for _, m := range lse.GetChromeBrowserMachineLse().GetVms() {
		r := util.AddPrefix(util.VMCollection, m.GetName())
		toDeleteResources = append(toDeleteResources, r)
		oldS, _ := state.GetStateRecord(ctx, r)
		su.logChanges(LogStateChanges(oldS, nil))
	}
	toDeleteResources = append(toDeleteResources, su.ResourceName)
	oldS, _ := state.GetStateRecord(ctx, su.ResourceName)
	su.logChanges(LogStateChanges(oldS, nil))
	state.DeleteStates(ctx, toDeleteResources)
	return nil
}

func (su *stateUpdater) addRackStateHelper(ctx context.Context, rack *ufspb.Rack) error {
	stateRecords := make([]*ufspb.StateRecord, 0)
	for _, m := range rack.GetChromeBrowserRack().GetSwitchObjects() {
		s := &ufspb.StateRecord{
			State:        m.GetResourceState(),
			ResourceName: util.AddPrefix(util.SwitchCollection, m.Name),
			User:         util.CurrentUser(ctx),
		}
		stateRecords = append(stateRecords, s)
		su.logChanges(LogStateChanges(nil, s))
	}
	for _, m := range rack.GetChromeBrowserRack().GetKvmObjects() {
		s := &ufspb.StateRecord{
			State:        m.GetResourceState(),
			ResourceName: util.AddPrefix(util.KVMCollection, m.GetName()),
			User:         util.CurrentUser(ctx),
		}
		stateRecords = append(stateRecords, s)
		su.logChanges(LogStateChanges(nil, s))
	}
	for _, m := range rack.GetChromeBrowserRack().GetRpmObjects() {
		s := &ufspb.StateRecord{
			State:        m.GetResourceState(),
			ResourceName: util.AddPrefix(util.RPMCollection, m.GetName()),
			User:         util.CurrentUser(ctx),
		}
		stateRecords = append(stateRecords, s)
		su.logChanges(LogStateChanges(nil, s))
	}
	newS := &ufspb.StateRecord{
		State:        rack.GetResourceState(),
		ResourceName: util.AddPrefix(util.RackCollection, rack.GetName()),
		User:         util.CurrentUser(ctx),
	}
	stateRecords = append(stateRecords, newS)
	su.logChanges(LogStateChanges(nil, newS))
	if _, err := state.BatchUpdateStates(ctx, stateRecords); err != nil {
		return err
	}
	return nil
}
