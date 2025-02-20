// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitiles

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/proto"
	gitpb "go.chromium.org/luci/common/proto/git"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/proto/gitiles/mock_gitiles"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/chromium/bootstrapper/clients/gob"
)

func TestClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ctx = gob.UseTestClock(ctx)

	ftt.Run("Client", t, func(t *ftt.Test) {

		t.Run("gitilesClientForHost", func(t *ftt.Test) {

			t.Run("fails if factory fails", func(t *ftt.Test) {
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return nil, errors.New("fake client factory failure")
				})

				client := NewClient(ctx)
				gitilesClient, err := client.gitilesClientForHost(ctx, "fake-host")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, gitilesClient, should.BeNil)
			})

			t.Run("returns gitiles client from factory", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})

				client := NewClient(ctx)
				gitilesClient, err := client.gitilesClientForHost(ctx, "fake-host")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, gitilesClient, should.Equal(mockGitilesClient))
			})

			t.Run("re-uses gitiles client for host", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mock_gitiles.NewMockGitilesClient(ctl), nil
				})

				client := NewClient(ctx)
				gitilesClientFoo1, _ := client.gitilesClientForHost(ctx, "fake-host-foo")
				gitilesClientFoo2, _ := client.gitilesClientForHost(ctx, "fake-host-foo")
				gitilesClientBar, _ := client.gitilesClientForHost(ctx, "fake-host-bar")

				assert.Loosely(t, gitilesClientFoo1, should.NotBeNil)
				assert.Loosely(t, gitilesClientFoo2, should.Equal(gitilesClientFoo1))
				assert.Loosely(t, gitilesClientBar, should.NotEqual(gitilesClientFoo1))
			})

		})

		t.Run("FetchLatestRevision", func(t *ftt.Test) {

			t.Run("fails if getting gitiles client fails", func(t *ftt.Test) {
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return nil, errors.New("test gitiles client factory failure")
				})

				client := NewClient(ctx)
				revision, err := client.FetchLatestRevision(ctx, "fake-host", "fake/project", "refs/heads/fake-branch")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("fails if API call fails", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("fake Log failure"))

				client := NewClient(ctx)
				revision, err := client.FetchLatestRevision(ctx, "fake-host", "fake/project", "refs/heads/fake-branch")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("returns latest revision for ref", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				matcher := proto.MatcherEqual(&gitilespb.LogRequest{
					Project:    "fake/project",
					Committish: "refs/heads/fake-branch",
					PageSize:   1,
				})
				// Check that potentially transient errors are retried
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(nil, status.Error(codes.Unavailable, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(&gitilespb.LogResponse{
						Log: []*gitpb.Commit{
							{Id: "fake-revision"},
						},
					}, nil)

				client := NewClient(ctx)
				revision, err := client.FetchLatestRevision(ctx, "fake-host", "fake/project", "refs/heads/fake-branch")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, revision, should.Equal("fake-revision"))
			})

		})

		t.Run("FetchLatestRevisionForPath", func(t *ftt.Test) {

			t.Run("fails if getting gitiles client fails", func(t *ftt.Test) {
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return nil, errors.New("test gitiles client factory failure")
				})

				client := NewClient(ctx)
				revision, err := client.FetchLatestRevisionForPath(ctx, "fake-host", "fake/project", "refs/heads/fake-branch", "fake-path")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("fails if API call fails", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("fake Log failure"))

				client := NewClient(ctx)
				revision, err := client.FetchLatestRevisionForPath(ctx, "fake-host", "fake/project", "refs/heads/fake-branch", "fake-path")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("returns latest revision for path on ref", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				matcher := proto.MatcherEqual(&gitilespb.LogRequest{
					Project:    "fake/project",
					Committish: "refs/heads/fake-branch",
					PageSize:   1,
					Path:       "fake-path",
				})
				// Check that potentially transient errors are retried
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(nil, status.Error(codes.Unavailable, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(&gitilespb.LogResponse{
						Log: []*gitpb.Commit{
							{Id: "fake-revision"},
						},
					}, nil)

				client := NewClient(ctx)
				revision, err := client.FetchLatestRevisionForPath(ctx, "fake-host", "fake/project", "refs/heads/fake-branch", "fake-path")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, revision, should.Equal("fake-revision"))
			})

		})

		t.Run("GetParentRevision", func(t *ftt.Test) {

			t.Run("fails if getting gitiles client fails", func(t *ftt.Test) {
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return nil, errors.New("test gitiles client factory failure")
				})

				client := NewClient(ctx)
				revision, err := client.GetParentRevision(ctx, "fake-host", "fake/project", "fake-revision")

				assert.Loosely(t, err, should.ErrLike("test gitiles client factory failure"))
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("fails if API call fails", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("fake Log failure"))

				client := NewClient(ctx)
				revision, err := client.GetParentRevision(ctx, "fake-host", "fake/project", "fake-revision")

				assert.Loosely(t, err, should.ErrLike("fake Log failure"))
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("returns parent revision for revision", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				matcher := proto.MatcherEqual(&gitilespb.LogRequest{
					Project:    "fake/project",
					Committish: "fake-revision",
					PageSize:   2,
				})
				// Check that potentially transient errors are retried
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(nil, status.Error(codes.Unavailable, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					Log(gomock.Any(), matcher).
					Return(&gitilespb.LogResponse{
						Log: []*gitpb.Commit{
							{Id: "fake-revision"},
							{Id: "fake-parent-revision"},
						},
					}, nil)

				client := NewClient(ctx)
				revision, err := client.GetParentRevision(ctx, "fake-host", "fake/project", "fake-revision")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, revision, should.Equal("fake-parent-revision"))
			})

		})

		t.Run("GetSubmoduleRevision", func(t *ftt.Test) {

			t.Run("fails if getting gitiles client fails", func(t *ftt.Test) {
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return nil, errors.New("test gitiles client factory failure")
				})

				client := NewClient(ctx)
				revision, err := client.GetSubmoduleRevision(ctx, "fake-host", "fake/project", "fake-revision", "fake/submodule/path")

				assert.Loosely(t, err, should.ErrLike("test gitiles client factory failure"))
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("fails if API call fails", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("fake DownloadFile failure"))

				client := NewClient(ctx)
				revision, err := client.GetSubmoduleRevision(ctx, "fake-host", "fake/project", "fake-revision", "fake/submodule/path")

				assert.Loosely(t, err, should.ErrLike("fake DownloadFile failure"))
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("fails if json response doesn't contain revision", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				matcher := proto.MatcherEqual(&gitilespb.DownloadFileRequest{
					Project:    "fake/project",
					Committish: "fake-revision",
					Path:       "fake/submodule/path",
					Format:     gitilespb.DownloadFileRequest_JSON,
				})
				// Check that potentially transient errors are retried
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(nil, status.Error(codes.Unavailable, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(&gitilespb.DownloadFileResponse{
						Contents: `{}`,
					}, nil)

				client := NewClient(ctx)
				revision, err := client.GetSubmoduleRevision(ctx, "fake-host", "fake/project", "fake-revision", "fake/submodule/path")

				assert.Loosely(t, err, should.ErrLike("no revision found for fake-host/fake/project/+/fake-revision/fake/submodule/path"))
				assert.Loosely(t, revision, should.BeEmpty)
			})

			t.Run("returns submodule revision", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				matcher := proto.MatcherEqual(&gitilespb.DownloadFileRequest{
					Project:    "fake/project",
					Committish: "fake-revision",
					Path:       "fake/submodule/path",
					Format:     gitilespb.DownloadFileRequest_JSON,
				})
				// Check that potentially transient errors are retried
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(nil, status.Error(codes.Unavailable, "fake transient Log failure"))
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(&gitilespb.DownloadFileResponse{
						Contents: `{"revision": "fake-submodule-revision"}`,
					}, nil)

				client := NewClient(ctx)
				revision, err := client.GetSubmoduleRevision(ctx, "fake-host", "fake/project", "fake-revision", "fake/submodule/path")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, revision, should.Equal("fake-submodule-revision"))
			})

		})

		t.Run("DownloadFile", func(t *ftt.Test) {

			t.Run("fails if getting gitiles client fails", func(t *ftt.Test) {
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return nil, errors.New("test gitiles client factory failure")
				})

				client := NewClient(ctx)
				contents, err := client.DownloadFile(ctx, "fake-host", "fake/project", "fake-revision", "fake-file")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, contents, should.BeEmpty)
			})

			t.Run("fails if API call fails", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("fake DownloadFile failure"))

				client := NewClient(ctx)
				contents, err := client.DownloadFile(ctx, "fake-host", "fake/project", "fake-revision", "fake-file")

				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, contents, should.BeEmpty)
			})

			t.Run("returns file contents", func(t *ftt.Test) {
				ctl := gomock.NewController(t)
				defer ctl.Finish()

				mockGitilesClient := mock_gitiles.NewMockGitilesClient(ctl)
				ctx := UseGitilesClientFactory(ctx, func(ctx context.Context, host string) (GitilesClient, error) {
					return mockGitilesClient, nil
				})
				matcher := proto.MatcherEqual(&gitilespb.DownloadFileRequest{
					Project:    "fake/project",
					Committish: "fake-revision",
					Path:       "fake-file",
					Format:     gitilespb.DownloadFileRequest_TEXT,
				})
				// Check that potentially transient errors are retried
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient DownloadFile failure"))
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(nil, status.Error(codes.NotFound, "fake transient DownloadFile failure"))
				mockGitilesClient.EXPECT().
					DownloadFile(gomock.Any(), matcher).
					Return(&gitilespb.DownloadFileResponse{
						Contents: "fake-contents",
					}, nil)

				client := NewClient(ctx)
				contents, err := client.DownloadFile(ctx, "fake-host", "fake/project", "fake-revision", "fake-file")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, contents, should.Equal("fake-contents"))
			})

		})

	})
}
