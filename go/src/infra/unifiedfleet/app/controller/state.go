// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"strings"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func deleteNonExistingStates(ctx context.Context, states []*ufspb.StateRecord, pageSize int) (*ufsds.OpResults, error) {
	resMap := make(map[string]bool)
	for _, r := range states {
		resMap[r.GetResourceName()] = true
	}
	resp, err := state.GetAllStates(ctx)
	if err != nil {
		return nil, err
	}
	var toDelete []string
	for _, sr := range resp.Passed() {
		s := sr.Data.(*ufspb.StateRecord)
		// Skip deleting os hosts' state
		if strings.HasPrefix(s.GetResourceName(), "hosts/chromeos") {
			continue
		}
		if _, ok := resMap[s.GetResourceName()]; !ok {
			toDelete = append(toDelete, s.GetResourceName())
		}
	}
	logging.Infof(ctx, "Deleting %d non-existing states", len(toDelete))
	return deleteByPage(ctx, toDelete, pageSize, state.DeleteStates), nil
}

// UpdateState updates state record for a resource.
func UpdateState(ctx context.Context, stateRecord *ufspb.StateRecord) (*ufspb.StateRecord, error) {
	f := func(ctx context.Context) error {
		// To update the MachineLSE state when a state record is being updated.
		// TODO(eshwarn): Remove this code once this is in drone(https://chromium-review.googlesource.com/c/infra/infra/+/2739908)
		name := util.RemovePrefix(stateRecord.GetResourceName())
		lse, err := inventory.GetMachineLSE(ctx, name)
		if err != nil {
			logging.Errorf(ctx, "Failed to update ResourceState: GetMachineLSE %s failed: %s", name, err)
		} else {
			if err := util.CheckPermission(ctx, util.ConfigurationsUpdate, lse.GetRealm()); err != nil {
				return err
			}

			// Copy for logging
			oldMachinelseCopy := proto.Clone(lse).(*ufspb.MachineLSE)
			lse.ResourceState = stateRecord.GetState()
			if _, err := inventory.BatchUpdateMachineLSEs(ctx, []*ufspb.MachineLSE{lse}); err != nil {
				logging.Errorf(ctx, "Failed to update ResourceState: BatchUpdateMachineLSEs %s : %s", lse.GetName(), err)
			} else {
				hclse := getHostHistoryClient(lse)
				hclse.LogMachineLSEChanges(oldMachinelseCopy, lse)
				hclse.SaveChangeEvents(ctx)
			}
		}
		hc := getStateRecordHistoryClient(stateRecord)
		if err := hc.stUdt.updateStateHelper(ctx, stateRecord.GetState()); err != nil {
			return err
		}
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "UpdateState - failed to update %s", stateRecord.GetResourceName()).Err()
	}
	return stateRecord, nil
}

// GetState returns state record for a resource.
func GetState(ctx context.Context, resourceName string) (*ufspb.StateRecord, error) {
	// First try to find in os namespace, if not find in default namespace
	// TODO(eshwarn): Remove this - once all state data is migrated to os namespace
	newCtx, err := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	if err != nil {
		logging.Debugf(ctx, "GetState - Failed to set os namespace in context", err)
		return state.GetStateRecord(ctx, resourceName)
	}
	record, err := state.GetStateRecord(newCtx, resourceName)
	if err == nil {
		return record, err
	}

	// default namespace
	newCtx, err = util.SetupDatastoreNamespace(ctx, "")
	if err != nil {
		logging.Debugf(ctx, "GetState - Failed to set default namespace in context", err)
		return state.GetStateRecord(ctx, resourceName)
	}

	return state.GetStateRecord(newCtx, resourceName)
}

func getStateRecordHistoryClient(sr *ufspb.StateRecord) *HistoryClient {
	return &HistoryClient{
		stUdt: &stateUpdater{
			ResourceName: sr.GetResourceName(),
		},
	}
}
