// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package controller

import (
	"context"
	"regexp"
	"slices"
	"sync"

	"go.chromium.org/luci/common/logging"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
)

// getTopStateReq returns the top prioritized state change request along with
// the remaining requests (the new pending requests).
//
// Two rules we use here:
//  1. The request with higher priority wins.
//  2. When there are multiple requests have the same priority, the one comes
//     later wins.
//
// Requests with priority 0 are dropped from the pending queue because they will
// never be selected over new incoming requests, which either have
// 1) a higher priority, or
// 2) a later arrival time.
func getTopStateReq(ctx context.Context, user string, askingState ufspb.State, originalRecord *ufspb.StateRecord) (*ufspb.StateRequest, []*ufspb.StateRequest) {
	resource := originalRecord.GetResourceName()
	logging.Debugf(ctx, "State change req incoming for %q: user %q, state %s", resource, user, askingState)
	logging.Debugf(ctx, "Current State record for %q: user %q, state %s, pending reqs: %v", resource, originalRecord.GetUser(), originalRecord.GetState(), originalRecord.GetPendingRequests())
	// Merge (dedup by user) the incoming req with the existing pending reqs.
	// Put incoming req always as the first one, so after (stable) sorting, it
	// will still be the first one in its group of priority.
	// Don't use map to merge. It wipes the order.
	allReqs := []*stateReqWithPriority{buildStateReqWithPriority(ctx, user, askingState)}

	if u := originalRecord.GetUser(); !isTheSameUser(ctx, u, user) {
		allReqs = append(allReqs, buildStateReqWithPriority(ctx, u, originalRecord.GetState()))
	}

	for _, r := range originalRecord.GetPendingRequests() {
		if isTheSameUser(ctx, r.GetUser(), user) {
			continue
		}
		allReqs = append(allReqs, buildStateReqWithPriority(ctx, r.GetUser(), r.GetState()))
	}

	// We sort the slice by priority in DESC order, i.e. the higher priority is
	// before a lower priority in the result slice.
	slices.SortStableFunc(allReqs, byPriorityDESC)

	// No priority needed in the returned values.
	result := make([]*ufspb.StateRequest, len(allReqs))
	for i, r := range allReqs {
		// The first element will be the new state to return, so we will keep it
		// even if its priority is 0.
		if i > 0 && r.priority == 0 {
			continue
		}
		result[i] = &ufspb.StateRequest{User: allReqs[i].user, State: allReqs[i].state}
	}
	logging.Debugf(ctx, "The state req prioritized result for %q: %v", originalRecord.GetResourceName(), result)
	return result[0], result[1:]
}

// buildStateReqWithPriority builds the struct of buildStateReqWithPriority.
func buildStateReqWithPriority(ctx context.Context, user string, state ufspb.State) *stateReqWithPriority {
	return &stateReqWithPriority{
		user:     user,
		state:    state,
		priority: getStateReqPriority(ctx, user, state),
	}
}

// A struct to store the state change request with its priority.
type stateReqWithPriority struct {
	user     string
	state    ufspb.State
	priority uint32
}

func byPriorityDESC(a, b *stateReqWithPriority) int {
	if a.priority > b.priority {
		return -1 // negative integer means a shows up before b.
	}
	if a.priority < b.priority {
		return 1
	}
	return 0
}

// isTheSameUser checks if two users are same.
//
// We treat all human users as the same user, so it will be true for two human
// user though they may be different names.
// For services, we just check if the user names (i.e. service accounts) are
// literally the same.
func isTheSameUser(ctx context.Context, u1, u2 string) bool {
	if isHuman(ctx, u1) && isHuman(ctx, u2) {
		return true
	}
	return u1 == u2
}

// getStateReqPriority gets the priority based the state change request user and
// state value based on the UFS configuration.
//
// We return 0 (the lowest priority) as the default or on any errors, to prevent
// any unexpected state overwriting.
func getStateReqPriority(ctx context.Context, user string, state ufspb.State) uint32 {
	stateChangePriorityCfgs.mu.RLock()
	defer stateChangePriorityCfgs.mu.RUnlock()

	for _, cfg := range stateChangePriorityCfgs.data {
		if !cfg.userRegexp.MatchString(user) {
			continue
		}
		if pri, ok := cfg.priorityMap[state]; ok {
			return pri
		}
	}
	logging.Warningf(ctx, "Cannot find state priority for %q %s (return the lowest priority instead)", user, state)
	return 0
}

type priorityCfg struct {
	userRegexp  *regexp.Regexp
	priorityMap map[ufspb.State]uint32
}

var stateChangePriorityCfgs struct {
	data []priorityCfg
	mu   sync.RWMutex
}

// ParseStateChangePriorityCfg parse loaded state change priority configuration.
func ParseStateChangePriorityCfg(cfg *config.Config) {
	priorities := cfg.GetStateChangeReqPriority().GetPriorities()
	res := make([]priorityCfg, len(priorities))
	for i, c := range priorities {
		res[i].userRegexp = regexp.MustCompile(c.GetUserRegexp())
		states := c.GetIncludedStates()
		if len(states) == 0 {
			states = allUFSStates()
		}
		states = removeSubsetStates(c.GetExcludedStates(), states)

		res[i].priorityMap = make(map[ufspb.State]uint32)
		for _, s := range states {
			res[i].priorityMap[s] = c.GetPriority()
		}
	}

	stateChangePriorityCfgs.mu.Lock()
	stateChangePriorityCfgs.data = res
	stateChangePriorityCfgs.mu.Unlock()
}

// allUFSStates returns all UFS state enums.
func allUFSStates() []ufspb.State {
	res := []ufspb.State{}
	for k := range ufspb.State_name {
		res = append(res, ufspb.State(k))
	}
	return res
}

// removeSubsetStates returns the states in the main set but not subset.
func removeSubsetStates(subset, mainSet []ufspb.State) []ufspb.State {
	subsetMap := make(map[ufspb.State]struct{}, len(subset))
	for _, s := range subset {
		subsetMap[s] = struct{}{}
	}
	res := []ufspb.State{}
	for _, k := range mainSet {
		if _, ok := subsetMap[k]; !ok {
			res = append(res, k)
		}
	}
	return res
}
