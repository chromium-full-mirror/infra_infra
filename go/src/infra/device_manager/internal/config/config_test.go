// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"os"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/secrets"
)

func TestGetSecret(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := &secrets.SecretManagerStore{
		CloudProject: "test-project",
	}
	ctx = secrets.Use(ctx, store)

	ftt.Run("Test GetSecret", t, func(t *ftt.Test) {
		t.Run("success - valid secret location in base64", func(t *ftt.Test) {
			secret, err := GetSecret(ctx, "devsecret://cGFzc3dvcmQ")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, secret, should.Equal("password"))
		})
		t.Run("fail - invalid secret location with padding", func(t *ftt.Test) {
			secret, err := GetSecret(ctx, "devsecret://cGFzc3dvcmQ=")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("not base64 encoding"))
			assert.Loosely(t, secret, should.BeEmpty)
		})
	})
}

func TestGetEnvVar(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Test GetEnvVar", t, func(t *ftt.Test) {
		t.Run("success - env var found", func(t *ftt.Test) {
			err := os.Setenv("FOO", "BAR")
			assert.Loosely(t, err, should.BeNil)

			v, err := GetEnvVar(ctx, "FOO")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, v, should.Equal("BAR"))

			err = os.Setenv("FOO", "")
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("fail - env var not found", func(t *ftt.Test) {
			v, err := GetEnvVar(ctx, "NO_FOO")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("environment variable not set"))
			assert.Loosely(t, v, should.BeEmpty)
		})
	})
}
