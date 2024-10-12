// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package client

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

type mockBuildersClient struct{}

func (mbc mockBuildersClient) ListBuilders(c context.Context, req *buildbucketpb.ListBuildersRequest, opts ...grpc.CallOption) (*buildbucketpb.ListBuildersResponse, error) {
	if req.PageToken == "" {
		return &buildbucketpb.ListBuildersResponse{
			Builders: []*buildbucketpb.BuilderItem{
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "ci",
						Builder: "ci_1",
					},
				},
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "ci",
						Builder: "ci_2",
					},
				},
			},
			NextPageToken: "token",
		}, nil
	}
	if req.PageToken == "token" {
		return &buildbucketpb.ListBuildersResponse{
			Builders: []*buildbucketpb.BuilderItem{
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "ci",
						Builder: "ci_3",
					},
				},
			},
		}, nil
	}
	return nil, nil
}

func TestListBuildersByBucket(t *testing.T) {
	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)
	cl := mockBuildersClient{}

	ftt.Run("no builder", t, func(t *ftt.Test) {
		builders, err := ListBuildersByBucket(ctx, cl, "chromium", "ci")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(builders), should.Equal(3))
		assert.Loosely(t, builders[0].Id.Builder, should.Equal("ci_1"))
		assert.Loosely(t, builders[1].Id.Builder, should.Equal("ci_2"))
		assert.Loosely(t, builders[2].Id.Builder, should.Equal("ci_3"))
	})
}
