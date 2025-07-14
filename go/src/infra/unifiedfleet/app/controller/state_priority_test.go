// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestUpdateMachineLSEStateByHumanAndService(t *testing.T) {
	// Don't run this test in parallel. It depends on a package level variable.
	ctx := testingContext()
	ParseStateChangePriorityCfg(config.Get(ctx))
	lseName := "machinelse-update-human"
	ftt.Run("Update machineLSE state by human and service", t, func(t *ftt.Test) {
		machineLSE1 := &ufspb.MachineLSE{
			Name:     lseName,
			Hostname: lseName,
			Machines: []string{lseName},
			Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
				ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
			},
		}
		_, err := registration.CreateMachine(ctx, &ufspb.Machine{
			Name: lseName,
		})
		assert.Loosely(t, err, should.BeNil)
		_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
		assert.Loosely(t, err, should.BeNil)

		{ // One human user changes the state.
			machineLSE1.ResourceState = ufspb.State_STATE_REGISTERED
			ctx := initializeFakeAuthDB(ctx, "user:alice@google.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_REGISTERED))
		}
		{ // State can be changed by another human user in any time to any value.
			machineLSE1.ResourceState = ufspb.State_STATE_DEPLOYED_TESTING
			ctx := initializeFakeAuthDB(ctx, "user:bob@google.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DEPLOYED_TESTING))
		}
		{ // The service can change the human set state.
			machineLSE1.ResourceState = ufspb.State_STATE_MISSING
			ctx := initializeFakeAuthDB(ctx, "user:account@service.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_MISSING))
		}
		{ // A service can change the state set by another service immediately.
			machineLSE1.ResourceState = ufspb.State_STATE_DISABLED
			ctx := initializeFakeAuthDB(ctx, "user:account2@service.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DISABLED))
		}
		{ // A human can change a service set state in any time, to any value.
			machineLSE1.ResourceState = ufspb.State_STATE_SERVING
			ctx := initializeFakeAuthDB(ctx, "user:bob@google.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))
		}
	})
}
func TestUpdateMachineLSEStateByHumanAndServiceWithPriority(t *testing.T) {
	// Don't run this test in parallel. It depends on a package level variable.

	ctx := config.Use(testingContext(), mockStateChangeReqPriority())
	ParseStateChangePriorityCfg(config.Get(ctx))
	lseName := "machinelse-update-human-priority"
	ftt.Run("Update machineLSE state by human and service with priority", t, func(t *ftt.Test) {
		machineLSE1 := &ufspb.MachineLSE{
			Name:     lseName,
			Hostname: lseName,
			Machines: []string{lseName},
			Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
				ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
			},
		}
		_, err := registration.CreateMachine(ctx, &ufspb.Machine{
			Name: lseName,
		})
		assert.Loosely(t, err, should.BeNil)
		_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
		assert.Loosely(t, err, should.BeNil)

		ctxHuman := initializeFakeAuthDB(ctx, "user:alice@google.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
		ctxServiceEvenLower := initializeFakeAuthDB(ctx, "user:even-lower@service.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
		ctxServiceLower := initializeFakeAuthDB(ctx, "user:lower@service.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
		ctxServiceHigher := initializeFakeAuthDB(ctx, "user:higher@service.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
		{ // One human user changes the state REGISTERED.
			machineLSE1.ResourceState = ufspb.State_STATE_REGISTERED
			resp, err := UpdateMachineLSE(ctxHuman, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_REGISTERED))
		}
		{ // A service can disable the LSE later as DISABLED request is configured
			// with higher priority than REGISTERED.
			machineLSE1.ResourceState = ufspb.State_STATE_DISABLED
			resp, err := UpdateMachineLSE(ctxServiceLower, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DISABLED))
		}
		{ // The human reserve the LSE.
			machineLSE1.ResourceState = ufspb.State_STATE_RESERVED
			resp, err := UpdateMachineLSE(ctxHuman, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_RESERVED))
		}
		{ // The service cannot change the LSE to serving because the RESERVED state
			// from a human is configured with higher priority.
			machineLSE1.ResourceState = ufspb.State_STATE_SERVING
			_, err := UpdateMachineLSE(ctxServiceLower, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			// The state will still be 'reserved'.
			lse, _ := inventory.GetMachineLSE(ctx, lseName)
			assert.Loosely(t, lse.GetResourceState(), should.Equal(ufspb.State_STATE_RESERVED))
			s, err := state.GetStateRecord(ctx, util.AddPrefix("hosts", lseName))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetUser(), should.Equal("alice@google.com"))
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_RESERVED))
		}
		{ // The human release the LSE by setting the state to a very low state,
			// i.e.READY
			machineLSE1.ResourceState = ufspb.State_STATE_READY
			resp, err := UpdateMachineLSE(ctxHuman, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_READY))
		}
		{ // The lower service change the state after some time.
			machineLSE1.ResourceState = ufspb.State_STATE_DISABLED
			resp, err := UpdateMachineLSE(ctxServiceLower, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DISABLED))
		}
		{ // The higher service can overwrite the state.
			machineLSE1.ResourceState = ufspb.State_STATE_MISSING
			resp, err := UpdateMachineLSE(ctxServiceHigher, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_MISSING))
		}
		{ // The even-lower service requests state change, but it won't be effect.
			machineLSE1.ResourceState = ufspb.State_STATE_BUILD
			resp, err := UpdateMachineLSE(ctxServiceEvenLower, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			// State is not changed, so still 'MISSING' which is set by the 'higher
			// service'.
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_MISSING))
		}
		{ // When the higher service release the LSE, the previous set lower
			// prioritized state will take place. But the even-lower service's
			// request is still pending.
			machineLSE1.ResourceState = ufspb.State_STATE_SERVING
			resp, err := UpdateMachineLSE(ctxServiceHigher, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			// The previous state was DISABLED set by the lower service.
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DISABLED))
		}
	})
}

func mockStateChangeReqPriority() *config.Config {
	return &config.Config{
		StateChangeReqPriority: &config.StateChangeReqPriority{
			Priorities: []*config.StateChangeReqPriority_Priority{
				{
					UserRegexp:     ".*@google.com",
					IncludedStates: []ufspb.State{ufspb.State_STATE_DISABLED, ufspb.State_STATE_DECOMMISSIONED, ufspb.State_STATE_RESERVED},
					Priority:       100,
				},
				{ // A higher priority service.
					UserRegexp:     "^higher@service.com$",
					IncludedStates: []ufspb.State{ufspb.State_STATE_MISSING},
					Priority:       50,
				},
				{ // A lower priority service.
					UserRegexp:     "^lower@service.com$",
					IncludedStates: []ufspb.State{ufspb.State_STATE_DISABLED, ufspb.State_STATE_SERVING},
					ExcludedStates: []ufspb.State{ufspb.State_STATE_SERVING},
					Priority:       40,
				},
				{ // An even lower priority service.
					UserRegexp:     "^even-lower@service.com$",
					IncludedStates: []ufspb.State{ufspb.State_STATE_BUILD},
					Priority:       30,
				},
				{
					UserRegexp: "foo",
					Priority:   1,
				},
				// The default priority for all other requests will be 0.
			},
		},
	}
}
