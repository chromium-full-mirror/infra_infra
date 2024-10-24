// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cas

import (
	"context"
	"testing"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	bscas "infra/chromium/bootstrapper/clients/cas"
	"infra/chromium/util"
)

func TestFactory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Factory", t, func(t *ftt.Test) {

		t.Run("succeeds by default", func(t *ftt.Test) {
			factory := Factory(nil)

			client, err := factory(ctx, "fake-instance")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, client, should.NotBeNil)
		})

		t.Run("fails for a nil instance", func(t *ftt.Test) {
			factory := Factory(map[string]*Instance{
				"fake-instance": nil,
			})

			client, err := factory(ctx, "fake-instance")

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, client, should.BeNil)
		})

		t.Run("succeeds for provided instance", func(t *ftt.Test) {
			factory := Factory(map[string]*Instance{
				"fake-instance": {},
			})

			client, err := factory(ctx, "fake-instance")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, client, should.NotBeNil)
		})

	})

}

func TestDownload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("casClient.Download", t, func(t *ftt.Test) {

		execRoot := t.TempDir()

		d := digest.Digest{
			Hash: "fake-hash",
			Size: 42,
		}

		cache := filemetadata.NewNoopCache()

		t.Run("succeeds by default", func(t *ftt.Test) {
			client, err := Factory(nil)(ctx, "fake-instance")
			util.PanicOnError(err)

			_, _, err = client.DownloadDirectory(ctx, d, execRoot, cache)

			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("fails for a false blob", func(t *ftt.Test) {
			client, err := Factory(map[string]*Instance{
				"fake-instance": {
					Blobs: map[string]bool{
						"fake-hash": false,
					},
				},
			})(ctx, "fake-instance")
			util.PanicOnError(err)

			_, _, err = client.DownloadDirectory(ctx, d, execRoot, cache)

			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("succeeds for a true blob", func(t *ftt.Test) {
			client, err := Factory(map[string]*Instance{
				"fake-instance": {
					Blobs: map[string]bool{
						"fake-hash": true,
					},
				},
			})(ctx, "fake-instance")
			util.PanicOnError(err)

			_, _, err = client.DownloadDirectory(ctx, d, execRoot, cache)

			assert.Loosely(t, err, should.BeNil)
		})

	})
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("package using fake CAS client", t, func(t *ftt.Test) {

		ctx := bscas.UseCasClientFactory(ctx, Factory(nil))

		t.Run("succeeds when calling Download", func(t *ftt.Test) {
			client := bscas.NewClient(ctx)

			err := client.Download(ctx, "fake-out-dir", "fake-instance", &apipb.Digest{
				Hash:      "fake-hash",
				SizeBytes: 42,
			})

			assert.Loosely(t, err, should.BeNil)
		})

	})
}
