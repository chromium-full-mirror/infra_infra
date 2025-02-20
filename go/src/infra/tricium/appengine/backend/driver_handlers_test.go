// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"testing"
	"time"

	"google.golang.org/api/pubsub/v1"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	tq "go.chromium.org/luci/gae/service/taskqueue"

	admin "go.chromium.org/infra/tricium/api/admin/v1"
	"go.chromium.org/infra/tricium/appengine/common"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

var (
	msg = &pubsub.PubsubMessage{
		MessageId:   "58708071417623",
		PublishTime: "2017-02-28T19:39:28.104Z",
		Data:        "eyJ0YXNrX2lkIjoiMzQ5ZjBkODQ5MjI3Y2QxMCIsInVzZXJkYXRhIjoiQ0lDQWdJQ0E2TjBLRWdkaFltTmxaR1puR2hoSVpXeHNiMTlWWW5WdWRIVXhOQzR3TkY5NE9EWXROalE9In0=",
	}
	msgBB = &pubsub.PubsubMessage{
		MessageId:   "58708071417623",
		PublishTime: "2017-02-28T19:39:28.104Z",
		Data:        "eyJidWlsZCI6eyJpZCI6IjEyMzQifSwidXNlcmRhdGEiOiJDSUNBZ0lDQTZOMEtFZ2RoWW1ObFpHWm5HaGhJWld4c2IxOVZZblZ1ZEhVeE5DNHdORjk0T0RZdE5qUT0ifQ==",
	}
	msgBBV2 = &pubsub.PubsubMessage{
		MessageId:   "58708071417624",
		PublishTime: "2023-01-30T19:39:28.104Z",
		Data:        "eyJidWlsZFB1YnN1YiI6eyJidWlsZCI6eyJpZCI6IjEyMzQiLCAiYnVpbGRlciI6eyJwcm9qZWN0IjoicHJvamVjdCIsICJidWNrZXQiOiJidWNrZXQiLCAiYnVpbGRlciI6ImJ1aWxkZXIifX0sICJidWlsZExhcmdlRmllbGRzIjoiZUp5cVltaGlaQUFFQUFELy93UHZBUDQ9In0sICJ1c2VyRGF0YSI6IlEwbERRV2RKUTBFMlRqQkxSV2RrYUZsdFRteGFSMXB1UjJob1NWcFhlSE5pTVRsV1dXNVdkV1JJVlhoT1F6UjNUa1k1TkU5RVdYUk9hbEU5In0=",
		Attributes:  map[string]string{"version": "v2"},
	}
	buildID       = 1234             // matches the above pubsub message
	runID   int64 = 6042091272536064 // matches the above pubsub messages
)

func TestDecodePubsubMessage(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		t.Run("Decodes pubsub message without error", func(t *ftt.Test) {
			_, _, err := decodePubsubMessage(ctx, msg)
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestEnqueueCollectRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		t.Run("Enqueued task shouldn't start until after delay time is up", func(t *ftt.Test) {
			assert.Loosely(t, enqueueCollectRequest(ctx, &admin.CollectRequest{}, 7*time.Minute), should.BeNil)

			task := tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]["5023444679101355902"]
			// ETA is the earliest time that the task should execute; an ETA of now
			// + delay means that the task should start after a delay. When ETA is set
			// on the task, then Delay is unset.
			assert.Loosely(t, task.ETA, should.Match(clock.Now(ctx).Add(7*time.Minute)))
			assert.Loosely(t, task.Delay, should.Match(time.Duration(0)))
		})
	})
}

func TestHandlePubSubMessage(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		t.Run("Enqueues buildbucket collect task", func(t *ftt.Test) {
			t.Run("Buildbucket old message", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.BeZero)
				received := &ReceivedPubSubMessage{ID: fmt.Sprintf("%d:%d", buildID, runID)}
				assert.Loosely(t, ds.Get(ctx, received), should.Equal(ds.ErrNoSuchEntity))
				err := handlePubSubMessage(ctx, msgBB)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, ds.Get(ctx, received), should.BeNil)
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.Equal(1))
			})
			t.Run("Buildbucket new message", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.BeZero)
				received := &ReceivedPubSubMessage{ID: fmt.Sprintf("%d:%d", buildID, runID)}
				assert.Loosely(t, ds.Get(ctx, received), should.Equal(ds.ErrNoSuchEntity))
				err := handlePubSubMessage(ctx, msgBBV2)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, ds.Get(ctx, received), should.BeNil)
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.Equal(1))
			})
		})

		t.Run("Avoids duplicate processing", func(t *ftt.Test) {
			t.Run("Buildbucket old message", func(t *ftt.Test) {
				assert.Loosely(t, handlePubSubMessage(ctx, msgBB), should.BeNil)
				assert.Loosely(t, handlePubSubMessage(ctx, msgBB), should.BeNil)
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.Equal(1))
			})
			t.Run("Buildbucket new message", func(t *ftt.Test) {
				assert.Loosely(t, handlePubSubMessage(ctx, msgBBV2), should.BeNil)
				assert.Loosely(t, handlePubSubMessage(ctx, msgBBV2), should.BeNil)
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.Equal(1))
			})
		})
	})
}
