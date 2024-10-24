// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gerrit

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/proto"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/grpc/grpcutil"

	"infra/chromium/bootstrapper/clients/gob"
)

func TestGerritClientForHost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Client.gerritClientForHost", t, func(t *ftt.Test) {

		t.Run("fails if factory fails", func(t *ftt.Test) {
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return nil, errors.New("fake client factory failure")
			})

			client := NewClient(ctx)
			gerritClient, err := client.gerritClientForHost(ctx, "fake-host")

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, gerritClient, should.BeNil)
		})

		t.Run("returns gerrit client from factory", func(t *ftt.Test) {
			mockGerritClient := gerritpb.NewMockGerritClient(gomock.NewController(t))
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return mockGerritClient, nil
			})

			client := NewClient(ctx)
			gerritClient, err := client.gerritClientForHost(ctx, "fake-host")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gerritClient, should.Equal(mockGerritClient))
		})

		t.Run("re-uses gerrit client for host", func(t *ftt.Test) {
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return gerritpb.NewMockGerritClient(gomock.NewController(t)), nil
			})

			client := NewClient(ctx)
			gerritClientFoo1, _ := client.gerritClientForHost(ctx, "fake-host-foo")
			gerritClientFoo2, _ := client.gerritClientForHost(ctx, "fake-host-foo")
			gerritClientBar, _ := client.gerritClientForHost(ctx, "fake-host-bar")

			assert.Loosely(t, gerritClientFoo1, should.NotBeNil)
			assert.Loosely(t, gerritClientFoo2, should.Equal(gerritClientFoo1))
			assert.Loosely(t, gerritClientBar, should.NotEqual(gerritClientFoo1))
		})

	})
}

func TestGetChangeInfo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ctx = gob.UseTestClock(ctx)

	ftt.Run("Client.getChangeInfo", t, func(t *ftt.Test) {

		t.Run("fails if getting client for host fails", func(t *ftt.Test) {
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return nil, errors.New("fake client factory failure")
			})

			client := NewClient(ctx)
			changeInfo, err := client.GetChangeInfo(ctx, "fake-host", "fake/project", 123, 1)

			assert.Loosely(t, err, should.ErrLike("fake client factory failure"))
			assert.Loosely(t, changeInfo, should.BeNil)
		})

		t.Run("fails if API call fails", func(t *ftt.Test) {
			mockGerritClient := gerritpb.NewMockGerritClient(gomock.NewController(t))
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return mockGerritClient, nil
			})
			mockGerritClient.EXPECT().
				GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Project: "fake/project",
					Number:  123,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_ALL_REVISIONS},
				})).
				Return(nil, errors.New("fake GetChange failure"))

			client := NewClient(ctx)
			changeInfo, err := client.GetChangeInfo(ctx, "fake-host", "fake/project", 123, 1)

			assert.Loosely(t, err, should.ErrLike("fake GetChange failure"))
			assert.Loosely(t, changeInfo, should.BeNil)
		})

		t.Run("returns change info for change", func(t *ftt.Test) {
			mockGerritClient := gerritpb.NewMockGerritClient(gomock.NewController(t))
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return mockGerritClient, nil
			})
			mockChangeInfo := &gerritpb.ChangeInfo{
				Project: "fake/project",
				Number:  123,
				Ref:     "fake-ref",
				Revisions: map[string]*gerritpb.RevisionInfo{
					"fake-revision": {
						Number: 1,
					},
				},
			}
			matcher := proto.MatcherEqual(&gerritpb.GetChangeRequest{
				Project: "fake/project",
				Number:  123,
				Options: []gerritpb.QueryOption{gerritpb.QueryOption_ALL_REVISIONS},
			})
			// Check that potentially transient errors are retried
			mockGerritClient.EXPECT().
				GetChange(gomock.Any(), matcher).
				Return(nil, status.Error(codes.NotFound, "fake transient GetChange failure"))
			mockGerritClient.EXPECT().
				GetChange(gomock.Any(), matcher).
				Return(nil, status.Error(codes.Unavailable, "fake transient GetChange failure"))
			mockGerritClient.EXPECT().
				GetChange(gomock.Any(), matcher).
				Return(mockChangeInfo, nil)

			client := NewClient(ctx)
			changeInfo, err := client.GetChangeInfo(ctx, "fake-host", "fake/project", 123, 1)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changeInfo, should.Resemble(&ChangeInfo{
				TargetRef:       "fake-ref",
				GitilesRevision: "fake-revision",
			}))
		})

		t.Run("fails with not found if change doesn't have requested patchset", func(t *ftt.Test) {
			mockGerritClient := gerritpb.NewMockGerritClient(gomock.NewController(t))
			ctx := UseGerritClientFactory(ctx, func(ctx context.Context, host string) (GerritClient, error) {
				return mockGerritClient, nil
			})
			mockGerritClient.EXPECT().
				GetChange(gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(ctx context.Context, req *gerritpb.GetChangeRequest, opts ...grpc.CallOption) (*gerritpb.ChangeInfo, error) {
					return &gerritpb.ChangeInfo{
						Project: req.Project,
						Number:  req.Number,
						Ref:     "fake-ref",
						Revisions: map[string]*gerritpb.RevisionInfo{
							"fake-revision": {
								Number: 1,
							},
						},
					}, nil
				})

			client := NewClient(ctx)
			changeInfo, err := client.GetChangeInfo(ctx, "fake-host", "fake/project", 123, 2)

			assert.Loosely(t, err, should.ErrLike("fake-host/c/fake/project/+/123 does not have patchset 2"))
			assert.Loosely(t, grpcutil.Code(err), should.Equal(codes.NotFound))
			assert.Loosely(t, changeInfo, should.BeNil)
		})

	})
}
