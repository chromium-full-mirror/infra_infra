// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cipd

import (
	"context"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/testfs"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	real "go.chromium.org/infra/chromium/bootstrapper/clients/cipd"
	"go.chromium.org/infra/chromium/util"
)

func collect(cipdRoot, subdir string) map[string]string {
	layout, err := testfs.Collect(filepath.Join(cipdRoot, subdir))
	util.PanicOnError(err)
	return layout
}

func TestEnsure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Client.Ensure", t, func(t *ftt.Test) {
		cipdRoot := t.TempDir()

		t.Run("returns pin for a package by default", func(t *ftt.Test) {
			client := Client{}

			packageVersions, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packageVersions, should.ContainKey("fake-subdir"))
			assert.Loosely(t, packageVersions["fake-subdir"], should.NotBeEmpty)
		})

		t.Run("fails for a nil package", func(t *ftt.Test) {
			client := Client{map[string]*Package{
				"fake-package": nil,
			}}

			packageVersions, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike(`unknown package "fake-package"`))
			assert.Loosely(t, packageVersions, should.BeNil)
		})

		t.Run("fails for an a version mapping to an empty instance ID", func(t *ftt.Test) {
			client := Client{map[string]*Package{
				"fake-package": {
					Refs: map[string]string{
						"fake-version": "",
					},
				},
			}}

			packageVersions, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike(`unknown version "fake-version" of package "fake-package"`))
			assert.Loosely(t, packageVersions, should.BeNil)
		})

		t.Run("returns pin for version mapping to provided instance ID", func(t *ftt.Test) {
			client := Client{map[string]*Package{
				"fake-package": {
					Refs: map[string]string{
						"fake-version": "fake-instance-id",
					},
				},
			}}

			packageVersions, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packageVersions, should.ContainKey("fake-subdir"))
			assert.Loosely(t, packageVersions["fake-subdir"], should.Equal("fake-instance-id"))
		})

		t.Run("fails for a non-existent instance ID", func(t *ftt.Test) {
			client := Client{map[string]*Package{
				"fake-package": {
					Instances: map[string]*PackageInstance{
						"fake-instance-id": nil,
					},
				},
			}}

			packageVersions, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-instance-id",
				},
			})

			assert.Loosely(t, err, should.ErrLike(`unknown version "fake-instance-id" of package "fake-package"`))
			assert.Loosely(t, packageVersions, should.BeNil)
		})

		t.Run("returns pin for instance ID", func(t *ftt.Test) {
			client := Client{map[string]*Package{
				"fake-package": {
					Instances: map[string]*PackageInstance{
						"fake-instance-id": {},
					},
				},
			}}

			packageVersions, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-instance-id",
				},
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packageVersions, should.ContainKey("fake-subdir"))
			assert.Loosely(t, packageVersions["fake-subdir"], should.Equal("fake-instance-id"))
		})

		t.Run("deploys specified files", func(t *ftt.Test) {
			client := Client{map[string]*Package{
				"fake-package": {
					Instances: map[string]*PackageInstance{
						"fake-instance-id": {
							Contents: map[string]string{
								"infra/config/recipes.cfg": "fake-recipes.cfg",
								"recipes/foo.py":           "fake-recipe-foo",
							},
						},
					},
				},
			}}

			_, err := client.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-instance-id",
				},
			})

			assert.Loosely(t, err, should.BeNil)
			layout := collect(cipdRoot, "fake-subdir")
			assert.Loosely(t, layout, should.Resemble(map[string]string{
				"infra/config/recipes.cfg": "fake-recipes.cfg",
				"recipes/foo.py":           "fake-recipe-foo",
			}))
		})

	})
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("CIPD using fake factory", t, func(t *ftt.Test) {

		cipdRoot := t.TempDir()

		ctx := real.UseClientFactory(ctx, Factory(nil))

		t.Run("succeeds when calling EnsurePackages", func(t *ftt.Test) {
			packages, err := real.Ensure(ctx, "fake-url", cipdRoot, map[string]*real.Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packages, should.ContainKey("fake-subdir"))
			pkg := packages["fake-subdir"]
			assert.Loosely(t, pkg.Name, should.Equal("fake-package"))
			assert.Loosely(t, pkg.RequestedVersion, should.Equal("fake-version"))
			assert.Loosely(t, pkg.ActualVersion, should.NotBeEmpty)
		})

	})
}
