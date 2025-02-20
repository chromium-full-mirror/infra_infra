// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bq_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"cloud.google.com/go/bigquery"
	bqapi "google.golang.org/api/bigquery/v2"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/result_flow"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/result_flow/internal/bq"
)

// Original TestRAMBufferedBQInserter was at "go.chromium.org/infra/qscheduler/service/app/eventlog/ram_test.go".
// Tests here verify that insert_id is propagated correctly because it is important for deduplication.

func TestRamBufferedBQInserter(t *testing.T) {
	fakeConfig := &result_flow.BigqueryConfig{
		Project: "test-project",
		Dataset: "test-dataset",
		Table:   "test-table",
	}

	ftt.Run("With mock context", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx = logging.SetLevel(gologger.StdConfig.Use(ctx), logging.Debug)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		t.Run("verify insertIDs", func(t *ftt.Test) {
			var lock sync.Mutex
			insertIDs := stringset.Set{}
			fakeOptions := bq.Options{
				Target:     fakeConfig,
				HTTPClient: nil,
				InsertRPCMock: func(_ context.Context, req *bqapi.TableDataInsertAllRequest) (*bqapi.TableDataInsertAllResponse, error) {
					lock.Lock()
					for _, row := range req.Rows {
						insertIDs.Add(row.InsertId)
					}
					lock.Unlock()
					return &bqapi.TableDataInsertAllResponse{}, nil
				},
			}
			entries := make([]bigquery.ValueSaver, 5)
			for i := 0; i < 5; i++ {
				entries[i] = mkTestEntry(fmt.Sprintf("given:%d", i))
			}
			bi, _ := bq.NewInserter(ctx, fakeOptions)
			assert.Loosely(t, bi.Insert(ctx, entries...), should.BeNil)
			bi.CloseAndDrain(ctx)
			// All rows should have non-empty insert_id.
			assert.Loosely(t, insertIDs.Has(""), should.BeFalse)
			// insert_id should be recognized.
			assert.Loosely(t, insertIDs.HasAll("given:0", "given:1", "given:2", "given:3", "given:4"), should.BeTrue)
			assert.Loosely(t, insertIDs.Len(), should.Equal(5))
		})
	})
}

func mkTestEntry(insertID string) bigquery.ValueSaver {
	return &testEntry{InsertID: insertID, Data: map[string]bigquery.Value{
		"uid": insertID,
	}}
}

type testEntry struct {
	InsertID string
	Data     map[string]bigquery.Value
}

// Save implements bigquery.ValueSaver interface.
func (e testEntry) Save() (map[string]bigquery.Value, string, error) {
	return e.Data, e.InsertID, nil
}
