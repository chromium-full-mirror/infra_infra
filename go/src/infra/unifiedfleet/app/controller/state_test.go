// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/state"
	"infra/unifiedfleet/app/util"
)

func TestGetState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	bm1, _ := state.UpdateStateRecord(ctx, &ufspb.StateRecord{
		ResourceName: "machine/browser-machine-1",
	})
	state.UpdateStateRecord(ctx, &ufspb.StateRecord{
		ResourceName: "machine/os-machine-1",
		State:        ufspb.State_STATE_REGISTERED,
	})
	os2Registered, _ := state.UpdateStateRecord(ctx, &ufspb.StateRecord{
		ResourceName: "machine/os-machine-2",
		State:        ufspb.State_STATE_REGISTERED,
	})
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	os1Serving, _ := state.UpdateStateRecord(osCtx, &ufspb.StateRecord{
		ResourceName: "machine/os-machine-1",
		State:        ufspb.State_STATE_SERVING,
	})
	os3Serving, _ := state.UpdateStateRecord(osCtx, &ufspb.StateRecord{
		ResourceName: "machine/os-machine-3",
		State:        ufspb.State_STATE_SERVING,
	})
	ftt.Run("GetState", t, func(t *ftt.Test) {
		t.Run("GetState for a browser machine with default namespace context", func(t *ftt.Test) {
			res, err := GetState(ctx, "machine/browser-machine-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(bm1))
		})

		t.Run("GetState for a os machine with default namespace context", func(t *ftt.Test) {
			res, err := GetState(ctx, "machine/os-machine-1")
			assert.Loosely(t, err, should.BeNil)
			// TODO(eshwarn): change this check when fall back read is removed
			assert.Loosely(t, res, should.Match(os1Serving))
			res, err = GetState(ctx, "machine/os-machine-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(os2Registered))
			res, err = GetState(ctx, "machine/os-machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(os3Serving))
		})

		t.Run("GetState for a os machine with os namespace context", func(t *ftt.Test) {
			res, err := GetState(osCtx, "machine/os-machine-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(os1Serving))
			res, err = GetState(osCtx, "machine/os-machine-2")
			assert.Loosely(t, err, should.BeNil)
			// TODO(eshwarn): change this check when fall back read is removed
			assert.Loosely(t, res, should.Match(os2Registered))
			res, err = GetState(osCtx, "machine/os-machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(os3Serving))
		})
	})
}

func TestUpdateState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	// os namespace context
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	ftt.Run("UpdateState", t, func(t *ftt.Test) {
		t.Run("UpdateState for machine only in os namespace", func(t *ftt.Test) {
			// creating in os namespace
			state.UpdateStateRecord(osCtx, &ufspb.StateRecord{
				ResourceName: "machines/os-machine-1",
				State:        ufspb.State_STATE_SERVING,
			})

			sr := &ufspb.StateRecord{
				ResourceName: "machines/os-machine-1",
				State:        ufspb.State_STATE_NEEDS_REPAIR,
			}
			res, err := UpdateState(osCtx, sr)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(sr))

			res, err = state.GetStateRecord(osCtx, "machines/os-machine-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetResourceName(), should.Equal("machines/os-machine-1"))
			assert.Loosely(t, res.GetState(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR))

			changes, err := history.QueryChangesByPropertyName(osCtx, "name", "states/machines/os-machine-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetName(), should.Equal("states/machines/os-machine-1"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR.String()))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", "states/machines/os-machine-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("UpdateState for non-existing machine in os namespace", func(t *ftt.Test) {
			sr := &ufspb.StateRecord{
				ResourceName: "machines/os-machine-2",
				State:        ufspb.State_STATE_NEEDS_REPAIR,
			}
			res, err := UpdateState(osCtx, sr)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(sr))

			res, err = state.GetStateRecord(osCtx, "machines/os-machine-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetResourceName(), should.Equal("machines/os-machine-2"))
			assert.Loosely(t, res.GetState(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR))

			changes, err := history.QueryChangesByPropertyName(osCtx, "name", "states/machines/os-machine-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetName(), should.Equal("states/machines/os-machine-2"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR.String()))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", "states/machines/os-machine-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("UpdateState for machine lse lacking perms", func(t *ftt.Test) {
			// user has no realm permissions
			noPermsCtx := withAuthorizedNoPermsUser(osCtx)
			state.UpdateStateRecord(noPermsCtx, &ufspb.StateRecord{
				ResourceName: "machinelses/os-machine-3",
				State:        ufspb.State_STATE_SERVING,
			})
			inventory.CreateMachineLSE(noPermsCtx, &ufspb.MachineLSE{
				Name:  "os-machine-3",
				Realm: util.AtlLabAdminRealm,
			})

			sr := &ufspb.StateRecord{
				ResourceName: "machineslses/os-machine-3",
				State:        ufspb.State_STATE_NEEDS_REPAIR,
			}
			res, err := UpdateState(noPermsCtx, sr)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))

			changes, err := history.QueryChangesByPropertyName(osCtx, "name", "states/machines/os-machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", "states/machines/os-machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})
		t.Run("UpdateState for machine lse with perms", func(t *ftt.Test) {
			// user has correct realm perms
			atlPermsCtx := withAuthorizedAtlUser(osCtx)
			state.UpdateStateRecord(atlPermsCtx, &ufspb.StateRecord{
				ResourceName: "machinelses/os-machine-4",
				State:        ufspb.State_STATE_SERVING,
			})
			inventory.CreateMachineLSE(atlPermsCtx, &ufspb.MachineLSE{
				Name:  "os-machine-4",
				Realm: util.AtlLabAdminRealm,
			})

			sr := &ufspb.StateRecord{
				ResourceName: "machinelses/os-machine-4",
				State:        ufspb.State_STATE_NEEDS_REPAIR,
			}
			res, err := UpdateState(atlPermsCtx, sr)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(sr))

			res, err = state.GetStateRecord(atlPermsCtx, "machinelses/os-machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetResourceName(), should.Equal("machinelses/os-machine-4"))
			assert.Loosely(t, res.GetState(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR))

			changes, err := history.QueryChangesByPropertyName(atlPermsCtx, "name", "states/machinelses/os-machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetName(), should.Equal("states/machinelses/os-machine-4"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR.String()))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(atlPermsCtx, "resource_name", "states/machinelses/os-machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
	})
}
