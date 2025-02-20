// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	tq "go.chromium.org/luci/gae/service/taskqueue"

	admin "go.chromium.org/infra/tricium/api/admin/v1"
	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common"
	gc "go.chromium.org/infra/tricium/appengine/common/gerrit"
	"go.chromium.org/infra/tricium/appengine/common/track"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

func TestReportResultsRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		// Add request -> run -> function -> worker to datastore.
		functionName := "MyLinter"
		request := &track.AnalyzeRequest{
			GitURL: "https://chromium-review.googlesource.com",
			GitRef: "refs/changes/88/508788/1",
			Files: []tricium.Data_File{
				{Path: "dir/file.txt"},
			},
		}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		run := &track.WorkflowRun{ID: 1, Parent: requestKey}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &track.FunctionRun{
			ID:     "MyLinter",
			Parent: runKey,
		}), should.BeNil)
		analyzerKey := ds.NewKey(ctx, "FunctionRun", "MyLinter", 0, runKey)
		assert.Loosely(t, ds.Put(ctx, &track.FunctionRunResult{
			ID:          1,
			Parent:      analyzerKey,
			Name:        "MyLinter",
			NumComments: 2,
		}), should.BeNil)
		workerName := "MyLinter_UBUNTU"
		assert.Loosely(t, ds.Put(ctx, &track.WorkerRun{
			ID:     workerName,
			Parent: analyzerKey,
		}), should.BeNil)

		// Add example Comment and associated CommentSelection entities.
		workerKey := ds.NewKey(ctx, "WorkerRun", workerName, 0, analyzerKey)

		changedLines := gc.ChangedLinesInfo{
			"dir/file.txt": {2, 5, 6},
		}
		dataCommentDeleted := tricium.Data_Comment{
			Category: "L",
			Message:  "Line too long",
			Path:     "dir/deleted_file.txt",
		}
		deletedFileCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&dataCommentDeleted)
		dataComment := tricium.Data_Comment{
			Category:  "L",
			Message:   "Line too short",
			Path:      "dir/file.txt",
			StartLine: 2,
			EndLine:   3,
		}
		inChangeCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&dataComment)
		assert.Loosely(t, err, should.BeNil)
		comments := []*track.Comment{
			{Parent: workerKey, Comment: []byte(deletedFileCommentJSON)},
			{Parent: workerKey, Comment: []byte(inChangeCommentJSON)},
			{Parent: workerKey, Comment: []byte(inChangeCommentJSON)},
		}
		assert.Loosely(t, ds.Put(ctx, comments), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comments[0]),
			Included: true,
		}), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID: 1, Parent: ds.KeyForObj(ctx, comments[1]),
			Included: true,
		}), should.BeNil)
		// The third comment added is not "included" when reporting
		// comments.
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comments[2]),
			Included: false,
		}), should.BeNil)

		t.Run("Reports only included comments", func(t *ftt.Test) {
			mock := &gc.MockRestAPI{ChangedLines: changedLines}
			err = reportResults(ctx, &admin.ReportResultsRequest{
				RunId:    run.ID,
				Analyzer: "MyLinter",
			}, mock)
			assert.Loosely(t, err, should.BeNil)
			// This only includes the two selected comments.
			assert.Loosely(t, len(mock.LastComments), should.Equal(len(comments)-1))

			t.Run("A successful request also sends a row to BQ", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.FeedbackEventsQueue]), should.Equal(1))
			})
		})

		t.Run("Does not report results when reporting is disabled", func(t *ftt.Test) {
			request.GerritReportingDisabled = true
			assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
			mock := &gc.MockRestAPI{ChangedLines: changedLines}
			err = reportResults(ctx, &admin.ReportResultsRequest{
				RunId:    run.ID,
				Analyzer: functionName,
			}, mock)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mock.LastComments), should.BeZero)

			t.Run("When no comments are posted, no rows are sent to BQ", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.FeedbackEventsQueue]), should.BeZero)
			})
		})

		// Put more comments in until the number of included comments
		// has reached the maximum comment number threshold.
		for len(comments) < maxComments+1 {
			comment := &track.Comment{Parent: workerKey, Comment: []byte(deletedFileCommentJSON)}
			comments = append(comments, comment)
			assert.Loosely(t, ds.Put(ctx, comment), should.BeNil)
			assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
				ID:       1,
				Parent:   ds.KeyForObj(ctx, comment),
				Included: true,
			}), should.BeNil)
		}
		assert.Loosely(t, len(comments), should.Equal(maxComments+1))

		t.Run("Reports when number of comments is at maximum", func(t *ftt.Test) {
			mock := &gc.MockRestAPI{ChangedLines: changedLines}
			err = reportResults(ctx, &admin.ReportResultsRequest{
				RunId:    run.ID,
				Analyzer: functionName,
			}, mock)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mock.LastComments), should.Equal(maxComments))
		})

		// This comment is not on a changed line.
		dataOutside := tricium.Data_Comment{
			Category:  "L",
			Message:   "Line too short",
			Path:      "dir/file.txt",
			StartLine: 3,
			EndLine:   3,
		}
		outsideChangeCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&dataOutside)
		// Put the new comment with line numbers in.
		outsideChangeComment := &track.Comment{Parent: workerKey, Comment: []byte(outsideChangeCommentJSON)}
		comments = append(comments, outsideChangeComment)
		assert.Loosely(t, ds.Put(ctx, outsideChangeComment), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, outsideChangeComment),
			Included: gc.CommentIsInChangedLines(ctx, &dataOutside, changedLines, 0),
		}), should.BeNil)
		assert.Loosely(t, len(comments), should.Equal(maxComments+2))

		t.Run("Does not report comments that are not on changed lines", func(t *ftt.Test) {
			mock := &gc.MockRestAPI{ChangedLines: changedLines}
			err := reportResults(ctx, &admin.ReportResultsRequest{
				RunId:    run.ID,
				Analyzer: functionName,
			}, mock)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mock.LastComments), should.Equal(maxComments))
		})

		// Put one more comment in;
		comment := &track.Comment{Parent: workerKey, Comment: []byte(deletedFileCommentJSON)}
		comments = append(comments, comment)
		assert.Loosely(t, ds.Put(ctx, comment), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comment),
			Included: gc.CommentIsInChangedLines(ctx, &dataCommentDeleted, changedLines, 0),
		}), should.BeNil)
		assert.Loosely(t, len(comments), should.Equal(maxComments+3))

		t.Run("Truncates comments when number of comments exceeds maximum", func(t *ftt.Test) {
			mock := &gc.MockRestAPI{ChangedLines: changedLines}
			err := reportResults(ctx, &admin.ReportResultsRequest{
				RunId:    run.ID,
				Analyzer: functionName,
			}, mock)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mock.LastComments), should.Equal(maxComments))
		})
	})
}

func TestReportResultsRequestWithRenamedOrCopiedFiles(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		// Add request -> run -> function -> worker to datastore.
		request := &track.AnalyzeRequest{
			GitURL: "https://chromium-review.googlesource.com",
			GitRef: "refs/changes/88/508788/1",
			Files: []tricium.Data_File{
				{Path: "dir/file.txt"},
				{Path: "dir/renamed_file.txt", Status: tricium.Data_RENAMED},
				{Path: "dir/copied_file.txt", Status: tricium.Data_COPIED},
			},
		}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		run := &track.WorkflowRun{ID: 1, Parent: requestKey}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &track.FunctionRun{
			ID:     "MyLinter",
			Parent: runKey,
		}), should.BeNil)
		analyzerKey := ds.NewKey(ctx, "FunctionRun", "MyLinter", 0, runKey)
		assert.Loosely(t, ds.Put(ctx, &track.FunctionRunResult{
			ID:          1,
			Parent:      analyzerKey,
			Name:        "MyLinter",
			NumComments: 2,
		}), should.BeNil)
		workerName := "MyLinter_UBUNTU"
		assert.Loosely(t, ds.Put(ctx, &track.WorkerRun{
			ID:     workerName,
			Parent: analyzerKey,
		}), should.BeNil)

		// Add example Comment and associated CommentSelection entities.
		workerKey := ds.NewKey(ctx, "WorkerRun", workerName, 0, analyzerKey)

		changedLines := gc.ChangedLinesInfo{
			"dir/file.txt":         {2, 5, 6},
			"dir/renamed_file.txt": {1, 2, 3, 4, 5, 6, 7},
			"dir/copied_file.txt":  {1, 2, 3, 4, 5, 6, 7},
		}
		gc.FilterRequestChangedLines(request, &changedLines)
		dataCommentChanged := tricium.Data_Comment{
			Category:  "L",
			Message:   "Line too short",
			Path:      "dir/file.txt",
			StartLine: 2,
			EndLine:   3,
		}
		inChangeCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&dataCommentChanged)
		dataCommentRenamed := tricium.Data_Comment{
			Category:  "L",
			Message:   "Line doesn't look right",
			Path:      "dir/renamed_file.txt",
			StartLine: 2,
			EndLine:   3,
		}
		inRenamedFileCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&dataCommentRenamed)
		dataCommentCopied := tricium.Data_Comment{
			Category:  "L",
			Message:   "Line doesn't look right",
			Path:      "dir/copied_file.txt",
			StartLine: 2,
			EndLine:   3,
		}
		inCopiedFileCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&dataCommentCopied)
		assert.Loosely(t, err, should.BeNil)
		comments := []*track.Comment{
			{Parent: workerKey, Comment: []byte(inChangeCommentJSON)},
			{Parent: workerKey, Comment: []byte(inRenamedFileCommentJSON)},
			{Parent: workerKey, Comment: []byte(inCopiedFileCommentJSON)},
		}
		assert.Loosely(t, ds.Put(ctx, comments), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comments[0]),
			Included: gc.CommentIsInChangedLines(ctx, &dataCommentChanged, changedLines, 0),
		}), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comments[1]),
			Included: gc.CommentIsInChangedLines(ctx, &dataCommentRenamed, changedLines, 0),
		}), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comments[2]),
			Included: gc.CommentIsInChangedLines(ctx, &dataCommentCopied, changedLines, 0),
		}), should.BeNil)

		t.Run("Does not report comments in renamed or copied files", func(t *ftt.Test) {
			mock := &gc.MockRestAPI{ChangedLines: changedLines}
			err := reportResults(ctx, &admin.ReportResultsRequest{
				RunId:    run.ID,
				Analyzer: "MyLinter",
			}, mock)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mock.LastComments), should.Equal(1))
		})
	})
}
