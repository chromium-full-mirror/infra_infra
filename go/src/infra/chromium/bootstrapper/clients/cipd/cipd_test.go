// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cipd

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

type fakeClient struct {
	ensure func(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]string, error)
}

func (f *fakeClient) Ensure(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]string, error) {
	ensure := f.ensure
	if ensure != nil {
		return ensure(ctx, serviceUrl, cipdRoot, packages)
	}
	return nil, nil
}

func TestEnsure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Ensure", t, func(t *ftt.Test) {

		cipdRoot := t.TempDir()

		t.Run("fails if provided empty service URL", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "", cipdRoot, map[string]*Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike("empty serviceUrl"))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if provided empty CIPD root", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "fake-url", "", map[string]*Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike("empty cipdRoot"))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if provided empty packages", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, nil)

			assert.Loosely(t, err, should.ErrLike("empty packages"))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if provided empty subdir", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, map[string]*Package{
				"": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike("empty subdir in packages"))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if provided nil package", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, map[string]*Package{
				"fake-subdir": nil,
			})

			assert.Loosely(t, err, should.ErrLike(`nil package for subdir "fake-subdir"`))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if provided empty package name", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, map[string]*Package{
				"fake-subdir": {
					Name:    "",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike(`empty package name for subdir "fake-subdir"`))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if provided empty package version", func(t *ftt.Test) {
			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, map[string]*Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "",
				},
			})

			assert.Loosely(t, err, should.ErrLike(`empty package version for subdir "fake-subdir"`))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("fails if ensuring packages fails", func(t *ftt.Test) {
			factory := func(ctx context.Context) Client {
				return &fakeClient{ensure: func(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]string, error) {
					return nil, errors.New("test Ensure failure")
				}}
			}
			ctx := UseClientFactory(ctx, factory)

			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, map[string]*Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.ErrLike("test Ensure failure"))
			assert.Loosely(t, resolvedPackages, should.BeNil)
		})

		t.Run("returns resolved package information on success", func(t *ftt.Test) {
			factory := func(ctx context.Context) Client {
				return &fakeClient{ensure: func(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]string, error) {
					return map[string]string{"fake-subdir": "fake-instance-id"}, nil
				}}
			}
			ctx := UseClientFactory(ctx, factory)

			resolvedPackages, err := Ensure(ctx, "fake-url", cipdRoot, map[string]*Package{
				"fake-subdir": {
					Name:    "fake-package",
					Version: "fake-version",
				},
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resolvedPackages, should.Resemble(map[string]*ResolvedPackage{
				"fake-subdir": {
					Name:             "fake-package",
					RequestedVersion: "fake-version",
					ActualVersion:    "fake-instance-id",
				},
			}))
		})

	})
}

func TestUnmarshalEnsureJsonOut(t *testing.T) {

	ftt.Run("unmarshallEnsureJsonOut decodes valid ensure json out", t, func(t *ftt.Test) {
		jsonOutContents := []byte(`{
			"result": {
				"exe": [
					{
						"package": "fake-package",
						"instance_id": "fake-instance-id"
					}
				]
			}
		}`)

		out, err := unmarshalEnsureJsonOut(jsonOutContents)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Resemble(&jsonOut{
			Result: map[string][]jsonPackage{
				"exe": {
					{
						Package:    "fake-package",
						InstanceId: "fake-instance-id",
					},
				},
			},
		}))

	})
}
