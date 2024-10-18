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

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/model/configuration"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/inventory"
)

func mockRackLSEPrototype(id string) *ufspb.RackLSEPrototype {
	return &ufspb.RackLSEPrototype{
		Name: id,
	}
}

func TestListRackLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rackLSEPrototypes := make([]*ufspb.RackLSEPrototype, 0, 4)
	for i := 0; i < 4; i++ {
		rackLSEPrototype1 := mockRackLSEPrototype("")
		rackLSEPrototype1.Name = fmt.Sprintf("rackLSEPrototype-%d", i)
		resp, _ := configuration.CreateRackLSEPrototype(ctx, rackLSEPrototype1)
		rackLSEPrototypes = append(rackLSEPrototypes, resp)
	}
	ftt.Run("ListRackLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("List RackLSEPrototypes - filter invalid", func(t *ftt.Test) {
			_, _, err := ListRackLSEPrototypes(ctx, 5, "", "machine=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to read filter for listing racklseprototypes"))
		})

		t.Run("ListRackLSEPrototypes - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListRackLSEPrototypes(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototypes))
		})
	})
}

func TestDeleteRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rackLSEPrototype1 := mockRackLSEPrototype("rackLSEPrototype-1")
	rackLSEPrototype2 := mockRackLSEPrototype("rackLSEPrototype-2")
	ftt.Run("DeleteRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Delete rackLSEPrototype by existing ID with racklse reference", func(t *ftt.Test) {
			resp, cerr := configuration.CreateRackLSEPrototype(ctx, rackLSEPrototype1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))

			rackLSE1 := &ufspb.RackLSE{
				Name:             "racklse-1",
				RackLsePrototype: "rackLSEPrototype-1",
			}
			mresp, merr := inventory.CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(rackLSE1))

			err := DeleteRackLSEPrototype(ctx, "rackLSEPrototype-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(CannotDelete))

			resp, cerr = configuration.GetRackLSEPrototype(ctx, "rackLSEPrototype-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
		})
		t.Run("Delete rackLSEPrototype successfully by existing ID without references", func(t *ftt.Test) {
			resp, cerr := configuration.CreateRackLSEPrototype(ctx, rackLSEPrototype2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype2))

			err := DeleteRackLSEPrototype(ctx, "rackLSEPrototype-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = configuration.GetRackLSEPrototype(ctx, "rackLSEPrototype-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestBatchGetRackLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetRackLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("Batch get rack lse prototypes - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.RackLSEPrototype, 4)
			for i := 0; i < 4; i++ {
				entities[i] = &ufspb.RackLSEPrototype{
					Name: fmt.Sprintf("racklseprototype-batchGet-%d", i),
				}
			}
			_, err := configuration.BatchUpdateRackLSEPrototypes(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := configuration.BatchGetRackLSEPrototypes(ctx, []string{"racklseprototype-batchGet-0", "racklseprototype-batchGet-1", "racklseprototype-batchGet-2", "racklseprototype-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get rack lse prototypes  - missing id", func(t *ftt.Test) {
			resp, err := configuration.BatchGetRackLSEPrototypes(ctx, []string{"racklseprototype-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("racklseprototype-batchGet-non-existing"))
		})
		t.Run("Batch get rack lse prototypes  - empty input", func(t *ftt.Test) {
			resp, err := configuration.BatchGetRackLSEPrototypes(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = configuration.BatchGetRackLSEPrototypes(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}
