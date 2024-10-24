// Copyright 2015 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudtail

import (
	"context"
	"net/http"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	logging "google.golang.org/api/logging/v2"
)

func TestClient(t *testing.T) {
	ftt.Run("PushEntries works", t, func(t *ftt.Test) {
		ctx := testContext()

		opts := ClientOptions{
			ClientID: ClientID{
				ResourceType: "res-type",
				ResourceID:   "res-id",
				LogID:        "log-id",
			},
			Client:    http.DefaultClient,
			ProjectID: "proj-id",
		}

		requests := []*logging.WriteLogEntriesRequest{}
		writeFunc := func(ctx context.Context, req *logging.WriteLogEntriesRequest) error {
			requests = append(requests, req)
			return nil
		}

		ts, err := time.Parse(time.RFC3339, "2015-10-02T15:00:00Z")
		assert.Loosely(t, err, should.BeNil)

		c, err := clientWithMockWrite(opts, writeFunc)
		assert.Loosely(t, err, should.BeNil)
		err = c.PushEntries(ctx, []*Entry{
			{
				InsertID:    "insert-id",
				Timestamp:   ts,
				Severity:    Debug,
				TextPayload: "hi",
			},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(requests), should.Equal(1))
		assert.Loosely(t, requests[0].LogName, should.Equal("projects/proj-id/logs/log-id"))
		assert.Loosely(t, requests[0].Labels, should.Resemble(map[string]string{
			"cloudtail/resource_id":   "res-id",
			"cloudtail/resource_type": "res-type",
		}))
		assert.Loosely(t, requests[0].Resource, should.Resemble(&logging.MonitoredResource{
			Type: "global",
			Labels: map[string]string{
				"project_id": "proj-id",
			},
		}))
		assert.Loosely(t, len(requests[0].Entries), should.Equal(1))
		assert.Loosely(t, requests[0].Entries[0], should.Resemble(&logging.LogEntry{
			InsertId:    "insert-id",
			Severity:    "DEBUG",
			TextPayload: "hi",
			Timestamp:   "2015-10-02T15:00:00Z",
		}))
	})
}

func clientWithMockWrite(opts ClientOptions, writeFunc writeFunc) (Client, error) {
	c, err := NewClient(opts)
	if err == nil {
		cl := c.(*loggingClient)
		cl.writeFunc = writeFunc
	}
	return c, err
}
