// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
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

// decideMachineLSEState decides the final state based on current state and
// other factors.
func (su *stateUpdater) decideMachineLSEState(ctx context.Context, machineLSE *ufspb.MachineLSE, originalState ufspb.State, byForce bool) error {
	oldRecord, _ := state.GetStateRecord(ctx, su.ResourceName)
	newRecord, err := resolveNewState(ctx, machineLSE, oldRecord, byForce)
	if err != nil {
		return errors.Annotate(err, "decide machineLSE state").Err()
	}
	machineLSE.ResourceState = newRecord.GetState()
	if !needToUpdateStateRecord(oldRecord, newRecord) {
		return nil
	}
	state.DeleteStates(ctx, []string{su.ResourceName})
	newRecord.ResourceName = su.ResourceName
	newRecord.User = util.CurrentUser(ctx)
	if _, err := state.BatchUpdateStates(ctx, []*ufspb.StateRecord{newRecord}); err != nil {
		return err
	}
	su.logChanges(LogStateChanges(oldRecord, newRecord))
	return nil
}

// resolveNewState resolves the machineLSE state based on user requests and other
// factors.
// See go/ufs-browser-state-sync for the detail algorithm.
func resolveNewState(ctx context.Context, machineLSE *ufspb.MachineLSE, oldRecord *ufspb.StateRecord, byForce bool) (*ufspb.StateRecord, error) {
	askingState := machineLSE.GetResourceState()
	// Currently only apply this rules to the Browser fleet.
	if util.GetNamespaceFromCtx(ctx) != util.BrowserNamespace {
		return &ufspb.StateRecord{State: askingState}, nil
	}
	user := util.CurrentUser(ctx)
	human, err := isHuman(user)
	if err != nil {
		return nil, errors.Annotate(err, "resolve machine LSE state").Err()
	}
	if human {
		return resolveStateForHuman(ctx, oldRecord, askingState, byForce), nil
	}
	if byForce {
		return nil, errors.Reason("resolve machine LSE state: non-human user %q cannot force a state change", user).Err()
	}
	if askingState != ufspb.State_STATE_READY && askingState != ufspb.State_STATE_DISABLED {
		return nil, errors.Reason("resolve machine LSE state: non-human user %q cannot change state to %q, only READY/DISABLED is allowed", user, askingState).Err()
	}
	return resolveStateForNonHuman(ctx, oldRecord, askingState), nil
}

// resolveStateForHuman resolves the MachineLSE state for human users.
func resolveStateForHuman(ctx context.Context, originalRecord *ufspb.StateRecord, askingState ufspb.State, byForce bool) *ufspb.StateRecord {
	user := util.CurrentUser(ctx)
	if byForce {
		if askingState == ufspb.State_STATE_READY || askingState == ufspb.State_STATE_SERVING {
			return &ufspb.StateRecord{State: askingState}
		}
		return &ufspb.StateRecord{State: askingState, ConcurrentRequesters: map[string]string{user: askingState.String()}}
	}
	requesters := map[string]string{}
	for k, v := range originalRecord.GetConcurrentRequesters() {
		requesters[k] = v
	}

	// Set to non-ready|serving state.
	// In this case, we sign up the caller name and set the target state to the
	// asking state as the human user requests have higher priority.
	if askingState != ufspb.State_STATE_READY && askingState != ufspb.State_STATE_SERVING {
		// The value can be a reason. But for now we only use the asking state as a placeholder.
		requesters[user] = askingState.String()
		return &ufspb.StateRecord{State: askingState, ConcurrentRequesters: requesters}
	}

	// Set to ready|serving state.
	// Sign off the caller and check if there are non-human users remained to
	// decide the final state.
	for r := range requesters {
		// All human users are regarded as one user, so one can
		// revert changes made by others.
		// Suppose there's no case of human users from different domains changes the
		// state of the same machineLSE. Otherwise, we don't know what's the new
		// state should be.
		if human, _ := isHuman(r); human {
			delete(requesters, r)
		}
	}
	if len(requesters) == 0 {
		return &ufspb.StateRecord{State: askingState, ConcurrentRequesters: map[string]string{}}
	}
	// There are non-human requesters, which can only set the state to DISABLED.
	return &ufspb.StateRecord{State: ufspb.State_STATE_DISABLED, ConcurrentRequesters: requesters}
}

// Suppose all human users are from predefined domain list.
// Human users can force a state update, while a robot user can't.
// The @example.com and @ter.com domain is for tests.
var humanUserDoamins = map[string]bool{
	"google.com":  true,
	"example.com": true,
	"ter.com":     true,
}

func isHuman(email string) (bool, error) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid email format (missing '@'): %q", email)
	}
	domain := parts[1]
	return humanUserDoamins[domain], nil
}

// resolveStateForHuman resolves the MachineLSE state for non-human users.
func resolveStateForNonHuman(ctx context.Context, originalRecord *ufspb.StateRecord, askingState ufspb.State) *ufspb.StateRecord {
	// for non-human users, the asking state can only be either READY or DISABLED.
	user := util.CurrentUser(ctx)
	requesters := map[string]string{}
	for k, v := range originalRecord.GetConcurrentRequesters() {
		requesters[k] = v
	}

	if askingState == ufspb.State_STATE_READY {
		delete(requesters, user)
		if len(requesters) == 0 {
			return &ufspb.StateRecord{State: askingState}
		}
		return &ufspb.StateRecord{State: originalRecord.GetState(), ConcurrentRequesters: requesters}
	}
	// askingState is DISABLED
	requesters[user] = askingState.String()
	if len(requesters) == 1 { // the only caller
		return &ufspb.StateRecord{State: askingState, ConcurrentRequesters: requesters}
	}
	// Due to lower priority, non-human user requests won't change the state if
	// there's human users changed the state.
	return &ufspb.StateRecord{State: originalRecord.GetState(), ConcurrentRequesters: requesters}
}

// needToUpdateStateRecord checks two StateRecord for changes matter which need
// to write back to datastore.
func needToUpdateStateRecord(old, new *ufspb.StateRecord) bool {
	if old.GetState() != new.GetState() {
		return true
	}
	oldRequesters := old.GetConcurrentRequesters()
	newRequesters := new.GetConcurrentRequesters()
	if oldRequesters == nil && newRequesters == nil {
		return false
	}
	if oldRequesters == nil {
		oldRequesters = map[string]string{}
	}
	if newRequesters == nil {
		newRequesters = map[string]string{}
	}
	if len(oldRequesters) != len(newRequesters) {
		return true
	}
	for k := range oldRequesters {
		if _, ok := newRequesters[k]; !ok {
			return true
		}
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
