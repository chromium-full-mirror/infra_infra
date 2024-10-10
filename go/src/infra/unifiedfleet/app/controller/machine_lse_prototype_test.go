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

func mockMachineLSEPrototype(id string) *ufspb.MachineLSEPrototype {
	return &ufspb.MachineLSEPrototype{
		Name: id,
	}
}

func TestListMachineLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	machineLSEPrototypes := make([]*ufspb.MachineLSEPrototype, 0, 4)
	for i := 0; i < 4; i++ {
		machineLSEPrototype1 := mockMachineLSEPrototype("")
		machineLSEPrototype1.Name = fmt.Sprintf("machineLSEPrototype-%d", i)
		resp, _ := configuration.CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
		machineLSEPrototypes = append(machineLSEPrototypes, resp)
	}
	ftt.Run("ListMachineLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("List MachineLSEPrototypes - filter invalid", func(t *ftt.Test) {
			_, _, err := ListMachineLSEPrototypes(ctx, 5, "", "machine=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to read filter for listing machinelseprototypes"))
		})

		t.Run("ListMachineLSEPrototypes - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListMachineLSEPrototypes(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototypes))
		})
	})
}

func TestDeleteMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	machineLSEPrototype1 := mockMachineLSEPrototype("machineLSEPrototype-1")
	machineLSEPrototype2 := mockMachineLSEPrototype("machineLSEPrototype-2")
	ftt.Run("DeleteMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Delete machineLSEPrototype by existing ID with machinelse reference", func(t *ftt.Test) {
			resp, cerr := configuration.CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))

			machineLSE1 := &ufspb.MachineLSE{
				Name:                "machinelse-1",
				MachineLsePrototype: "machineLSEPrototype-1",
			}
			mresp, merr := inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(machineLSE1))

			err := DeleteMachineLSEPrototype(ctx, "machineLSEPrototype-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(CannotDelete))

			resp, cerr = configuration.GetMachineLSEPrototype(ctx, "machineLSEPrototype-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
		})
		t.Run("Delete machineLSEPrototype successfully by existing ID without references", func(t *ftt.Test) {
			resp, cerr := configuration.CreateMachineLSEPrototype(ctx, machineLSEPrototype2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype2))

			err := DeleteMachineLSEPrototype(ctx, "machineLSEPrototype-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = configuration.GetMachineLSEPrototype(ctx, "machineLSEPrototype-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestBatchGetMachineLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetMachineLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("Batch get machine lse prototypes - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.MachineLSEPrototype, 4)
			for i := 0; i < 4; i++ {
				entities[i] = &ufspb.MachineLSEPrototype{
					Name: fmt.Sprintf("machinelseprototype-batchGet-%d", i),
				}
			}
			_, err := configuration.BatchUpdateMachineLSEPrototypes(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := configuration.BatchGetMachineLSEPrototypes(ctx, []string{"machinelseprototype-batchGet-0", "machinelseprototype-batchGet-1", "machinelseprototype-batchGet-2", "machinelseprototype-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get machine lse prototypes  - missing id", func(t *ftt.Test) {
			resp, err := configuration.BatchGetMachineLSEPrototypes(ctx, []string{"machinelseprototype-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("machinelseprototype-batchGet-non-existing"))
		})
		t.Run("Batch get machine lse prototypes  - empty input", func(t *ftt.Test) {
			resp, err := configuration.BatchGetMachineLSEPrototypes(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = configuration.BatchGetMachineLSEPrototypes(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}
