// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manifest

import (
	"testing"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/flag/stringsetflag"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCheckTargetName(t *testing.T) {
	t.Parallel()

	ftt.Run("Unrestricted", t, func(t *ftt.Test) {
		r := Restrictions{}
		assert.Loosely(t, r.CheckTargetName("blah"), should.BeNil)
	})

	ftt.Run("Restricted", t, func(t *ftt.Test) {
		r := Restrictions{
			targets: stringsetflag.Flag{Data: stringset.NewFromSlice("some-prefix/")},
		}
		assert.Loosely(t, r.CheckTargetName("some-prefix/zzz"), should.BeNil)
		assert.Loosely(t, r.CheckTargetName("another"), should.Resemble([]string{
			`forbidden target name "another" (allowed prefixes are ["some-prefix/"])`,
		}))
	})
}

func TestCheckBuildSteps(t *testing.T) {
	t.Parallel()

	ftt.Run("Unrestricted", t, func(t *ftt.Test) {
		r := Restrictions{}
		assert.Loosely(t, r.CheckBuildSteps([]*BuildStep{
			{concrete: &CopyBuildStep{}},
			{concrete: &GoBuildStep{}},
			{concrete: &RunBuildStep{}},
			{concrete: &GoGAEBundleBuildStep{}},
		}), should.BeNil)
	})

	ftt.Run("Restricted", t, func(t *ftt.Test) {
		r := Restrictions{
			steps: stringsetflag.Flag{Data: stringset.NewFromSlice("copy", "go_gae_bundle")},
		}
		assert.Loosely(t, r.CheckBuildSteps([]*BuildStep{
			{concrete: &CopyBuildStep{}},
			{concrete: &GoGAEBundleBuildStep{}},
		}), should.BeNil)
		assert.Loosely(t, r.CheckBuildSteps([]*BuildStep{
			{concrete: &CopyBuildStep{}},
			{concrete: &GoBuildStep{}},
			{concrete: &RunBuildStep{}},
			{concrete: &GoGAEBundleBuildStep{}},
		}), should.Resemble([]string{
			`forbidden build step kind "go_binary" (allowed values are ["copy" "go_gae_bundle"])`,
			`forbidden build step kind "run" (allowed values are ["copy" "go_gae_bundle"])`,
		}))
	})
}

func TestCheckInfra(t *testing.T) {
	t.Parallel()

	call := func(infra Infra, storage, registry, notifications []string) []string {
		r := Restrictions{
			storage:       stringsetflag.Flag{Data: stringset.NewFromSlice(storage...)},
			registry:      stringsetflag.Flag{Data: stringset.NewFromSlice(registry...)},
			notifications: stringsetflag.Flag{Data: stringset.NewFromSlice(notifications...)},
		}
		return r.CheckInfra(&infra)
	}

	infra := Infra{
		Storage:  "gs://something/a/b/c",
		Registry: "gcr.io/something",
		Notify: []NotifyConfig{
			{
				Kind:   "git",
				Repo:   "https://repo.example.com/something",
				Script: "some/script.py",
			},
		},
	}

	ftt.Run("Unrestricted", t, func(t *ftt.Test) {
		violations := call(infra, nil, nil, nil)
		assert.Loosely(t, violations, should.BeEmpty)
	})

	ftt.Run("Passing via direct hits", t, func(t *ftt.Test) {
		violations := call(infra,
			[]string{"gs://something/a/b/c", "gs://something/else"},
			[]string{"gcr.io/something", "gcr.io/else"},
			[]string{"git:https://repo.example.com/something/some/script.py"},
		)
		assert.Loosely(t, violations, should.BeEmpty)
	})

	ftt.Run("Passing via prefix hits", t, func(t *ftt.Test) {
		violations := call(infra,
			[]string{"gs://something/", "gs://something/else"},
			[]string{"gcr.io/something"},
			[]string{"git:https://repo.example.com/something/"},
		)
		assert.Loosely(t, violations, should.BeEmpty)
	})

	ftt.Run("Bad storage", t, func(t *ftt.Test) {
		violations := call(infra, []string{"gs://allowed"}, nil, nil)
		assert.Loosely(t, violations, should.Resemble([]string{
			`forbidden Google Storage destination "gs://something/a/b/c" (allowed prefixes are ["gs://allowed"])`,
		}))
	})

	ftt.Run("Bad registry", t, func(t *ftt.Test) {
		violations := call(infra, nil, []string{"gcr.io/some"}, nil)
		assert.Loosely(t, violations, should.Resemble([]string{
			`forbidden Container Registry destination "gcr.io/something" (allowed values are ["gcr.io/some"])`,
		}))
	})

	ftt.Run("Bad notify config", t, func(t *ftt.Test) {
		violations := call(infra, nil, nil, []string{"git:https://another"})
		assert.Loosely(t, violations, should.Resemble([]string{
			`forbidden notification destination "git:https://repo.example.com/something/some/script.py" (allowed prefixes are ["git:https://another"])`,
		}))
	})
}

func TestCheckCloudBuild(t *testing.T) {
	t.Parallel()

	call := func(cb CloudBuildBuilder, build []string) []string {
		r := Restrictions{
			build: stringsetflag.Flag{Data: stringset.NewFromSlice(build...)},
		}
		return r.CheckCloudBuild(&cb)
	}

	cfg := CloudBuildBuilder{
		Project: "some-project",
	}

	ftt.Run("Unrestricted", t, func(t *ftt.Test) {
		violations := call(cfg, nil)
		assert.Loosely(t, violations, should.BeEmpty)
	})

	ftt.Run("Passing via direct hits", t, func(t *ftt.Test) {
		violations := call(cfg, []string{"another-project", "some-project"})
		assert.Loosely(t, violations, should.BeEmpty)
	})

	ftt.Run("Passing via prefix hits", t, func(t *ftt.Test) {
		violations := call(cfg, []string{"some-project"})
		assert.Loosely(t, violations, should.BeEmpty)
	})

	ftt.Run("Bad project", t, func(t *ftt.Test) {
		violations := call(cfg, []string{"some"})
		assert.Loosely(t, violations, should.Resemble([]string{
			`forbidden Cloud Build project "some-project" (allowed values are ["some"])`,
		}))
	})
}
