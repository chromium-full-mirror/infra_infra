// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	"go.chromium.org/luci/common/logging"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	"go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
)

func deleteNonExistingRacks(ctx context.Context, racks []*ufspb.Rack, pageSize int) (*datastore.OpResults, error) {
	resMap := make(map[string]bool)
	for _, r := range racks {
		resMap[r.GetName()] = true
	}
	rackRes, err := registration.GetAllRacks(ctx)
	if err != nil {
		return nil, err
	}
	var toDelete []string
	for _, sr := range rackRes.Passed() {
		s := sr.Data.(*ufspb.Rack)
		if _, ok := resMap[s.GetName()]; !ok {
			toDelete = append(toDelete, s.GetName())
		}
	}
	logging.Infof(ctx, "Deleting %d non-existing racks", len(toDelete))
	return deleteByPage(ctx, toDelete, pageSize, registration.DeleteRacks), nil
}

func deleteNonExistingKVMs(ctx context.Context, kvms []*ufspb.KVM, pageSize int) (*datastore.OpResults, error) {
	resMap := make(map[string]bool)
	for _, r := range kvms {
		resMap[r.GetName()] = true
	}
	resp, err := registration.GetAllKVMs(ctx)
	if err != nil {
		return nil, err
	}
	var toDelete []string
	for _, sr := range resp.Passed() {
		s := sr.Data.(*ufspb.KVM)
		if _, ok := resMap[s.GetName()]; !ok {
			toDelete = append(toDelete, s.GetName())
		}
	}
	logging.Infof(ctx, "Deleting %d non-existing kvms", len(toDelete))
	allRes := *deleteByPage(ctx, toDelete, pageSize, registration.DeleteKVMs)
	logging.Infof(ctx, "Deleting %d non-existing kvm-related dhcps", len(toDelete))
	allRes = append(allRes, *deleteByPage(ctx, toDelete, pageSize, configuration.DeleteDHCPs)...)
	return &allRes, nil
}

func deleteNonExistingSwitches(ctx context.Context, switches []*ufspb.Switch, pageSize int) (*datastore.OpResults, error) {
	resMap := make(map[string]bool)
	for _, r := range switches {
		resMap[r.GetName()] = true
	}
	resp, err := registration.GetAllSwitches(ctx)
	if err != nil {
		return nil, err
	}
	var toDelete []string
	for _, sr := range resp.Passed() {
		s := sr.Data.(*ufspb.Switch)
		if _, ok := resMap[s.GetName()]; !ok {
			toDelete = append(toDelete, s.GetName())
		}
	}
	logging.Infof(ctx, "Deleting %d non-existing switches", len(toDelete))
	return deleteByPage(ctx, toDelete, pageSize, registration.DeleteSwitches), nil
}
