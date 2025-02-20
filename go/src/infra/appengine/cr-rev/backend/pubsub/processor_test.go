// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pubsub

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/proto/git"
	gitilesProto "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/cr-rev/backend/gitiles"
	"go.chromium.org/infra/appengine/cr-rev/config"
	"go.chromium.org/infra/appengine/cr-rev/models"
)

func TestPubsubProcessor(t *testing.T) {
	ctx := gaetesting.TestingContext()
	ds := datastore.GetTestable(ctx)
	ds.Consistent(true)
	ds.AutoIndex(true)

	host := &config.Host{
		Name: "foo",
		Repos: []*config.Repository{
			{
				Name: "not-indexed",
				Indexing: &config.Repository_DoNotIndex{
					DoNotIndex: true,
				},
			},
			{
				Name:        "custom-refs",
				Refs:        []string{"refs/my-heads"},
				ExcludeRefs: []string{"refs/my-heads/exclude"},
			},
		},
	}
	processor := Processor(host)
	ftt.Run("invalid event name", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "invalid/name",
		}
		err := processor(ctx, m)
		assert.Loosely(t, err, should.ErrLike("Invalid repository format"))
	})

	ftt.Run("skip not indexed", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/not-indexed",
		}
		// We don't expect any gitiles calls, therefore we are not
		// setting gitiles fake to return anything.
		err := processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("non update events", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/bar",
			Event: &SourceRepoEvent_CreateRepoEvent_{
				CreateRepoEvent: &SourceRepoEvent_CreateRepoEvent{},
			},
		}
		// We don't expect any gitiles calls, therefore we are not
		// setting gitiles fake to return anything.
		err := processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("valid events", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/bar",
			Event: &SourceRepoEvent_RefUpdateEvent_{
				RefUpdateEvent: &SourceRepoEvent_RefUpdateEvent{
					RefUpdates: map[string]*SourceRepoEvent_RefUpdateEvent_RefUpdate{
						// indexed:
						"refs/heads/master": {
							RefName: "refs/heads/foo",
							OldId:   "000000000000000000000000000000000000000E",
							NewId:   "000000000000000000000000000000000000000F",
						},
						// not indexed:
						"refs/foo": {
							RefName: "refs/foo",
							OldId:   "0000000000000000000000000000000000000010",
							NewId:   "000000000000000000000000000000000000001F",
						},
					},
				},
			},
		}
		commits := []*git.Commit{
			{
				Id:      "000000000000000000000000000000000000000F",
				Parents: []string{"000000000000000000000000000000000000000E"},
			},
			{
				Id:      "000000000000000000000000000000000000000E",
				Parents: []string{"000000000000000000000000000000000000000D"},
			},
			{
				Id: "000000000000000000000000000000000000000D",
			},
			{
				Id: "000000000000000000000000000000000000001F",
			},
		}
		c := &gitilesProto.Fake{}
		c.SetRepository("bar", nil, commits)
		ctx := gitiles.SetClient(ctx, c)
		err := processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)

		datastoreCommits := []*models.Commit{}
		q := datastore.NewQuery("Commit").Eq("Repository", "bar")
		datastore.GetAll(ctx, q, &datastoreCommits)
		assert.Loosely(t, len(datastoreCommits), should.Equal(1))
		assert.Loosely(t, datastoreCommits[0].CommitHash, should.Equal("000000000000000000000000000000000000000F"))
	})

	ftt.Run("respect include/exclude refs", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/custom-refs",
			Event: &SourceRepoEvent_RefUpdateEvent_{
				RefUpdateEvent: &SourceRepoEvent_RefUpdateEvent{
					RefUpdates: map[string]*SourceRepoEvent_RefUpdateEvent_RefUpdate{
						// not-indexed:
						"refs/heads/master": {
							RefName: "refs/heads/master",
							OldId:   "000000000000000000000000000000000000000E",
							NewId:   "000000000000000000000000000000000000000F",
						},
						"refs/my-heads/excluded": {
							RefName: "refs/my-heads/exclude",
							OldId:   "000000000000000000000000000000000000000D",
							NewId:   "000000000000000000000000000000000000000E",
						},
						// indexed:
						"refs/my-heads/indexed": {
							RefName: "refs/my-heads/indexed",
							OldId:   "000000000000000000000000000000000000000C",
							NewId:   "000000000000000000000000000000000000000D",
						},
					},
				},
			},
		}
		commits := []*git.Commit{
			{
				Id:      "000000000000000000000000000000000000000F",
				Parents: []string{"000000000000000000000000000000000000000E"},
			},
			{
				Id:      "000000000000000000000000000000000000000E",
				Parents: []string{"000000000000000000000000000000000000000D"},
			},
			{
				Id:      "000000000000000000000000000000000000000D",
				Parents: []string{"000000000000000000000000000000000000000C"},
			},
			{
				Id: "000000000000000000000000000000000000000C",
			},
		}
		c := &gitilesProto.Fake{}
		c.SetRepository("custom-refs", nil, commits)
		ctx := gitiles.SetClient(ctx, c)
		err := processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)

		datastoreCommits := []*models.Commit{}
		q := datastore.NewQuery("Commit").Eq("Repository", "custom-refs")
		datastore.GetAll(ctx, q, &datastoreCommits)
		assert.Loosely(t, len(datastoreCommits), should.Equal(1))
		assert.Loosely(t, datastoreCommits[0].CommitHash, should.Equal("000000000000000000000000000000000000000D"))
	})

	ftt.Run("create ref", t, func(t *ftt.Test) {
		n := 2001
		commits := make([]*git.Commit, n)
		for i := 0; i < n; i++ {
			commits[i] = &git.Commit{
				Id: fmt.Sprintf("%040x", i),
			}
			if i > 0 {
				commits[i].Parents = []string{commits[i-1].Id}
			}
		}
		c := &gitilesProto.Fake{}
		c.SetRepository("createref", nil, commits)
		ctx := gitiles.SetClient(ctx, c)
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/createref",
			Event: &SourceRepoEvent_RefUpdateEvent_{
				RefUpdateEvent: &SourceRepoEvent_RefUpdateEvent{
					RefUpdates: map[string]*SourceRepoEvent_RefUpdateEvent_RefUpdate{
						"refs/heads/master": {
							RefName: "refs/heads/master",
							OldId:   "",
							NewId:   commits[n-1].Id,
						},
					},
				},
			},
		}
		err := processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)

		datastoreCommits := []*models.Commit{}
		q := datastore.NewQuery("Commit").Eq("Repository", "createref")
		datastore.GetAll(ctx, q, &datastoreCommits)
		assert.Loosely(t, len(datastoreCommits), should.Equal(n))

		// 3 Gitiles Log calls (2001..1001, 1001..1, 1..0)
		assert.Loosely(t, len(c.GetCallLogs()), should.Equal(3))

		// Re-run indexing, we expect only one call to gitiles
		err = processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)

		datastoreCommits = []*models.Commit{}
		datastore.GetAll(ctx, q, &datastoreCommits)
		assert.Loosely(t, len(datastoreCommits), should.Equal(n))

		assert.Loosely(t, len(c.GetCallLogs()), should.Equal(4))
	})

	ftt.Run("ignore deleted branch", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/deleted_branch",
			Event: &SourceRepoEvent_RefUpdateEvent_{
				RefUpdateEvent: &SourceRepoEvent_RefUpdateEvent{
					RefUpdates: map[string]*SourceRepoEvent_RefUpdateEvent_RefUpdate{
						"refs/heads/master": {
							RefName:    "refs/heads/master",
							NewId:      "0000000000000000000000000000000000000001",
							OldId:      "0000000000000000000000000000000000000000",
							UpdateType: SourceRepoEvent_RefUpdateEvent_RefUpdate_DELETE,
						},
					},
				},
			},
		}
		commits := []*git.Commit{
			{
				Id:      "0000000000000000000000000000000000000001",
				Parents: []string{"0000000000000000000000000000000000000000"},
			},
			{
				Id: "0000000000000000000000000000000000000000",
			},
		}
		c := &gitilesProto.Fake{}
		c.SetRepository("deleted_branch", nil, commits)
		ctx := gitiles.SetClient(ctx, c)
		err := processor(ctx, m)
		assert.Loosely(t, err, should.BeNil)

		datastoreCommits := []*models.Commit{}
		q := datastore.NewQuery("Commit").Eq("Repository", "deleted_branch")
		datastore.GetAll(ctx, q, &datastoreCommits)
		assert.Loosely(t, len(datastoreCommits), should.BeZero)
	})

	ftt.Run("move forward with errors", t, func(t *ftt.Test) {
		m := &SourceRepoEvent{
			Name: "projects/foo/repos/partial_error",
			Event: &SourceRepoEvent_RefUpdateEvent_{
				RefUpdateEvent: &SourceRepoEvent_RefUpdateEvent{
					RefUpdates: map[string]*SourceRepoEvent_RefUpdateEvent_RefUpdate{
						"refs/heads/foo": {
							RefName: "refs/heads/foo",
							NewId:   "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
							OldId:   "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0",
						},
						"refs/heads/master": {
							RefName: "refs/heads/master",
							NewId:   "0000000000000000000000000000000000000001",
							OldId:   "0000000000000000000000000000000000000000",
						},
					},
				},
			},
		}
		commits := []*git.Commit{
			{
				Id:      "0000000000000000000000000000000000000001",
				Parents: []string{"0000000000000000000000000000000000000000"},
			},
			{
				Id: "0000000000000000000000000000000000000000",
			},
		}
		c := &gitilesProto.Fake{}
		c.SetRepository("partial_error", nil, commits)
		ctx := gitiles.SetClient(ctx, c)
		err := processor(ctx, m)
		assert.Loosely(t, err, should.ErrLike("not found"))

		datastoreCommits := []*models.Commit{}
		q := datastore.NewQuery("Commit").Eq("Repository", "partial_error")
		datastore.GetAll(ctx, q, &datastoreCommits)
		assert.Loosely(t, len(datastoreCommits), should.Equal(1))
	})
}
