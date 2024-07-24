// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"google.golang.org/genproto/protobuf/field_mask"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/model/caching"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/state"
)

func mockCachingService(name string) *ufspb.CachingService {
	return &ufspb.CachingService{
		Name: name,
	}
}

func TestCreateCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("CreateCachingService", t, func(t *ftt.Test) {
		t.Run("Create new CachingService - happy path", func(t *ftt.Test) {
			cs := mockCachingService("127.0.0.1")
			cs.State = ufspb.State_STATE_SERVING
			resp, err := CreateCachingService(ctx, cs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(cs))

			s, err := state.GetStateRecord(ctx, "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("cachingservice"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new CachingService - already existing", func(t *ftt.Test) {
			cs1 := mockCachingService("128.0.0.1")
			caching.CreateCachingService(ctx, cs1)

			cs2 := mockCachingService("128.0.0.1")
			_, err := CreateCachingService(ctx, cs2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already exists"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "cachingservices/128.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "cachingservices/128.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})
	})
}

func TestUpdateCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateCachingService", t, func(t *ftt.Test) {
		t.Run("Update CachingService for existing CachingService - happy path", func(t *ftt.Test) {
			cs1 := mockCachingService("127.0.0.1")
			cs1.Port = 43560
			caching.CreateCachingService(ctx, cs1)

			cs2 := mockCachingService("127.0.0.1")
			cs2.Port = 25653
			resp, _ := UpdateCachingService(ctx, cs2, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(cs2))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("cachingservice.port"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("43560"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("25653"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update CachingService for non-existing CachingService", func(t *ftt.Test) {
			cs := mockCachingService("128.0.0.1")
			resp, err := UpdateCachingService(ctx, cs, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "cachingservices/128.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update CachingService for existing CachingService - partial update state", func(t *ftt.Test) {
			cs1 := mockCachingService("129.0.0.1")
			cs1.Port = 10101
			cs1.PrimaryNode = "0.0.0.0"
			cs1.State = ufspb.State_STATE_SERVING
			caching.CreateCachingService(ctx, cs1)

			cs2 := mockCachingService("129.0.0.1")
			cs2.State = ufspb.State_STATE_DISABLED
			resp, _ := UpdateCachingService(ctx, cs2, &field_mask.FieldMask{Paths: []string{"state"}})
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetName(), should.Equal(cs2.GetName()))
			assert.Loosely(t, resp.GetPort(), should.Equal(10101))
			assert.Loosely(t, resp.GetPrimaryNode(), should.Equal("0.0.0.0"))
			assert.Loosely(t, resp.GetState(), should.Equal(ufspb.State_STATE_DISABLED))

			s, err := state.GetStateRecord(ctx, "cachingservices/129.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_DISABLED))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "cachingservices/129.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("cachingservice.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DISABLED.String()))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "cachingservices/129.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
	})
}

func TestGetCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	cs, _ := caching.CreateCachingService(ctx, &ufspb.CachingService{
		Name: "127.0.0.1",
	})
	ftt.Run("GetCachingService", t, func(t *ftt.Test) {
		t.Run("Get CachingService by existing ID - happy path", func(t *ftt.Test) {
			resp, _ := GetCachingService(ctx, "127.0.0.1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(cs))
		})

		t.Run("Get CachingService by non-existing ID", func(t *ftt.Test) {
			_, err := GetCachingService(ctx, "128.0.0.1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestDeleteCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	caching.CreateCachingService(ctx, &ufspb.CachingService{
		Name: "127.0.0.1",
	})
	ftt.Run("DeleteCachingService", t, func(t *ftt.Test) {
		t.Run("Delete CachingService by existing ID - happy path", func(t *ftt.Test) {
			err := DeleteCachingService(ctx, "127.0.0.1")
			assert.Loosely(t, err, should.BeNil)

			res, err := caching.GetCachingService(ctx, "127.0.0.1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "cachingservices/127.0.0.1")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("cachingservice"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "cachingservices/127.0.0.1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete CachingService by non-existing ID", func(t *ftt.Test) {
			err := DeleteCachingService(ctx, "128.0.0.1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestListCachingServices(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	cachingServicesWithState := make([]*ufspb.CachingService, 0, 2)
	cachingServices := make([]*ufspb.CachingService, 0, 4)
	for i := 0; i < 4; i++ {
		cs := mockCachingService(fmt.Sprintf("cs-%d", i))
		if i%2 == 0 {
			cs.State = ufspb.State_STATE_SERVING
		}
		resp, _ := caching.CreateCachingService(ctx, cs)
		if i%2 == 0 {
			cachingServicesWithState = append(cachingServicesWithState, resp)
		}
		cachingServices = append(cachingServices, resp)
	}
	ftt.Run("ListCachingServices", t, func(t *ftt.Test) {
		t.Run("List CachingServices - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListCachingServices(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List CachingServices - filter switch - happy path", func(t *ftt.Test) {
			resp, _, _ := ListCachingServices(ctx, 5, "", "state=serving", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(cachingServicesWithState))
		})

		t.Run("ListCachingServices - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListCachingServices(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(cachingServices))
		})
	})
}
