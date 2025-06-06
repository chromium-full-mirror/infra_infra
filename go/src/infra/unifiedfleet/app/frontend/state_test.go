// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	api "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestUpdateState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Update state", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "hosts/chromeos1-row2-rack3-host4",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			s, err := state.GetStateRecord(osCtx, "hosts/chromeos1-row2-rack3-host4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetResourceName(), should.Equal("hosts/chromeos1-row2-rack3-host4"))
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_RESERVED))
		})
		t.Run("invalid resource prefix", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "resources/chromeos1-row2-rack3-host4",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("empty resource name", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("invalid characters in resource name", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "hosts/host1@_@",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
	})
}

func TestGetState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Get state", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			s := &ufspb.StateRecord{
				ResourceName: "hosts/chromeos1-row2-rack3-host4",
				State:        ufspb.State_STATE_RESERVED,
			}
			_, err := state.UpdateStateRecord(ctx, s)
			assert.Loosely(t, err, should.BeNil)
			req := &api.GetStateRequest{
				ResourceName: "hosts/chromeos1-row2-rack3-host4",
			}
			res, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(s))
		})
		t.Run("valid resource name, but not found", func(t *ftt.Test) {
			res, err := tf.Fleet.GetState(ctx, &api.GetStateRequest{
				ResourceName: "hosts/chromeos-fakehost",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("invalid resource prefix", func(t *ftt.Test) {
			req := &api.GetStateRequest{
				ResourceName: "resources/chromeos1-row2-rack3-host4",
			}
			_, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("empty resource name", func(t *ftt.Test) {
			req := &api.GetStateRequest{
				ResourceName: "",
			}
			_, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("invalid characters in resource name", func(t *ftt.Test) {
			req := &api.GetStateRequest{
				ResourceName: "hosts/host1@_@",
			}
			_, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
	})
}
