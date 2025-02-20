// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package repoimport

import (
	"context"
	"strconv"
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/proto/git"
	gitilesProto "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/cr-rev/backend/gitiles"
	"go.chromium.org/infra/appengine/cr-rev/common"
	"go.chromium.org/infra/appengine/cr-rev/models"
)

func TestGitilesImporter(t *testing.T) {
	repo := common.GitRepository{
		Host: "foo",
		Name: "bar",
	}

	doc := &models.Repository{
		ID: models.RepoID{
			Host:       "foo",
			Repository: "bar",
		},
	}

	prepareEnvironment := func() (context.Context, *gitilesProto.Fake, Importer) {
		ctx := gaetesting.TestingContext()
		ds := datastore.GetTestable(ctx)
		ds.Consistent(true)
		ds.AutoIndex(true)

		testclock := testclock.New(time.Now())
		ctx = clock.Set(ctx, testclock)

		client := &gitilesProto.Fake{}
		ctx = gitiles.SetClient(ctx, client)

		imp := NewGitilesImporter(ctx, repo)
		return ctx, client, imp
	}

	assertCommitDocuments := func(ctx context.Context, expected int) []*models.Commit {
		dsCommits := []*models.Commit{}
		q := datastore.NewQuery("Commit")
		datastore.GetAll(ctx, q, &dsCommits)
		assert.Loosely(t, len(dsCommits), should.Equal(expected))
		return dsCommits
	}

	ftt.Run("non existing repository", t, func(t *ftt.Test) {
		ctx, _, importer := prepareEnvironment()
		err := importer.Run(ctx)
		assert.Loosely(t, err, should.ErrLike("Repository not found"))
		assert.Loosely(t, err.Error(), should.Equal("Repository not found"))
		// Datastore should not have lock anymore, and should unset last run
		datastore.Get(ctx, doc)
		assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
		assert.Loosely(t, doc.FullScanLastRun, should.Match(time.Time{}))
	})

	ftt.Run("existing repository", t, func(t *ftt.Test) {
		t.Run("empty repository", func(t *ftt.Test) {
			ctx, client, importer := prepareEnvironment()

			client.SetRepository("bar", map[string]string{}, []*git.Commit{})
			err := importer.Run(ctx)
			assert.Loosely(t, err, should.BeNil)

			// Datastore should not have lock anymore, and last run should be set
			datastore.Get(ctx, doc)
			assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
			assert.Loosely(t, doc.FullScanLastRun, should.Match(clock.Get(ctx).Now().UTC().Round(time.Millisecond)))
		})

		t.Run("empty default branch", func(t *ftt.Test) {
			ctx, client, importer := prepareEnvironment()

			refs := map[string]string{
				"main": "",
			}
			commits := []*git.Commit{}
			client.SetRepository("bar", refs, commits)
			err := importer.Run(ctx)
			assert.Loosely(t, err, should.BeNil)
			// Datastore should not have lock anymore, and last run should be set
			datastore.Get(ctx, doc)
			assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
			assert.Loosely(t, doc.FullScanLastRun, should.Match(clock.Get(ctx).Now().UTC().Round(time.Millisecond)))
			assertCommitDocuments(ctx, 0)
		})

		t.Run("one commit, two branches", func(t *ftt.Test) {
			ctx, client, importer := prepareEnvironment()

			refs := map[string]string{
				"refs/heads/main":    "0000000000000000000000000000000000000000",
				"refs/heads/release": "0000000000000000000000000000000000000000",
			}
			commits := []*git.Commit{
				{
					Id: "0000000000000000000000000000000000000000",
					Message: `Commit message

Bug: 123
Change-Id: Ifoo
Cr-Commit-Position: refs/heads/main@{#1}`,
				},
			}
			client.SetRepository("bar", refs, commits)
			err := importer.Run(ctx)
			assert.Loosely(t, err, should.BeNil)

			datastore.Get(ctx, doc)
			assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
			assert.Loosely(t, doc.FullScanLastRun, should.Match(clock.Get(ctx).Now().UTC().Round(time.Millisecond)))
			docs := assertCommitDocuments(ctx, 1)
			assert.Loosely(t, docs[0].PositionRef, should.Equal("refs/heads/main"))
			assert.Loosely(t, docs[0].PositionNumber, should.Equal(1))
		})

		t.Run("not indexed branch", func(t *ftt.Test) {
			ctx, client, importer := prepareEnvironment()

			refs := map[string]string{
				"refs/for/refs/heads/main": "5",
				"refs/heads/main":          "2",
			}
			commits := make([]*git.Commit, 5)
			for i := 0; i < 5; i++ {
				commits[i] = &git.Commit{
					Id: strconv.Itoa(i + 1),
				}
				if i > 0 {
					commits[i].Parents = []string{commits[i-1].GetId()}
				}
			}
			client.SetRepository("bar", refs, commits)
			err := importer.Run(ctx)
			assert.Loosely(t, err, should.BeNil)

			datastore.Get(ctx, doc)
			assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
			assert.Loosely(t, doc.FullScanLastRun, should.Match(clock.Get(ctx).Now().UTC().Round(time.Millisecond)))
			assertCommitDocuments(ctx, 2)
		})

		t.Run("diverged branches", func(t *ftt.Test) {
			ctx, client, importer := prepareEnvironment()

			refs := map[string]string{
				"refs/heads/main":    "5",
				"refs/heads/release": "2",
			}
			commits := make([]*git.Commit, 5)
			for i := 0; i < 5; i++ {
				commits[i] = &git.Commit{
					Id: strconv.Itoa(i + 1),
				}
				if i > 0 {
					commits[i].Parents = []string{commits[i-1].GetId()}
				}
			}
			client.SetRepository("bar", refs, commits)
			err := importer.Run(ctx)
			assert.Loosely(t, err, should.BeNil)

			datastore.Get(ctx, doc)
			assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
			assert.Loosely(t, doc.FullScanLastRun, should.Match(clock.Get(ctx).Now().UTC().Round(time.Millisecond)))
			assertCommitDocuments(ctx, 5)
		})

		t.Run("Log commit caching", func(t *ftt.Test) {
			ctx, client, importer := prepareEnvironment()

			// Require two Log pages for each branch
			refs := map[string]string{
				"refs/heads/main":    strconv.Itoa(gitilesLogPageSize + 2),
				"refs/heads/release": strconv.Itoa(gitilesLogPageSize + 1),
			}
			commits := make([]*git.Commit, gitilesLogPageSize+2)
			for i := 0; i < gitilesLogPageSize+2; i++ {
				commits[i] = &git.Commit{
					Id: strconv.Itoa(i + 1),
				}
				if i > 0 {
					commits[i].Parents = []string{commits[i-1].GetId()}
				}
			}
			client.SetRepository("bar", refs, commits)
			err := importer.Run(ctx)
			assert.Loosely(t, err, should.BeNil)

			datastore.Get(ctx, doc)
			assert.Loosely(t, doc.FullScanLeaseStartTime, should.Match(time.Time{}))
			assert.Loosely(t, doc.FullScanLastRun, should.Match(clock.Get(ctx).Now().UTC().Round(time.Millisecond)))
			assertCommitDocuments(ctx, gitilesLogPageSize+2)
			// We expect 4 calls.
			// One is for listing all refs and their revisions.
			// Very first indexed branch should make two calls to
			// Gitiles, and the last should make only one.
			assert.Loosely(t, len(client.GetCallLogs()), should.Equal(4))
		})
	})
}
