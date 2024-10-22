// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package models

import (
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
)

func TestRepoID(t *testing.T) {
	ftt.Run("ToProperty", t, func(t *ftt.Test) {
		t.Run("nothing set", func(t *ftt.Test) {
			rID := &RepoID{}
			_, err := rID.ToProperty()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("host only", func(t *ftt.Test) {
			rID := &RepoID{
				Host: "foo",
			}
			_, err := rID.ToProperty()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("repository only", func(t *ftt.Test) {
			rID := &RepoID{
				Repository: "foo",
			}
			_, err := rID.ToProperty()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("minimal name", func(t *ftt.Test) {
			rID := &RepoID{
				Host:       "f",
				Repository: "b",
			}
			ret, err := rID.ToProperty()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ret.Value(), should.Equal("f/b"))
		})
		t.Run("with subrepos", func(t *ftt.Test) {
			rID := &RepoID{
				Host:       "foo.example.org",
				Repository: "bar/baz/qux",
			}
			ret, err := rID.ToProperty()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ret.Value(), should.Equal("foo.example.org/bar/baz/qux"))
		})
	})

	ftt.Run("FromProperty", t, func(t *ftt.Test) {
		p := datastore.Property{}
		t.Run("nothing set", func(t *ftt.Test) {
			p.SetValue("", true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, rID.Host, should.BeZero)
			assert.Loosely(t, rID.Repository, should.BeZero)
		})
		t.Run("host only", func(t *ftt.Test) {
			p.SetValue("foo", true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, rID.Host, should.BeZero)
			assert.Loosely(t, rID.Repository, should.BeZero)
		})
		t.Run("host only with slash", func(t *ftt.Test) {
			p.SetValue("foo/", true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, rID.Host, should.BeZero)
			assert.Loosely(t, rID.Repository, should.BeZero)
		})
		t.Run("repository only", func(t *ftt.Test) {
			p.SetValue("/foo", true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, rID.Host, should.BeZero)
			assert.Loosely(t, rID.Repository, should.BeZero)
		})
		t.Run("minimal valid name", func(t *ftt.Test) {
			p.SetValue("f/b", true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rID.Host, should.Equal("f"))
			assert.Loosely(t, rID.Repository, should.Equal("b"))
		})
		t.Run("with subrepos", func(t *ftt.Test) {
			p.SetValue("foo.example.org/bar/baz/qux", true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rID.Host, should.Equal("foo.example.org"))
			assert.Loosely(t, rID.Repository, should.Equal("bar/baz/qux"))
		})
		t.Run("invalid type", func(t *ftt.Test) {
			p.SetValue(42, true)
			rID := &RepoID{}
			err := rID.FromProperty(p)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, rID.Host, should.BeZero)
			assert.Loosely(t, rID.Repository, should.BeZero)
		})
	})
}

func TestRepository(t *testing.T) {
	r := &Repository{}
	ftt.Run("indexing", t, func(t *ftt.Test) {
		currentTime := time.Now()
		t.Run("not started", func(t *ftt.Test) {
			assert.Loosely(t,
				r.IsScanRequired(currentTime),
				should.BeTrue)
		})

		t.Run("started", func(t *ftt.Test) {
			r.SetStartIndexing(currentTime, "hostname")
			assert.Loosely(t,
				r.IsScanRequired(currentTime),
				should.BeFalse)
		})

		t.Run("expired", func(t *ftt.Test) {
			indexingTime := currentTime.Add(
				(RepositoryStaleIndexingDuration + time.Second) * -1)
			r.SetStartIndexing(indexingTime, "hostname")
			assert.Loosely(t,
				r.IsScanRequired(currentTime),
				should.BeTrue)
		})

		t.Run("completed", func(t *ftt.Test) {
			indexingTime := currentTime.Add(
				(RepositoryStaleIndexingDuration + time.Second) * -1)
			r.SetStartIndexing(indexingTime, "hostname")
			r.SetIndexingCompleted(currentTime)
			assert.Loosely(t,
				r.IsScanRequired(currentTime),
				should.BeFalse)
		})
	})

}
