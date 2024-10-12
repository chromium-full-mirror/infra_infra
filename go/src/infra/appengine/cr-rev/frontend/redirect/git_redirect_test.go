// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package redirect

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/appengine/cr-rev/models"
)

func TestGitilesRedirect(t *testing.T) {
	commit := models.Commit{
		Host:       "foo",
		Repository: "bar/baz",
		CommitHash: "1234567890123456789012345678901234567890",
	}
	redirect := &gitilesRedirect{}
	ftt.Run("Commit redirect", t, func(t *ftt.Test) {
		t.Run("no path provided", func(t *ftt.Test) {
			url, err := redirect.Commit(commit, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://foo.googlesource.com/bar/baz/+/1234567890123456789012345678901234567890",
				))
		})

		t.Run("path provided", func(t *ftt.Test) {
			url, err := redirect.Commit(commit, "README.md")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://foo.googlesource.com/bar/baz/+/1234567890123456789012345678901234567890/README.md",
				))
		})
	})

	ftt.Run("Diff redirect", t, func(t *ftt.Test) {
		t.Run("identical repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "foo",
				Repository: "bar/baz",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			url, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://foo.googlesource.com/bar/baz/+/1234567890123456789012345678901234567890..0000000000000000000000000000000000000000",
				))
		})

		t.Run("different repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "foo",
				Repository: "bar/baz/baq",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			_, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotIdenticalRepositories))
		})

		t.Run("different host repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "bar",
				Repository: "bar/baz",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			_, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotIdenticalRepositories))
		})
	})

	ftt.Run("Log redirect", t, func(t *ftt.Test) {
		t.Run("identical repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "foo",
				Repository: "bar/baz",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			url, err := redirect.Log(commit, commit2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://foo.googlesource.com/bar/baz/+log/1234567890123456789012345678901234567890..0000000000000000000000000000000000000000",
				))
		})

		t.Run("different repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "foo",
				Repository: "bar/baz/baq",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			_, err := redirect.Log(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotIdenticalRepositories))
		})

		t.Run("different host repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "bar",
				Repository: "bar/baz",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			_, err := redirect.Log(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotIdenticalRepositories))
		})
	})
}

func TestCodesearchRedirect(t *testing.T) {
	redirect := &codesearchRedirect{}
	ftt.Run("Test commit redirect", t, func(t *ftt.Test) {
		commit := models.Commit{
			Host:       "chromium",
			Repository: "bar/baz",
			CommitHash: "1234567890123456789012345678901234567890",
		}
		t.Run("no path provided", func(t *ftt.Test) {
			url, err := redirect.Commit(commit, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://source.chromium.org/chromium/bar/baz/+/1234567890123456789012345678901234567890",
				))
		})

		t.Run("path provided", func(t *ftt.Test) {
			url, err := redirect.Commit(commit, "README.md")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://source.chromium.org/chromium/bar/baz/+/1234567890123456789012345678901234567890:README.md",
				))
		})
	})

	ftt.Run("Diff redirect", t, func(t *ftt.Test) {
		commit := models.Commit{
			Host:       "chromium",
			Repository: "bar/baz",
			CommitHash: "1234567890123456789012345678901234567890",
		}
		t.Run("identical repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "chromium",
				Repository: "bar/baz",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			url, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t,
				url,
				should.Equal(
					"https://source.chromium.org/chromium/bar/baz/+/1234567890123456789012345678901234567890...0000000000000000000000000000000000000000",
				))
		})

		t.Run("different repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "chromium",
				Repository: "bar/baz/baq",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			_, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotIdenticalRepositories))
		})

		t.Run("different host repositories", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "foo",
				Repository: "bar/baz",
				CommitHash: "0000000000000000000000000000000000000000",
			}
			_, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotIdenticalRepositories))
		})
	})

	ftt.Run("Test not supported GoB hosts", t, func(t *ftt.Test) {
		commit := models.Commit{
			Host:       "foo",
			Repository: "bar/baz",
		}
		t.Run("no commit redirect", func(t *ftt.Test) {
			_, err := redirect.Commit(commit, "")
			assert.Loosely(t, err, should.Equal(errNotSupportedRepository))
		})
		t.Run("no diff redirect", func(t *ftt.Test) {
			commit2 := models.Commit{
				Host:       "foo",
				Repository: "bar/baz",
			}
			_, err := redirect.Diff(commit, commit2)
			assert.Loosely(t, err, should.Equal(errNotSupportedRepository))
		})
	})
}
