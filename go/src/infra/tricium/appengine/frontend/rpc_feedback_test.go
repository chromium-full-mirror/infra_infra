// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"

	"infra/tricium/api/v1"
	"infra/tricium/appengine/common/track"
	"infra/tricium/appengine/common/triciumtest"
)

func TestFeedback(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		now := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, tc := testclock.UseTime(ctx, now)
		ds.GetTestable(ctx).AddIndexes(&ds.IndexDefinition{
			Kind: "Comment",
			SortBy: []ds.IndexColumn{
				{Property: "Analyzer"},
				{Property: "CreationTime"},
			},
		})
		ds.GetTestable(ctx).CatchupIndexes()
		// Add comment entities with ancestors:
		// AnalyzeRequest>WorkflowRun>FunctionRun>WorkerRun>Comment>CommentFeedback
		stime := tc.Now().UTC().Add(3 * time.Minute)
		etime := tc.Now().UTC().Add(7 * time.Minute)
		ctime1 := tc.Now().UTC().Add(2 * time.Minute) // before stime
		ctime2 := tc.Now().UTC().Add(4 * time.Minute) // between stime and etime
		ctx, _ = testclock.UseTime(ctx, now.Add(10*time.Minute))
		commentID1 := "7ef59cda-183c-48b3-8343-d9036a7f1419"
		commentID2 := "9400f12d-b425-4cf6-85d5-5636fc4e55a4"
		functionName := "Spacey"
		category1 := "Spacey/MixedSpace"
		category2 := "Spacey/TrailingSpace"
		platform := tricium.Platform_UBUNTU
		request := &track.AnalyzeRequest{}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		run := &track.WorkflowRun{ID: 1, Parent: ds.KeyForObj(ctx, request)}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		functionRun := &track.FunctionRun{ID: functionName, Parent: ds.KeyForObj(ctx, run)}
		assert.Loosely(t, ds.Put(ctx, functionRun), should.BeNil)
		worker := &track.WorkerRun{
			ID:     fmt.Sprintf("%s_%s", functionName, platform),
			Parent: ds.KeyForObj(ctx, functionRun),
		}
		assert.Loosely(t, ds.Put(ctx, worker), should.BeNil)
		comment1 := &track.Comment{
			UUID:         commentID1,
			Parent:       ds.KeyForObj(ctx, worker),
			Platforms:    1,
			Analyzer:     functionName,
			Category:     category1,
			CreationTime: ctime1,
		}
		assert.Loosely(t, ds.Put(ctx, comment1), should.BeNil)
		feedback1 := &track.CommentFeedback{
			ID:               1,
			Parent:           ds.KeyForObj(ctx, comment1),
			NotUsefulReports: 2,
		}
		assert.Loosely(t, ds.Put(ctx, feedback1), should.BeNil)
		selection1 := &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comment1),
			Included: true,
		}
		assert.Loosely(t, ds.Put(ctx, selection1), should.BeNil)
		comment2 := &track.Comment{
			UUID:         commentID2,
			Parent:       ds.KeyForObj(ctx, worker),
			Platforms:    1,
			Analyzer:     functionName,
			Category:     category2,
			CreationTime: ctime2,
		}
		assert.Loosely(t, ds.Put(ctx, comment2), should.BeNil)
		feedback2 := &track.CommentFeedback{
			ID:               1,
			Parent:           ds.KeyForObj(ctx, comment2),
			NotUsefulReports: 1,
		}
		assert.Loosely(t, ds.Put(ctx, feedback2), should.BeNil)
		selection2 := &track.CommentSelection{
			ID:       1,
			Parent:   ds.KeyForObj(ctx, comment2),
			Included: true,
		}
		assert.Loosely(t, ds.Put(ctx, selection2), should.BeNil)

		t.Run("Feedback request for unknown category", func(t *ftt.Test) {
			st, et, _ := parseTimeRange(ctx, nil, nil)
			count, reports, err := feedback(ctx, "Hello", st, et)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, count, should.BeZero)
			assert.Loosely(t, reports, should.BeZero)
		})

		t.Run("Feedback request for known analyzer name", func(t *ftt.Test) {
			st, et, _ := parseTimeRange(ctx, nil, nil)
			count, reports, err := feedback(ctx, functionName, st, et)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, count, should.Equal(2))
			assert.Loosely(t, reports, should.Equal(3))
		})

		t.Run("Feedback request for time period", func(t *ftt.Test) {
			count, reports, err := feedback(ctx, functionName, stime, etime)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, count, should.Equal(1))
			assert.Loosely(t, reports, should.Equal(1))
		})

		t.Run("Feedback request for subcategory", func(t *ftt.Test) {
			st, et, _ := parseTimeRange(ctx, nil, nil)
			count, reports, err := feedback(ctx, category1, st, et)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, count, should.Equal(1))
			assert.Loosely(t, reports, should.Equal(2))
		})
	})
}

func TestParseTimeRange(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		epoch := time.Unix(0, 0).UTC()
		now := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, _ = testclock.UseTime(ctx, now)

		t.Run("No start or end specified", func(t *ftt.Test) {
			st, et, err := parseTimeRange(ctx, nil, nil)
			assert.Loosely(t, st, should.Match(epoch))
			assert.Loosely(t, et, should.Match(now))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Both start and end time specified", func(t *ftt.Test) {
			start := time.Date(2016, 11, 5, 0, 0, 0, 0, time.UTC)
			end := time.Date(2016, 11, 8, 0, 0, 0, 0, time.UTC)
			startTimestamp, err := ptypes.TimestampProto(start)
			assert.Loosely(t, err, should.BeNil)
			endTimestamp, err := ptypes.TimestampProto(end)
			assert.Loosely(t, err, should.BeNil)
			st, et, err := parseTimeRange(ctx, startTimestamp, endTimestamp)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, st, should.Match(start))
			assert.Loosely(t, et, should.Match(end))
		})

		t.Run("Reversed time", func(t *ftt.Test) {
			start := time.Date(2016, 11, 8, 0, 0, 0, 0, time.UTC)
			end := time.Date(2016, 11, 5, 0, 0, 0, 0, time.UTC)
			startTimestamp, err := ptypes.TimestampProto(start)
			assert.Loosely(t, err, should.BeNil)
			endTimestamp, err := ptypes.TimestampProto(end)
			assert.Loosely(t, err, should.BeNil)
			_, _, err = parseTimeRange(ctx, startTimestamp, endTimestamp)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
