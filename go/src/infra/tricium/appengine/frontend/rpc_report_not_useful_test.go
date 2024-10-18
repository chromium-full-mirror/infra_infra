// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	tq "go.chromium.org/luci/gae/service/taskqueue"

	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common"
	"infra/tricium/appengine/common/track"
	"infra/tricium/appengine/common/triciumtest"
)

func TestReportNotUseful(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		commentID := "7ef59cda-183c-48b3-8343-d9036a7f1419"
		functionName := "Spacey"
		platform := tricium.Platform_UBUNTU

		// Add comment entity with ancestors:
		// AnalyzeRequest>WorkflowRun>FunctionRun>WorkerRun>Comment>CommentFeedback
		request := &track.AnalyzeRequest{}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		run := &track.WorkflowRun{ID: 1, Parent: ds.KeyForObj(ctx, request)}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		function := &track.FunctionRun{
			ID:     functionName,
			Parent: ds.KeyForObj(ctx, run),
		}
		assert.Loosely(t, ds.Put(ctx, function), should.BeNil)
		worker := &track.WorkerRun{
			ID:     fmt.Sprintf("%s_%s", functionName, platform),
			Parent: ds.KeyForObj(ctx, function),
		}
		assert.Loosely(t, ds.Put(ctx, worker), should.BeNil)
		json, err := (&jsonpb.Marshaler{}).MarshalToString(&tricium.Data_Comment{
			Path:      "dir/file.txt",
			Message:   "comment message",
			StartLine: 4,
		})
		assert.Loosely(t, err, should.BeNil)
		comment := &track.Comment{
			UUID:      commentID,
			Analyzer:  "Spacey",
			Comment:   []byte(json),
			Parent:    ds.KeyForObj(ctx, worker),
			Platforms: 1,
		}
		assert.Loosely(t, ds.Put(ctx, comment), should.BeNil)
		feedback := &track.CommentFeedback{ID: 1, Parent: ds.KeyForObj(ctx, comment)}
		assert.Loosely(t, ds.Put(ctx, feedback), should.BeNil)

		t.Run("Single request for known comment increments count", func(t *ftt.Test) {
			request := &tricium.ReportNotUsefulRequest{CommentId: commentID}
			response, err := server.ReportNotUseful(ctx, request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.Match(&tricium.ReportNotUsefulResponse{
				Owner:                   "yiwzhang@google.com",
				MonorailComponent:       "Infra>LUCI>BuildService>PreSubmit>Tricium",
				IssueTrackerComponentId: 1456522,
			}))
			assert.Loosely(t, ds.Get(ctx, feedback), should.BeNil)
			assert.Loosely(t, feedback.NotUsefulReports, should.Equal(1))

			t.Run("A successful request also sends a row to BQ", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.FeedbackEventsQueue]), should.Equal(1))
			})
		})

		t.Run("Request for unknown comment gives error", func(t *ftt.Test) {
			request := &tricium.ReportNotUsefulRequest{CommentId: "abcdefg"}
			response, err := server.ReportNotUseful(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, response, should.BeNil)
			assert.Loosely(t, ds.Get(ctx, feedback), should.BeNil)
			assert.Loosely(t, feedback.NotUsefulReports, should.BeZero)

			t.Run("An unsuccessful request sends no row to BQ", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.FeedbackEventsQueue]), should.BeZero)
			})
		})

		t.Run("Request with no comment gives error", func(t *ftt.Test) {
			request := &tricium.ReportNotUsefulRequest{}
			response, err := server.ReportNotUseful(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, response, should.BeNil)
			assert.Loosely(t, ds.Get(ctx, feedback), should.BeNil)
			assert.Loosely(t, feedback.NotUsefulReports, should.BeZero)
		})

		t.Run("Two requests increment twice and have the same response", func(t *ftt.Test) {
			request := &tricium.ReportNotUsefulRequest{CommentId: commentID}
			response, err := server.ReportNotUseful(ctx, request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.Match(&tricium.ReportNotUsefulResponse{
				Owner:                   "yiwzhang@google.com",
				MonorailComponent:       "Infra>LUCI>BuildService>PreSubmit>Tricium",
				IssueTrackerComponentId: 1456522,
			}))
			response, err = server.ReportNotUseful(ctx, request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.Match(&tricium.ReportNotUsefulResponse{
				Owner:                   "yiwzhang@google.com",
				MonorailComponent:       "Infra>LUCI>BuildService>PreSubmit>Tricium",
				IssueTrackerComponentId: 1456522,
			}))
			assert.Loosely(t, ds.Get(ctx, feedback), should.BeNil)
			assert.Loosely(t, feedback.NotUsefulReports, should.Equal(2))
		})

		t.Run("Hardcoded owner lookup table is used", func(t *ftt.Test) {
			commentID := "12345cda-abcd-48b3-8343-d9036a7f1419"
			comment := &track.Comment{
				UUID:      commentID,
				Analyzer:  "ClangTidy",
				Comment:   []byte(json),
				Parent:    ds.KeyForObj(ctx, worker),
				Platforms: 1,
			}
			assert.Loosely(t, ds.Put(ctx, comment), should.BeNil)
			feedback := &track.CommentFeedback{ID: 1, Parent: ds.KeyForObj(ctx, comment)}
			assert.Loosely(t, ds.Put(ctx, feedback), should.BeNil)

			response, err := server.ReportNotUseful(ctx, &tricium.ReportNotUsefulRequest{CommentId: commentID})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.Match(&tricium.ReportNotUsefulResponse{
				Owner:                   "gbiv@chromium.org",
				MonorailComponent:       "Infra>LUCI>BuildService>PreSubmit>Tricium",
				IssueTrackerComponentId: 1456522,
			}))
		})
	})
}
