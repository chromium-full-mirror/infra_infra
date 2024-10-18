// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	. "infra/unifiedfleet/app/model/datastore"
)

func mockSchedulingUnit(name string) *ufspb.SchedulingUnit {
	return &ufspb.SchedulingUnit{
		Name: name,
	}
}

func TestCreateSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("CreateSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Create new SchedulingUnit", func(t *ftt.Test) {
			su := mockSchedulingUnit("SU-X")
			resp, err := CreateSchedulingUnit(ctx, su)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(su))
		})
		t.Run("Create existing SchedulingUnit", func(t *ftt.Test) {
			su1 := mockSchedulingUnit("SU-Y")
			CreateSchedulingUnit(ctx, su1)

			resp, err := CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
	})
}

func TestBatchUpdateSchedulingUnits(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("BatchUpdateSchedulingUnits", t, func(t *ftt.Test) {
		t.Run("Create new SchedulingUnit", func(t *ftt.Test) {
			su := mockSchedulingUnit("SU-A")
			resp, err := BatchUpdateSchedulingUnits(ctx, []*ufspb.SchedulingUnit{su})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp[0], should.Match(su))
		})
	})
}

func TestQuerySchedulingUnitByPropertyNames(t *testing.T) {
	t.Parallel()
	keyOnlySchedulingUnit1 := &ufspb.SchedulingUnit{
		Name: "SchedulingUnit-1",
	}
	keyOnlySchedulingUnit2 := &ufspb.SchedulingUnit{
		Name: "SchedulingUnit-2",
	}
	keysOnlySchedulingUnits := []*ufspb.SchedulingUnit{keyOnlySchedulingUnit1, keyOnlySchedulingUnit2}
	schedulingUnit1 := &ufspb.SchedulingUnit{
		Name:        "SchedulingUnit-1",
		MachineLSEs: []string{"dut-1"},
		Pools:       []string{"pool-1", "pool-3"},
		Tags:        []string{"tags-3"},
	}
	schedulingUnit2 := &ufspb.SchedulingUnit{
		Name:        "SchedulingUnit-2",
		MachineLSEs: []string{"dut-2"},
		Pools:       []string{"pool-2", "pool-3"},
		Tags:        []string{"tags-3"},
	}
	schedulingUnits := []*ufspb.SchedulingUnit{schedulingUnit1, schedulingUnit2}
	ftt.Run("QuerySchedulingUnitByPropertyNames", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		_, err := BatchUpdateSchedulingUnits(ctx, schedulingUnits)
		assert.Loosely(t, err, should.BeNil)

		t.Run("Query By existing MachineLSE", func(t *ftt.Test) {
			resp, err := QuerySchedulingUnitByPropertyNames(ctx, map[string]string{"machinelses": "dut-1"}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.SchedulingUnit{schedulingUnit1}))
		})
		t.Run("Query By non-existing MachineLSE", func(t *ftt.Test) {
			resp, err := QuerySchedulingUnitByPropertyNames(ctx, map[string]string{"machinelses": "dut-4"}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing pools and tags", func(t *ftt.Test) {
			resp, err := QuerySchedulingUnitByPropertyNames(ctx, map[string]string{"pools": "pool-3", "tags": "tags-3"}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(schedulingUnits))
		})
		t.Run("Query By existing pools and MachineLSEs", func(t *ftt.Test) {
			resp, err := QuerySchedulingUnitByPropertyNames(ctx, map[string]string{"pools": "pool-3", "machinelses": "dut-2"}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.SchedulingUnit{schedulingUnit2}))
		})
		t.Run("Query By existing pools and tags by keysonly", func(t *ftt.Test) {
			resp, err := QuerySchedulingUnitByPropertyNames(ctx, map[string]string{"pools": "pool-3", "tags": "tags-3"}, true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(keysOnlySchedulingUnits))
		})
	})
}

func TestGetSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	su1 := mockSchedulingUnit("su-1")
	ftt.Run("GetSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Get SchedulingUnit by existing name/ID", func(t *ftt.Test) {
			resp, err := CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(su1))
			resp, err = GetSchedulingUnit(ctx, "su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(su1))
		})
		t.Run("Get SchedulingUnit by non-existing name/ID", func(t *ftt.Test) {
			resp, err := GetSchedulingUnit(ctx, "su-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get SchedulingUnit - invalid name/ID", func(t *ftt.Test) {
			resp, err := GetSchedulingUnit(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestDeleteSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	su1 := mockSchedulingUnit("su-1")
	CreateSchedulingUnit(ctx, su1)
	ftt.Run("DeleteSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Delete SchedulingUnit successfully by existing ID", func(t *ftt.Test) {
			err := DeleteSchedulingUnit(ctx, "su-1")
			assert.Loosely(t, err, should.BeNil)

			resp, err := GetSchedulingUnit(ctx, "su-1")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete SchedulingUnit by non-existing ID", func(t *ftt.Test) {
			err := DeleteSchedulingUnit(ctx, "su-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete SchedulingUnit - invalid ID", func(t *ftt.Test) {
			err := DeleteSchedulingUnit(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListSchedulingUnits(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	SchedulingUnits := make([]*ufspb.SchedulingUnit, 0, 4)
	for i := 0; i < 4; i++ {
		su := mockSchedulingUnit(fmt.Sprintf("su-%d", i))
		resp, _ := CreateSchedulingUnit(ctx, su)
		SchedulingUnits = append(SchedulingUnits, resp)
	}
	ftt.Run("ListSchedulingUnits", t, func(t *ftt.Test) {
		t.Run("List SchedulingUnits - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListSchedulingUnits(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List SchedulingUnits - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListSchedulingUnits(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(SchedulingUnits))
		})

		t.Run("List SchedulingUnits - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListSchedulingUnits(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(SchedulingUnits[:3]))

			resp, _, err = ListSchedulingUnits(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(SchedulingUnits[3:]))
		})
	})
}
