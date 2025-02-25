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

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func mockMachineLSEDeployment(id string) *ufspb.MachineLSEDeployment {
	return &ufspb.MachineLSEDeployment{
		SerialNumber: id,
	}
}

func TestUpdateMachineLSEDeployment(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("UpdateMachineLSE", t, func(t *ftt.Test) {
		t.Run("Update non-existing machineLSEDeployment", func(t *ftt.Test) {
			md1 := mockMachineLSEDeployment("serial-1")
			resp, err := UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{md1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(md1))
		})

		t.Run("Update existing machineLSEDeployment", func(t *ftt.Test) {
			md2 := mockMachineLSEDeployment("serial-2")
			resp, err := UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{md2})
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			md2.Hostname = "hostname-2"
			resp, err = UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{md2})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(md2))
		})

		t.Run("Update machineLSEDeployment - invalid hostname", func(t *ftt.Test) {
			md3 := mockMachineLSEDeployment("")
			resp, err := UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{md3})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Empty"))
		})
	})
}

func TestGetMachineLSEDeployment(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	dr1 := mockMachineLSEDeployment("dr-get-1")
	ftt.Run("GetMachineLSEDeployment", t, func(t *ftt.Test) {
		t.Run("Get machine deployment record by existing ID", func(t *ftt.Test) {
			resp, err := UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{dr1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp[0], should.Match(dr1))
			respDr, err := GetMachineLSEDeployment(ctx, "dr-get-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respDr, should.Match(dr1))
		})

		t.Run("Get machine deployment record by non-existing ID", func(t *ftt.Test) {
			resp, err := GetMachineLSEDeployment(ctx, "dr-get-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machine deployment record - invalid ID", func(t *ftt.Test) {
			resp, err := GetMachineLSEDeployment(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestBatchGetMachineLSEDeployments(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("BatchGetMachineLSEDeployments", t, func(t *ftt.Test) {
		t.Run("Batch get machine lse deployments - happy path", func(t *ftt.Test) {
			drs := make([]*ufspb.MachineLSEDeployment, 4)
			for i := range 4 {
				drs[i] = mockMachineLSEDeployment(fmt.Sprintf("dr-batchGet-%d", i))
			}
			_, err := UpdateMachineLSEDeployments(ctx, drs)
			assert.Loosely(t, err, should.BeNil)
			resp, err := BatchGetMachineLSEDeployments(ctx, []string{"dr-batchGet-0", "dr-batchGet-1", "dr-batchGet-2", "dr-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(drs))
		})

		t.Run("Batch get machine lse deployments - missing id", func(t *ftt.Test) {
			resp, err := BatchGetMachineLSEDeployments(ctx, []string{"dr-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("dr-batchGet-non-existing"))
		})

		t.Run("Batch get machine lse deployments - empty input", func(t *ftt.Test) {
			resp, err := BatchGetMachineLSEDeployments(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = BatchGetMachineLSEDeployments(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestListMachineLSEDeployments(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	drs := make([]*ufspb.MachineLSEDeployment, 4)
	for i := range 4 {
		drs[i] = mockMachineLSEDeployment(fmt.Sprintf("dr-List-%d", i))
	}
	updatedDrs, _ := UpdateMachineLSEDeployments(ctx, drs)
	ftt.Run("ListMachineLSEDeployments", t, func(t *ftt.Test) {
		t.Run("List machine lse deployment records - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEDeployments(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List machine lse deployment records - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEDeployments(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(updatedDrs))
		})

		t.Run("List machine lse deployment records - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEDeployments(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(updatedDrs[:3]))

			resp, _, err = ListMachineLSEDeployments(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(updatedDrs[3:]))
		})
	})
}
