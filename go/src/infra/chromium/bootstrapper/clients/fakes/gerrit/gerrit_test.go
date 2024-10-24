// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gerrit

import (
	"context"
	"sort"
	"strconv"
	"testing"

	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/chromium/bootstrapper/clients/gerrit"
)

func TestFactory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Factory", t, func(t *ftt.Test) {

		t.Run("returns an RPC client by default", func(t *ftt.Test) {
			factory := Factory(nil)

			client, err := factory(ctx, "fake-host")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, client, should.NotBeNil)
		})

		t.Run("fails for a nil host", func(t *ftt.Test) {
			factory := Factory(map[string]*Host{
				"fake-host": nil,
			})

			client, err := factory(ctx, "fake-host")

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, client, should.BeNil)
		})

		t.Run("returns RPC client for provided host", func(t *ftt.Test) {
			host := &Host{}
			factory := Factory(map[string]*Host{
				"fake-host": host,
			})

			client, err := factory(ctx, "fake-host")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, client, should.Resemble(&Client{
				hostname: "fake-host",
				gerrit:   host,
			}))
		})
	})
}

func getChangeRequest(project string, number int64) *gerritpb.GetChangeRequest {
	return &gerritpb.GetChangeRequest{
		Project: project,
		Number:  number,
		Options: []gerritpb.QueryOption{gerritpb.QueryOption_ALL_REVISIONS},
	}
}

func TestGetChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Client.GetChange", t, func(t *ftt.Test) {

		t.Run("returns a change info by default", func(t *ftt.Test) {
			client, _ := Factory(nil)(ctx, "fake-host")

			changeInfo, err := client.GetChange(ctx, getChangeRequest("fake/project", 123))

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changeInfo, should.NotBeNil)
			assert.Loosely(t, changeInfo.Project, should.Equal("fake/project"))
			assert.Loosely(t, changeInfo.Number, should.Equal(123))
			assert.Loosely(t, changeInfo.Revisions, should.HaveLength(1))
			for rev, revInfo := range changeInfo.Revisions {
				assert.Loosely(t, rev, should.NotBeEmpty)
				assert.Loosely(t, revInfo.Number, should.Equal(1))
			}
		})

		t.Run("fails for a nil project", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": nil,
					},
				},
			})(ctx, "fake-host")

			changeInfo, err := client.GetChange(ctx, getChangeRequest("fake/project", 123))

			assert.Loosely(t, err, should.ErrLike(`unknown project "fake/project" on host "fake-host"`))
			assert.Loosely(t, changeInfo, should.BeNil)
		})

		t.Run("fails for a nil change", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Changes: map[int64]*Change{
								123: nil,
							},
						},
					},
				},
			})(ctx, "fake-host")

			changeInfo, err := client.GetChange(ctx, getChangeRequest("fake/project", 123))

			assert.Loosely(t, err, should.ErrLike(`change 123 does not exist for project "fake/project" on host "fake-host"`))
			assert.Loosely(t, changeInfo, should.BeNil)
		})

		t.Run("returns info for provided change without revisions", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Changes: map[int64]*Change{
								123: {
									Ref: "fake-ref",
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			changeInfo, err := client.GetChange(ctx, getChangeRequest("fake/project", 123))

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changeInfo, should.NotBeNil)
			assert.Loosely(t, changeInfo.Project, should.Equal("fake/project"))
			assert.Loosely(t, changeInfo.Number, should.Equal(123))
			assert.Loosely(t, changeInfo.Ref, should.Equal("fake-ref"))
			for rev, revInfo := range changeInfo.Revisions {
				assert.Loosely(t, rev, should.NotBeEmpty)
				assert.Loosely(t, revInfo.Number, should.Equal(1))
			}
		})

		t.Run("returns info for provided change with revisions", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Changes: map[int64]*Change{
								123: {
									Ref: "fake-ref",
									Patchsets: map[int32]*Patchset{
										4: {Revision: "fake-revision-4"},
										7: {Revision: ""},
									},
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			changeInfo, err := client.GetChange(ctx, getChangeRequest("fake/project", 123))

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changeInfo, should.NotBeNil)
			assert.Loosely(t, changeInfo.Project, should.Equal("fake/project"))
			assert.Loosely(t, changeInfo.Number, should.Equal(123))
			assert.Loosely(t, changeInfo.Ref, should.Equal("fake-ref"))
			assert.Loosely(t, changeInfo.Revisions, should.ContainKey("fake-revision-4"))
			assert.Loosely(t, changeInfo.Revisions["fake-revision-4"].Number, should.Equal(4))
			var patchsets []int
			for rev, revInfo := range changeInfo.Revisions {
				assert.Loosely(t, rev, should.NotBeEmpty)
				patchsets = append(patchsets, int(revInfo.Number))
			}
			sort.Ints(patchsets)
			assert.Loosely(t, patchsets, should.Resemble([]int{1, 2, 3, 4, 5, 6, 7}))
		})

	})

}

func listFilesRequest(project string, change int64, patchset int32) *gerritpb.ListFilesRequest {
	return &gerritpb.ListFilesRequest{
		Project:    project,
		Number:     change,
		RevisionId: strconv.FormatInt(int64(patchset), 10),
	}
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("gerrit using fake factory", t, func(t *ftt.Test) {

		t.Run("succeeds when calling GetChangeInfo", func(t *ftt.Test) {
			ctx := gerrit.UseGerritClientFactory(ctx, Factory(nil))
			client := gerrit.NewClient(ctx)

			info, err := client.GetChangeInfo(ctx, "fake-host", "fake/project", 234, 1)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, info, should.NotBeNil)
		})

	})
}
