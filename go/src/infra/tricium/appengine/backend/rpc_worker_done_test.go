// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/qscheduler/qslib/tutils"
	admin "go.chromium.org/infra/tricium/api/admin/v1"
	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common/gerrit"
	"go.chromium.org/infra/tricium/appengine/common/track"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

func TestWorkerDoneRequest(t *testing.T) {
	ftt.Run("Worker done request with successful worker", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		simple := "Simple"
		simpleUbuntu := "Simple_Ubuntu"

		workflowProvider := &mockWorkflowProvider{
			Workflow: &admin.Workflow{
				Workers: []*admin.Worker{
					{
						Name:     simpleUbuntu,
						Needs:    tricium.Data_FILES,
						Provides: tricium.Data_RESULTS,
					},
				},
			},
		}

		// Add pending workflow run.
		request := &track.AnalyzeRequest{}
		request.GitRef = "refs/changes/88/508788/7"
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		run := &track.WorkflowRun{ID: 1, Parent: requestKey}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &track.WorkflowRunResult{
			ID:     1,
			Parent: runKey,
			State:  tricium.State_PENDING,
		}), should.BeNil)

		// Mark workflow as launched.
		assert.Loosely(t, workflowLaunched(ctx, &admin.WorkflowLaunchedRequest{
			RunId: request.ID,
		}, workflowProvider), should.BeNil)

		// Mark worker as launched.
		assert.Loosely(t, workerLaunched(ctx, &admin.WorkerLaunchedRequest{
			RunId:  request.ID,
			Worker: simpleUbuntu,
		}), should.BeNil)

		// Mark worker as done.
		assert.Loosely(t, workerDone(ctx, &admin.WorkerDoneRequest{
			RunId:             request.ID,
			Worker:            simpleUbuntu,
			Provides:          tricium.Data_RESULTS,
			State:             tricium.State_SUCCESS,
			BuildbucketOutput: `{"comments": [{"message": "foo"}, {"message": "bar"}]}`,
		}), should.BeNil)

		functionKey := ds.NewKey(ctx, "FunctionRun", simple, 0, runKey)

		t.Run("Marks worker as done", func(t *ftt.Test) {
			workerKey := ds.NewKey(ctx, "WorkerRun", simpleUbuntu, 0, functionKey)
			wr := &track.WorkerRunResult{ID: 1, Parent: workerKey}
			assert.Loosely(t, ds.Get(ctx, wr), should.BeNil)
			assert.Loosely(t, wr.State, should.Equal(tricium.State_SUCCESS))
		})

		t.Run("Marks function as done and adds no comments", func(t *ftt.Test) {
			fr := &track.FunctionRunResult{ID: 1, Parent: functionKey}
			assert.Loosely(t, ds.Get(ctx, fr), should.BeNil)
			assert.Loosely(t, fr.State, should.Equal(tricium.State_SUCCESS))
		})

		t.Run("Marks workflow as done and adds comments", func(t *ftt.Test) {
			wr := &track.WorkflowRunResult{ID: 1, Parent: runKey}
			assert.Loosely(t, ds.Get(ctx, wr), should.BeNil)
			assert.Loosely(t, wr.State, should.Equal(tricium.State_SUCCESS))
			assert.Loosely(t, wr.NumComments, should.Equal(2))
		})

		t.Run("Marks request as done", func(t *ftt.Test) {
			ar := &track.AnalyzeRequestResult{ID: 1, Parent: requestKey}
			assert.Loosely(t, ds.Get(ctx, ar), should.BeNil)
			assert.Loosely(t, ar.State, should.Equal(tricium.State_SUCCESS))
		})
	})
}

func TestRecipeWorkerDoneRequest(t *testing.T) {
	ftt.Run("Worker done request with successful worker", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		simple := "Simple"
		simpleUbuntu := "Simple_Ubuntu"

		workflowProvider := &mockWorkflowProvider{
			Workflow: &admin.Workflow{
				Workers: []*admin.Worker{
					{
						Name:     simpleUbuntu,
						Provides: tricium.Data_RESULTS,
					},
				},
			},
		}

		// Add pending workflow run.
		request := &track.AnalyzeRequest{}
		request.GitRef = "refs/changes/88/508788/7"
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		run := &track.WorkflowRun{ID: 1, Parent: requestKey}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &track.WorkflowRunResult{
			ID:     1,
			Parent: runKey,
			State:  tricium.State_PENDING,
		}), should.BeNil)

		// Mark workflow as launched.
		assert.Loosely(t, workflowLaunched(ctx, &admin.WorkflowLaunchedRequest{
			RunId: request.ID,
		}, workflowProvider), should.BeNil)

		// Mark worker as launched.
		assert.Loosely(t, workerLaunched(ctx, &admin.WorkerLaunchedRequest{
			RunId:  request.ID,
			Worker: simpleUbuntu,
		}), should.BeNil)

		// Mark worker as done.
		assert.Loosely(t, workerDone(ctx, &admin.WorkerDoneRequest{
			RunId:             request.ID,
			Worker:            simpleUbuntu,
			Provides:          tricium.Data_GIT_FILE_DETAILS,
			State:             tricium.State_SUCCESS,
			BuildbucketOutput: `{"comments": []}`,
		}), should.BeNil)

		functionKey := ds.NewKey(ctx, "FunctionRun", simple, 0, runKey)

		t.Run("Marks worker as done", func(t *ftt.Test) {
			workerKey := ds.NewKey(ctx, "WorkerRun", simpleUbuntu, 0, functionKey)
			wr := &track.WorkerRunResult{ID: 1, Parent: workerKey}
			assert.Loosely(t, ds.Get(ctx, wr), should.BeNil)
			assert.Loosely(t, wr.State, should.Equal(tricium.State_SUCCESS))
		})
	})
}

func TestAbortedWorkerDoneRequest(t *testing.T) {
	ftt.Run("Worker done request with an aborted worker", t, func(t *ftt.Test) {
		// This test is similar to the case above, except that one of
		// the workers is aborted, so the function is considered
		// failed, and thus the workflow run is failed.
		ctx := triciumtest.Context()

		simple := "Simple"
		simpleUbuntu := "Simple_Ubuntu"

		workflowProvider := &mockWorkflowProvider{
			Workflow: &admin.Workflow{
				Workers: []*admin.Worker{
					{
						Name:     simpleUbuntu,
						Needs:    tricium.Data_GIT_FILE_DETAILS,
						Provides: tricium.Data_RESULTS,
					},
				},
			},
		}

		// Add pending run entry.
		request := &track.AnalyzeRequest{}
		request.GitRef = "refs/changes/88/508788/7"
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		run := &track.WorkflowRun{ID: 1, Parent: requestKey}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &track.WorkflowRunResult{
			ID:     1,
			Parent: runKey,
			State:  tricium.State_PENDING,
		}), should.BeNil)

		// Mark workflow as launched.
		assert.Loosely(t, workflowLaunched(ctx, &admin.WorkflowLaunchedRequest{
			RunId: request.ID,
		}, workflowProvider), should.BeNil)

		// Mark worker as launched.
		assert.Loosely(t, workerLaunched(ctx, &admin.WorkerLaunchedRequest{
			RunId:  request.ID,
			Worker: simpleUbuntu,
		}), should.BeNil)

		// Mark worker as aborted.
		assert.Loosely(t, workerDone(ctx, &admin.WorkerDoneRequest{
			RunId:             request.ID,
			Worker:            simpleUbuntu,
			State:             tricium.State_ABORTED,
			BuildbucketOutput: `{"comments": []}`,
		}), should.BeNil)

		functionKey := ds.NewKey(ctx, "FunctionRun", simple, 0, runKey)

		t.Run("WorkerRun is marked as aborted", func(t *ftt.Test) {
			workerKey := ds.NewKey(ctx, "WorkerRun", simpleUbuntu, 0, functionKey)
			wr := &track.WorkerRunResult{ID: 1, Parent: workerKey}
			assert.Loosely(t, ds.Get(ctx, wr), should.BeNil)
			assert.Loosely(t, wr.State, should.Equal(tricium.State_ABORTED))
		})

		t.Run("FunctionRun is failed, with no comments", func(t *ftt.Test) {
			fr := &track.FunctionRunResult{ID: 1, Parent: functionKey}
			assert.Loosely(t, ds.Get(ctx, fr), should.BeNil)
			assert.Loosely(t, fr.State, should.Equal(tricium.State_FAILURE))
		})

		t.Run("WorkflowRun is marked as failed", func(t *ftt.Test) {
			wr := &track.WorkflowRunResult{ID: 1, Parent: runKey}
			assert.Loosely(t, ds.Get(ctx, wr), should.BeNil)
			assert.Loosely(t, wr.State, should.Equal(tricium.State_FAILURE))
		})

		t.Run("AnalyzeRequest is marked as failed", func(t *ftt.Test) {
			ar := &track.AnalyzeRequestResult{ID: 1, Parent: requestKey}
			assert.Loosely(t, ds.Get(ctx, ar), should.BeNil)
			assert.Loosely(t, ar.State, should.Equal(tricium.State_FAILURE))
		})
	})
}

func TestValidateWorkerDoneRequestRequest(t *testing.T) {
	ftt.Run("Request with all parts is valid", t, func(t *ftt.Test) {
		assert.Loosely(t, validateWorkerDoneRequest(&admin.WorkerDoneRequest{
			RunId:             1234,
			Worker:            "MyLint_Ubuntu",
			Provides:          tricium.Data_RESULTS,
			State:             tricium.State_SUCCESS,
			BuildbucketOutput: `{"comments": []}`,
		}), should.BeNil)
	})

	ftt.Run("Specifying provides and state is optional", t, func(t *ftt.Test) {
		assert.Loosely(t, validateWorkerDoneRequest(&admin.WorkerDoneRequest{
			RunId:             1234,
			Worker:            "MyLint_Ubuntu",
			BuildbucketOutput: `{"comments": []}`,
		}), should.BeNil)
	})

	ftt.Run("Request with no run ID is invalid", t, func(t *ftt.Test) {
		assert.Loosely(t, validateWorkerDoneRequest(&admin.WorkerDoneRequest{
			Worker:            "MyLint_Ubuntu",
			BuildbucketOutput: `{"comments": []}`,
		}), should.NotBeNil)
	})

	ftt.Run("Request with no worker name invalid", t, func(t *ftt.Test) {
		assert.Loosely(t, validateWorkerDoneRequest(&admin.WorkerDoneRequest{
			RunId:             1234,
			BuildbucketOutput: `{"comments": []}`,
		}), should.NotBeNil)
	})

	ftt.Run("Request with no output is valid", t, func(t *ftt.Test) {
		assert.Loosely(t, validateWorkerDoneRequest(&admin.WorkerDoneRequest{
			RunId:  1234,
			Worker: "MyLint_Ubuntu",
		}), should.BeNil)
	})

	ftt.Run("Providing just buildbucket output is OK", t, func(t *ftt.Test) {
		assert.Loosely(t, validateWorkerDoneRequest(&admin.WorkerDoneRequest{
			RunId:             1234,
			Worker:            "MyLint_Ubuntu",
			BuildbucketOutput: `{"comments": []}`,
		}), should.BeNil)
	})
}

func TestCreateCommentSelection(t *testing.T) {
	changedLines := gerrit.ChangedLinesInfo{
		"dir/file.txt": {2, 5, 6},
	}
	mock := &gerrit.MockRestAPI{ChangedLines: changedLines}
	ctx := triciumtest.Context()
	req := track.AnalyzeRequest{
		GerritProject: "my-project",
		GerritChange:  "my-project~master~I8473b95934b5732ac55d26311a706c9c2bde9940",
	}
	ftt.Run("createCommentSelection keeps unchanged lines if comment's ShowOnUnchangedLines is true", t, func(t *ftt.Test) {
		commentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&tricium.Data_Comment{
			Message:              "Not in Change",
			Path:                 "dir/file.txt",
			StartLine:            10,
			EndLine:              11,
			ShowOnUnchangedLines: true,
		})
		comments := []*track.Comment{
			{
				Comment: []byte(commentJSON),
			},
		}
		selections := []*track.CommentSelection{
			{
				ID:       1,
				Parent:   nil,
				Included: true,
			},
		}
		result, err := createCommentSelections(ctx, mock, &req, comments)
		assert.Loosely(t, result, should.Resemble(selections))
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("createCommentSelection discards unchanged lines if comment's ShowOnUnchangedLines is false", t, func(t *ftt.Test) {
		commentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&tricium.Data_Comment{
			Message:              "Not in Change",
			Path:                 "dir/file.txt",
			StartLine:            10,
			EndLine:              11,
			ShowOnUnchangedLines: false,
		})
		comments := []*track.Comment{
			{
				Comment: []byte(commentJSON),
			},
		}
		selections := []*track.CommentSelection{
			{
				ID:       1,
				Parent:   nil,
				Included: false,
			},
		}
		result, err := createCommentSelections(ctx, mock, &req, comments)
		assert.Loosely(t, result, should.Resemble(selections))
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCreateAnalysisResults(t *testing.T) {
	ftt.Run("Default objects", t, func(t *ftt.Test) {
		wres := track.WorkerRunResult{}
		areq := track.AnalyzeRequest{}
		ares := track.AnalyzeRequestResult{}
		comments := []*track.Comment{}
		selections := []*track.CommentSelection{}

		areq.GitRef = "refs/changes/88/508788/102"
		result, err := createAnalysisResults(&wres, &areq, &ares, comments, selections)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, result, should.NotBeNil)
		assert.Loosely(t, result.RevisionNumber, should.Equal(102))
	})

	ftt.Run("GitRef required", t, func(t *ftt.Test) {
		wres := track.WorkerRunResult{}
		areq := track.AnalyzeRequest{}
		ares := track.AnalyzeRequestResult{}
		comments := []*track.Comment{}
		selections := []*track.CommentSelection{}

		result, err := createAnalysisResults(&wres, &areq, &ares, comments, selections)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, result, should.BeNil)
	})

	ftt.Run("All values", t, func(t *ftt.Test) {
		wres := track.WorkerRunResult{}

		areq := track.AnalyzeRequest{}
		areq.GerritHost = "http://my-gerrit-review.com/my-project"
		areq.Project = "my-project"
		areq.GerritChange = "my-project~master~I8473b95934b5732ac55d26311a706c9c2bde9940"
		areq.GitURL = "http://the-git-url.com/my-project"
		areq.GitRef = "refs/changes/88/508788/7"
		areq.Received = time.Now()
		areq.Files = []tricium.Data_File{
			{Path: "dir/file.txt"},
			{Path: "dir/file2.txt"},
		}

		ares := track.AnalyzeRequestResult{}
		deletedFileCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&tricium.Data_Comment{
			Category: "L",
			Message:  "Line too long",
			Path:     "dir/deleted_file.txt",
		})
		inChangeCommentJSON, err := (&jsonpb.Marshaler{}).MarshalToString(&tricium.Data_Comment{
			Category:  "L",
			Message:   "Line too short",
			Path:      "dir/file.txt",
			StartLine: 2,
			EndLine:   3,
		})
		assert.Loosely(t, err, should.BeNil)
		comments := []*track.Comment{
			{
				UUID:         "1234",
				Parent:       nil,
				Platforms:    tricium.PlatformBitPosToMask(tricium.Platform_ANY),
				Analyzer:     "analyzerName",
				Category:     "analyzerName/categoryName",
				Comment:      []byte(deletedFileCommentJSON),
				CreationTime: time.Now(),
			},
			{
				UUID:         "1234",
				Parent:       nil,
				Platforms:    tricium.PlatformBitPosToMask(tricium.Platform_IOS) | tricium.PlatformBitPosToMask(tricium.Platform_WINDOWS),
				Analyzer:     "analyzerName",
				Category:     "analyzerName/categoryName",
				Comment:      []byte(inChangeCommentJSON),
				CreationTime: time.Now(),
			},
			{
				UUID:         "1234",
				Parent:       nil,
				Platforms:    tricium.PlatformBitPosToMask(tricium.Platform_OSX),
				Analyzer:     "notSelected",
				Category:     "notSelected/categoryName",
				Comment:      []byte(inChangeCommentJSON),
				CreationTime: time.Now(),
			},
		}
		selections := []*track.CommentSelection{
			{
				ID:       1,
				Parent:   nil,
				Included: true,
			},
			{
				ID:       1,
				Parent:   nil,
				Included: true,
			},
			{
				ID:       1,
				Parent:   nil,
				Included: false,
			},
		}

		result, err := createAnalysisResults(&wres, &areq, &ares, comments, selections)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, result, should.NotBeNil)
		assert.Loosely(t, result.GerritRevision.Host, should.Equal(areq.GerritHost))
		assert.Loosely(t, result.GerritRevision.Project, should.Equal(areq.Project))
		assert.Loosely(t, result.GerritRevision.Change, should.Equal(areq.GerritChange))
		assert.Loosely(t, result.GerritRevision.GitUrl, should.Equal(areq.GitURL))
		assert.Loosely(t, result.GerritRevision.GitRef, should.Equal(areq.GitRef))
		assert.Loosely(t, result.RevisionNumber, should.Equal(7))
		assert.Loosely(t, tutils.Timestamp(result.RequestedTime), should.Match(areq.Received))
		assert.Loosely(t, len(result.Files), should.Equal(len(areq.Files)))
		for i := 0; i < len(result.Files); i++ {
			assert.Loosely(t, result.Files[i], should.Resemble(&areq.Files[i]))
		}
		assert.Loosely(t, len(result.Comments), should.Equal(len(comments)))
		for i, gcomment := range result.Comments {
			tcomment := tricium.Data_Comment{}
			err := jsonpb.UnmarshalString(string(comments[i].Comment), &tcomment)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, &tcomment, should.Resemble(gcomment.Comment))
			assert.Loosely(t, gcomment.Analyzer, should.Equal(comments[i].Analyzer))
			assert.Loosely(t, gcomment.CreatedTime, should.Resemble(tutils.TimestampProto(comments[i].CreationTime)))
			platforms, _ := tricium.GetPlatforms(comments[i].Platforms)
			assert.Loosely(t, gcomment.Platforms, should.Resemble(platforms))
			assert.Loosely(t, gcomment.Selected, should.Equal(selections[i].Included))
		}
	})
}

func TestCommentFetchingFunctions(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		// Add a request with no Gerrit details; it will not be fetched.
		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequest{ID: 11}), should.BeNil)
		// Add two requests for the same CL.
		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequest{
			ID:           22,
			GitRef:       "refs/changes/99/99/1",
			GerritHost:   "example.com",
			GerritChange: "p~master~I2222",
		}), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequest{
			ID:           23,
			GitRef:       "refs/changes/99/99/2",
			GerritHost:   "example.com",
			GerritChange: "p~master~I2222",
		}), should.BeNil)
		// And one more request with the same change ID but different host.
		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequest{
			ID:           33,
			GitRef:       "refs/changes/99/99/1",
			GerritHost:   "other.test",
			GerritChange: "p~master~I2222",
		}), should.BeNil)

		t.Run("A non-existent change has no runs", func(t *ftt.Test) {
			keys, err := fetchRequestKeysByChange(ctx, "none.test", "none~m~Iabcd")
			assert.Loosely(t, len(keys), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("No runs match if there are no Gerrit details", func(t *ftt.Test) {
			keys, err := fetchRequestKeysByChange(ctx, "none.test", "")
			assert.Loosely(t, keys, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Two keys are fetched for a change with two runs", func(t *ftt.Test) {
			keys, err := fetchRequestKeysByChange(ctx, "example.com", "p~master~I2222")
			assert.Loosely(t, len(keys), should.Equal(2))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("One key is fetched for a change with one runs", func(t *ftt.Test) {
			keys, err := fetchRequestKeysByChange(ctx, "other.test", "p~master~I2222")
			assert.Loosely(t, len(keys), should.Equal(1))
			assert.Loosely(t, err, should.BeNil)
		})

		// In addition to the runs, add some comments and comment feedback.
		// In this example, run 22 has comments, some of which have not useful feedback.
		// Run 23 (same CL, different run) has one comment with not useful feedback.
		run22WorkerKey := ds.MakeKey(
			ctx, "AnalyzeRequest", 22, "WorkflowRun", 1,
			"FunctionRun", "Foo", "WorkerRun", "Foo_UBUNTU")

		c1 := &track.Comment{Parent: run22WorkerKey, Category: "Foo/C1"}
		assert.Loosely(t, ds.Put(ctx, c1), should.BeNil)
		c1Key := ds.KeyForObj(ctx, c1)
		assert.Loosely(t, ds.Put(ctx, &track.CommentFeedback{Parent: c1Key, ID: 1, NotUsefulReports: 1}), should.BeNil)

		c2 := &track.Comment{Parent: run22WorkerKey, Category: "Foo/C2"}
		assert.Loosely(t, ds.Put(ctx, c2), should.BeNil)
		c2Key := ds.KeyForObj(ctx, c2)
		assert.Loosely(t, ds.Put(ctx, &track.CommentFeedback{Parent: c2Key, ID: 1, NotUsefulReports: 2}), should.BeNil)

		c3 := &track.Comment{Parent: run22WorkerKey, Category: "Foo/C3"}
		assert.Loosely(t, ds.Put(ctx, c3), should.BeNil)
		c3Key := ds.KeyForObj(ctx, c3)
		assert.Loosely(t, ds.Put(ctx, &track.CommentFeedback{Parent: c3Key, ID: 1, NotUsefulReports: 0}), should.BeNil)

		run23WorkerKey := ds.MakeKey(
			ctx, "AnalyzeRequest", 23, "WorkflowRun", 1,
			"FunctionRun", "Foo", "WorkerRun", "Foo_UBUNTU")

		c4 := &track.Comment{Parent: run23WorkerKey, Category: "Foo/C4"}
		assert.Loosely(t, ds.Put(ctx, c4), should.BeNil)
		c4Key := ds.KeyForObj(ctx, c4)
		assert.Loosely(t, ds.Put(ctx, &track.CommentFeedback{Parent: c4Key, ID: 1, NotUsefulReports: 1}), should.BeNil)

		t.Run("No CommentFeedback keys fetched for empty input", func(t *ftt.Test) {
			keys, err := fetchAllCommentFeedback(ctx, nil)
			assert.Loosely(t, len(keys), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("No CommentFeedback keys fetched for run with no comments", func(t *ftt.Test) {
			keys, err := fetchAllCommentFeedback(ctx, []*ds.Key{ds.MakeKey(ctx, "AnalyzeRequest", 33)})
			assert.Loosely(t, len(keys), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Two CommentFeedback keys fetched for run with two comments", func(t *ftt.Test) {
			keys, err := fetchAllCommentFeedback(ctx, []*ds.Key{ds.MakeKey(ctx, "AnalyzeRequest", 22)})
			// Comments c1 and c2 have "not useful" feedback.
			assert.Loosely(t, len(keys), should.Equal(2))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("One CommentFeedback key fetched for run with one comment", func(t *ftt.Test) {
			keys, err := fetchAllCommentFeedback(ctx, []*ds.Key{ds.MakeKey(ctx, "AnalyzeRequest", 23)})
			assert.Loosely(t, len(keys), should.Equal(1))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Three CommentFeedback keys for both of those runs together", func(t *ftt.Test) {
			keys, err := fetchAllCommentFeedback(ctx, []*ds.Key{
				ds.MakeKey(ctx, "AnalyzeRequest", 22),
				ds.MakeKey(ctx, "AnalyzeRequest", 23),
			})
			assert.Loosely(t, len(keys), should.Equal(3))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("suppressedCategories returns all not useful categories for all patchsets", func(t *ftt.Test) {
			categories := suppressedCategories(ctx, "example.com", "p~master~I2222")
			assert.Loosely(t, categories, should.Resemble(stringset.NewFromSlice("Foo/C1", "Foo/C2", "Foo/C4")))
		})

		t.Run("suppressedCategories returns an empty set for nonexistent CLs", func(t *ftt.Test) {
			categories := suppressedCategories(ctx, "example.com", "p~master~I999")
			assert.Loosely(t, categories, should.BeEmpty)
		})
	})
}
