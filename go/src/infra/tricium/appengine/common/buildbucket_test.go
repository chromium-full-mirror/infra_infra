// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/grpc"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"

	admin "infra/tricium/api/admin/v1"
	tricium "infra/tricium/api/v1"
)

func TestTrigger(t *testing.T) {
	ftt.Run("Triggers a build", t, func(t *ftt.Test) {
		w := &admin.Worker{
			Name: "AnalyzerName",
			Impl: &admin.Worker_Recipe{
				Recipe: &tricium.Recipe{
					Project: "tricium",
					Bucket:  "try",
					Builder: "analyzer",
				},
			},
		}
		params := &TriggerParameters{
			Worker:         w,
			PubsubUserdata: "data",
			Patch: PatchDetails{
				GerritHost:    "https://chromium-review.googlesource.com",
				GerritProject: "chromium/src",
				GerritChange:  "chromium~master~I8473b95934b5732ac55d26311a706c9c2bde9940",
				GerritCl:      "12345",
				GerritPatch:   "2",
			},
		}
		ctx := memory.Use(memlogger.Use(context.Background()))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := buildbucketpb.NewMockBuildsClient(ctrl)

		scheduleBuild := func(ctx context.Context, req *buildbucketpb.ScheduleBuildRequest, opts ...grpc.CallOption) (*buildbucketpb.Build, error) {
			res := &buildbucketpb.Build{Id: 1}
			return res, nil
		}
		client.EXPECT().
			ScheduleBuild(gomock.Any(), gomock.Any()).
			AnyTimes().
			DoAndReturn(scheduleBuild)

		result, err := trigger(ctx, params, client)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, result.BuildID, should.Equal(1))
	})

	ftt.Run("Different analyzers have different request IDs", t, func(t *ftt.Test) {
		patch := &PatchDetails{
			GerritHost:    "cr.example.com",
			GerritProject: "foo",
			GerritCl:      "123",
			GerritPatch:   "7",
		}
		recipeX := &tricium.Recipe{Project: "cr", Bucket: "try", Builder: "x-lint"}
		recipeY := &tricium.Recipe{Project: "cr", Bucket: "try", Builder: "y-lint"}
		assert.Loosely(t, makeRequestID(patch, recipeX), should.Equal("foo~123~7~cr.try.x-lint"))
		assert.Loosely(t, makeRequestID(patch, recipeY), should.Equal("foo~123~7~cr.try.y-lint"))
	})

	ftt.Run("Request IDs do not contain slash", t, func(t *ftt.Test) {
		patch := &PatchDetails{
			GerritHost:    "cr-review.goo.com",
			GerritProject: "foo/bar",
			GerritCl:      "123",
			GerritPatch:   "7",
		}
		recipe := &tricium.Recipe{Project: "cr", Bucket: "try", Builder: "x"}
		assert.Loosely(t, makeRequestID(patch, recipe), should.Equal("foo_bar~123~7~cr.try.x"))
	})
}

func TestCollect(t *testing.T) {
	ftt.Run("Collects a build", t, func(t *ftt.Test) {
		params := &CollectParameters{
			BuildID: 1,
		}
		ctx := memory.Use(memlogger.Use(context.Background()))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := buildbucketpb.NewMockBuildsClient(ctrl)

		getBuild := func(ctx context.Context, req *buildbucketpb.GetBuildRequest, opts ...grpc.CallOption) (*buildbucketpb.Build, error) {
			res := &buildbucketpb.Build{
				Id:     1,
				Status: buildbucketpb.Status_SUCCESS,
				Output: &buildbucketpb.Build_Output{
					Properties: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"tricium": {
								Kind: &structpb.Value_StringValue{StringValue: "{\"tricium\": []}"},
							},
						}},
				},
			}
			return res, nil
		}
		client.EXPECT().
			GetBuild(gomock.Any(), gomock.Any()).
			AnyTimes().
			DoAndReturn(getBuild)

		result, err := collect(ctx, params, client)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, result.State, should.Equal(Success))
		assert.Loosely(t, result.BuildbucketOutput, should.Equal("{\"tricium\": []}"))
	})
}
