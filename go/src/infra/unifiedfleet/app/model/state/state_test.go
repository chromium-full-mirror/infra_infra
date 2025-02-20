// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func mockState(resource string, state ufspb.State) *ufspb.StateRecord {
	return &ufspb.StateRecord{
		ResourceName: resource,
		State:        state,
	}
}

func TestListState(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("list states", t, func(t *ftt.Test) {
		states := []*ufspb.StateRecord{
			mockState("machines/abc", ufspb.State_STATE_SERVING),
			mockState("vms/abc-1", ufspb.State_STATE_SERVING),
			mockState("vms/abc-2", ufspb.State_STATE_NEEDS_REPAIR),
		}
		t.Run("happy path with single filter", func(t *ftt.Test) {
			_, err := BatchUpdateStates(ctx, states)
			assert.Loosely(t, err, should.BeNil)

			resp, _, err := ListStateRecords(ctx, 10, "", map[string][]interface{}{
				"resource_type": {"vms"},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			for _, r := range resp {
				assert.Loosely(t, r.GetResourceName(), should.BeIn([]string{"vms/abc-1", "vms/abc-2"}...))
			}
		})
		t.Run("happy path with multiple filter", func(t *ftt.Test) {
			_, err := BatchUpdateStates(ctx, states)
			assert.Loosely(t, err, should.BeNil)

			resp, _, err := ListStateRecords(ctx, 10, "", map[string][]interface{}{
				"resource_type": {"vms"},
				"state":         {"STATE_SERVING"},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0].GetResourceName(), should.Equal("vms/abc-1"))
		})
	})
}
