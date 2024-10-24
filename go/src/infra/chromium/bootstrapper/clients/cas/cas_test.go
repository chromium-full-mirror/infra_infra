// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cas

import (
	"context"
	"errors"
	"testing"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

type fakeCasClient struct {
	downloadDirectory func(ctx context.Context, d digest.Digest, execRoot string, cache filemetadata.Cache) (map[string]*client.TreeOutput, *client.MovedBytesMetadata, error)
}

func (f *fakeCasClient) DownloadDirectory(ctx context.Context, d digest.Digest, outDir string, cache filemetadata.Cache) (map[string]*client.TreeOutput, *client.MovedBytesMetadata, error) {
	downloadDirectory := f.downloadDirectory
	if downloadDirectory != nil {
		return downloadDirectory(ctx, d, outDir, cache)
	}
	return nil, nil, nil
}

func TestClientForHost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Client.clientForInstance", t, func(t *ftt.Test) {

		t.Run("fails if factory fails", func(t *ftt.Test) {
			ctx := UseCasClientFactory(ctx, func(ctx context.Context, instance string) (CasClient, error) {
				return nil, errors.New("test client factory failure")
			})

			client := NewClient(ctx)
			casClient, err := client.clientForInstance(ctx, "fake-instance")

			assert.Loosely(t, err, should.ErrLike("test client factory failure"))
			assert.Loosely(t, casClient, should.BeNil)
		})

		t.Run("returns CAS client from factory", func(t *ftt.Test) {
			fakeClient := &fakeCasClient{}
			ctx := UseCasClientFactory(ctx, func(ctx context.Context, host string) (CasClient, error) {
				return fakeClient, nil
			})

			client := NewClient(ctx)
			casClient, err := client.clientForInstance(ctx, "fake-instance")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, casClient, should.Equal(fakeClient))
		})

		t.Run("re-uses CAS client for instance", func(t *ftt.Test) {
			ctx := UseCasClientFactory(ctx, func(ctx context.Context, host string) (CasClient, error) {
				return &fakeCasClient{}, nil
			})

			client := NewClient(ctx)
			casClientFoo1, _ := client.clientForInstance(ctx, "fake-instance-foo")
			casClientFoo2, _ := client.clientForInstance(ctx, "fake-instance-foo")
			casClientBar, _ := client.clientForInstance(ctx, "fake-instance-bar")

			assert.Loosely(t, casClientFoo1, should.NotBeNil)
			assert.Loosely(t, casClientFoo2, should.Equal(casClientFoo1))
			assert.Loosely(t, casClientBar, should.NotEqual(casClientFoo1))
		})

	})
}

func TestDownload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Client.Download", t, func(t *ftt.Test) {

		t.Run("fails if getting client for instance fails", func(t *ftt.Test) {
			ctx := UseCasClientFactory(ctx, func(ctx context.Context, instance string) (CasClient, error) {
				return nil, errors.New("test client factory failure")
			})

			client := NewClient(ctx)
			err := client.Download(ctx, "fake-dir", "fake-instance", &apipb.Digest{
				Hash:      "fake-hash",
				SizeBytes: 42,
			})

			assert.Loosely(t, err, should.ErrLike("test client factory failure"))
		})

		t.Run("fails if downloading directory fails", func(t *ftt.Test) {
			ctx := UseCasClientFactory(ctx, func(ctx context.Context, instance string) (CasClient, error) {
				return &fakeCasClient{
					downloadDirectory: func(ctx context.Context, d digest.Digest, execRoot string, cache filemetadata.Cache) (map[string]*client.TreeOutput, *client.MovedBytesMetadata, error) {
						return nil, nil, errors.New("test DownloadDirectory failure")
					},
				}, nil
			})

			client := NewClient(ctx)
			err := client.Download(ctx, "fake-dir", "fake-instance", &apipb.Digest{
				Hash:      "fake-hash",
				SizeBytes: 42,
			})

			assert.Loosely(t, err, should.ErrLike("test DownloadDirectory failure"))
		})

		t.Run("succeeds if downloading directory succeeds", func(t *ftt.Test) {
			ctx := UseCasClientFactory(ctx, func(ctx context.Context, instance string) (CasClient, error) {
				return &fakeCasClient{}, nil
			})

			client := NewClient(ctx)
			err := client.Download(ctx, "fake-dir", "fake-instance", &apipb.Digest{
				Hash:      "fake-hash",
				SizeBytes: 42,
			})

			assert.Loosely(t, err, should.BeNil)
		})

	})
}
