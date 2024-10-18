// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package redirect

import (
	"context"
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/appengine/cr-rev/config"
	"infra/appengine/cr-rev/models"
)

func redirectTestSetup() context.Context {
	ctx := gaetesting.TestingContext()
	ds := datastore.GetTestable(ctx)
	ds.Consistent(true)
	ds.AutoIndex(true)
	return ctx
}

func TestRedirects(t *testing.T) {
	r := NewRules(NewGitilesRedirect())
	ftt.Run("generic redirect", t, func(t *ftt.Test) {
		ctx := redirectTestSetup()
		commits := []*models.Commit{
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000000001",
				CommitHash:     "0000000000000000000000000000000000000001",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 1,
				PositionRef:    "svn://svn.chromium.org/chrome",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000000002",
				CommitHash:     "0000000000000000000000000000000000000002",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 2,
				PositionRef:    "svn://svn.chromium.org/chrome/trunk",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000000022",
				CommitHash:     "0000000000000000000000000000000000000022",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 2,
				PositionRef:    "svn://svn.chromium.org/blink",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000000222",
				CommitHash:     "0000000000000000000000000000000000000222",
				Host:           "chromium",
				Repository:     "foo",
				PositionNumber: 2,
				PositionRef:    "refs/heads/master",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000000003",
				CommitHash:     "0000000000000000000000000000000000000003",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 3,
				PositionRef:    "svn://svn.chromium.org/blink",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000288197",
				CommitHash:     "0000000000000000000000000000000000288197",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 288197,
				PositionRef:    "svn://svn.chromium.org/chrome/trunk/src",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000291560",
				CommitHash:     "0000000000000000000000000000000000291560",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 291560,
				PositionRef:    "refs/heads/master",
			},
			{
				ID:             "chromium-codesearch/chromium/src-0000000000000000000000000000000000291560",
				CommitHash:     "0000000000000000000000000000000000291560",
				Host:           "chromium",
				Repository:     "codesearch/chromium/src",
				PositionNumber: 291560,
				PositionRef:    "refs/heads/master",
			},
			{
				ID:             "chromium-codesearch/chromium/src-0000000000000000000000000000000000291561",
				CommitHash:     "0000000000000000000000000000000000291561",
				Host:           "chromium",
				Repository:     "codesearch/chromium/src",
				PositionNumber: 291561,
				PositionRef:    "refs/heads/master",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000291562",
				CommitHash:     "0000000000000000000000000000000000291562",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 291562,
				PositionRef:    "refs/heads/main",
			},
			{
				ID:             "chromium-chromium/src-0000000000000000000000000000000000291563",
				CommitHash:     "0000000000000000000000000000000000291563",
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionNumber: 291563,
				PositionRef:    "refs/heads/feature",
			},
		}
		datastore.Put(ctx, commits)

		t.Run("svn position style", func(t *ftt.Test) {
			t.Run("release ", func(t *ftt.Test) {
				url, commit, err := r.FindRedirectURL(ctx, "/1")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, commit, should.Match(commits[0]))
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000000001"))
			})

			t.Run("trunk", func(t *ftt.Test) {
				url, commit, err := r.FindRedirectURL(ctx, "/2")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, commit, should.Match(commits[1]))
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000000002"))
			})

			t.Run("trunk src", func(t *ftt.Test) {
				url, commit, err := r.FindRedirectURL(ctx, "/288197")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, commit, should.Match(commits[5]))
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000288197"))
			})

			t.Run("non chromium", func(t *ftt.Test) {
				_, _, err := r.FindRedirectURL(ctx, "/3")
				assert.Loosely(t, err, should.Equal(ErrNoMatch))
			})
		})

		t.Run("git numberer", func(t *ftt.Test) {
			t.Run("with mirror", func(t *ftt.Test) {
				url, commit, err := r.FindRedirectURL(ctx, "/291560")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, commit, should.Match(commits[6]))
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000291560"))
			})

			t.Run("not chromium repo, redirect to short hash", func(t *ftt.Test) {
				url, _, err := r.FindRedirectURL(ctx, "/291561")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/291561"))
			})

			t.Run("main branch", func(t *ftt.Test) {
				url, commit, err := r.FindRedirectURL(ctx, "/291562")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, commit, should.Match(commits[9]))
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000291562"))
			})

			t.Run("non default branch, redirect to short hash", func(t *ftt.Test) {
				url, _, err := r.FindRedirectURL(ctx, "/291563")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/291563"))
			})
		})
		t.Run("git numberer log", func(t *ftt.Test) {
			t.Run("existing commits", func(t *ftt.Test) {
				url, _, err := r.FindRedirectURL(ctx, "/291560..291562")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+log/0000000000000000000000000000000000291560..0000000000000000000000000000000000291562"))
			})

			t.Run("commit missing", func(t *ftt.Test) {
				_, _, err := r.FindRedirectURL(ctx, "/291..292")
				assert.Loosely(t, err, should.Equal(ErrNoMatch))
			})
		})
		t.Run("with path", func(t *ftt.Test) {
			url, _, err := r.FindRedirectURL(ctx, "/291560/foo/bar")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000291560/foo/bar"))
		})

		t.Run("full diff", func(t *ftt.Test) {
			t.Run("existing commits", func(t *ftt.Test) {
				url, _, err := r.FindRedirectURL(ctx, "/0000000000000000000000000000000000291560..0000000000000000000000000000000000291562")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000291560..0000000000000000000000000000000000291562"))
			})

			t.Run("commit missing", func(t *ftt.Test) {
				_, _, err := r.FindRedirectURL(ctx, "/0000000000000000000000000000000000291560..0000000000000000000000000000000000291561")
				assert.Loosely(t, err, should.Equal(ErrNoMatch))
			})
		})

	})

	ftt.Run("full hash redirect", t, func(t *ftt.Test) {
		ctx := redirectTestSetup()
		commits := []*models.Commit{
			{
				ID:         "foo-bar-0000000000000000000000000000000000000000",
				CommitHash: "0000000000000000000000000000000000000000",
				Host:       "foo",
				Repository: "bar",
			},
			{
				ID:         "foo-bar/mirror-0000000000000000000000000000000000000000",
				CommitHash: "0000000000000000000000000000000000000000",
				Host:       "foo",
				Repository: "bar/mirror",
			},
			{
				ID:         "foo-baz-0000000000000000000000000000000000000001",
				CommitHash: "0000000000000000000000000000000000000001",
				Host:       "foo",
				Repository: "baz",
			},
			{
				ID:         "foo-baz/mirror-0000000000000000000000000000000000000001",
				CommitHash: "0000000000000000000000000000000000000001",
				Host:       "foo",
				Repository: "baz/not/indexed",
			},
			{
				ID:         "foo-baz/mirror-0000000000000000000000000000000000000002",
				CommitHash: "0000000000000000000000000000000000000002",
				Host:       "foo",
				Repository: "baz/not/indexed",
			},
		}
		datastore.Put(ctx, commits)

		err := config.Override(ctx, &config.Config{
			Hosts: []*config.Host{
				{
					Name: "foo",
					Repos: []*config.Repository{
						{
							Name:     "bar",
							Indexing: &config.Repository_Priority{Priority: true},
						},
						{
							Name:     "baz/not/indexed",
							Indexing: &config.Repository_DoNotIndex{DoNotIndex: true},
						},
					},
				},
			},
		})
		if err != nil {
			panic(fmt.Sprintf("Could not set context: %s", err.Error()))
		}

		t.Run("respect priority", func(t *ftt.Test) {
			url, commit, err := r.FindRedirectURL(
				ctx, "/0000000000000000000000000000000000000000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, commit, should.Match(commits[0]))
			assert.Loosely(t, url, should.Equal("https://foo.googlesource.com/bar/+/0000000000000000000000000000000000000000"))
		})

		t.Run("avoid not indexed", func(t *ftt.Test) {
			url, commit, err := r.FindRedirectURL(
				ctx, "/0000000000000000000000000000000000000001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, commit, should.Match(commits[2]))
			assert.Loosely(t, url, should.Equal("https://foo.googlesource.com/baz/+/0000000000000000000000000000000000000001"))
		})

		t.Run("redirect to not indexed if only one", func(t *ftt.Test) {
			url, commit, err := r.FindRedirectURL(
				ctx, "/0000000000000000000000000000000000000002")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, commit, should.Match(commits[4]))
			assert.Loosely(t, url, should.Equal("https://foo.googlesource.com/baz/not/indexed/+/0000000000000000000000000000000000000002"))
		})

		t.Run("with path", func(t *ftt.Test) {
			url, commit, err := r.FindRedirectURL(
				ctx, "/0000000000000000000000000000000000000000/foo/bar")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, commit, should.Match(commits[0]))
			assert.Loosely(t, url, should.Equal("https://foo.googlesource.com/bar/+/0000000000000000000000000000000000000000/foo/bar"))
		})

		t.Run("commit not found", func(t *ftt.Test) {
			url, _, err := r.FindRedirectURL(
				ctx, "/FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"))
		})
	})

	ftt.Run("short hash redirect", t, func(t *ftt.Test) {
		ctx := redirectTestSetup()
		t.Run("without path", func(t *ftt.Test) {
			url, _, err := r.FindRedirectURL(
				ctx, "/000000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/000000"))
		})
		t.Run("without path, max int", func(t *ftt.Test) {
			// this is max int, before it starts conflicting with
			// rietveld IDs
			url, _, err := r.FindRedirectURL(
				ctx, "/99999999")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/99999999"))
		})
		t.Run("with path", func(t *ftt.Test) {
			url, _, err := r.FindRedirectURL(
				ctx, "/000fff/foo/bar")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/000fff/foo/bar"))
		})
	})

	ftt.Run("short diff redirect", t, func(t *ftt.Test) {
		ctx := redirectTestSetup()
		t.Run("without path", func(t *ftt.Test) {
			url, _, err := r.FindRedirectURL(
				ctx, "/000000..000001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://chromium.googlesource.com/chromium/src/+/000000..000001"))
		})
	})

	ftt.Run("rietveld redirect", t, func(t *ftt.Test) {
		ctx := redirectTestSetup()
		t.Run("without path", func(t *ftt.Test) {
			url, _, err := r.FindRedirectURL(
				ctx, "/784093002")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("https://codereview.chromium.org/784093002"))
		})
	})

	ftt.Run("default not found", t, func(t *ftt.Test) {
		ctx := redirectTestSetup()
		_, _, err := r.FindRedirectURL(
			ctx, "/foo")
		assert.Loosely(t, err, should.Equal(ErrNoMatch))
	})
}
