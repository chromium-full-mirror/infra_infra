// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/registry"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/comparison"
	"go.chromium.org/luci/common/testing/truth/failure"
	"go.chromium.org/luci/common/testing/truth/should"
)

func init() {
	registry.RegisterCmpOption(cmp.AllowUnexported(tableRow{}))
}

type savedValue struct {
	insertID string
	row      map[string]bigquery.Value
}

func value(t testing.TB, insertID, jsonVal string) savedValue {
	t.Helper()
	v := savedValue{insertID: insertID}
	assert.Loosely(t, json.Unmarshal([]byte(jsonVal), &v.row), should.BeNil, truth.LineContext())
	return v
}

func doReadInput(data string, jsonList bool, extraColumns string) ([]savedValue, error) {
	var cols map[string]bigquery.Value
	if extraColumns != "" {
		err := json.Unmarshal([]byte(extraColumns), &cols)
		if err != nil {
			return nil, err
		}
	}

	savers, err := readInput(strings.NewReader(data), "seed", jsonList, cols)
	if err != nil {
		return nil, err
	}
	out := make([]savedValue, len(savers))
	for i, saver := range savers {
		if out[i].row, out[i].insertID, err = saver.Save(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func TestReadInput(t *testing.T) {
	t.Parallel()

	ftt.Run("Empty", t, func(t *ftt.Test) {
		vals, err := doReadInput("", false, "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, vals, should.HaveLength(0))
	})

	ftt.Run("Whitespace only", t, func(t *ftt.Test) {
		vals, err := doReadInput("\n  \n\n  \n  ", false, "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, vals, should.HaveLength(0))
	})

	ftt.Run("One line", t, func(t *ftt.Test) {
		vals, err := doReadInput(`{"k": "v"}`, false, "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, vals, should.Resemble([]savedValue{
			value(t, "seed:0", `{"k": "v"}`),
		}))
	})

	ftt.Run("A bunch of lines (with spaces)", t, func(t *ftt.Test) {
		vals, err := doReadInput(`
			{"k": "v1"}

			{"k": "v2"}
			{"k": "v3"}

		`, false, "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, vals, should.Resemble([]savedValue{
			value(t, "seed:0", `{"k": "v1"}`),
			value(t, "seed:1", `{"k": "v2"}`),
			value(t, "seed:2", `{"k": "v3"}`),
		}))
	})

	ftt.Run("Broken line", t, func(t *ftt.Test) {
		_, err := doReadInput(`
			{"k": "v1"}

			{"k": "v2
			{"k": "v2"}
		`, false, "")
		assert.Loosely(t, err, should.ErrLike(`bad input line 4: bad JSON - unexpected end of JSON input`))
	})

	ftt.Run("JSON List", t, func(t *ftt.Test) {
		out, err := doReadInput(`[
			{"k": "v1"},
			{"k": "v2"},
			{"k": "v2"}
		]`, true, "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Resemble([]savedValue{
			value(t, "seed:0", `{"k": "v1"}`),
			value(t, "seed:1", `{"k": "v2"}`),
			value(t, "seed:2", `{"k": "v2"}`),
		}))
	})

	ftt.Run("JSON List (with extra columns)", t, func(t *ftt.Test) {
		out, err := doReadInput(`[
			{"k1": "v1", "k3": "v1"},
			{"k1": "v2"},
			{"k1": "v2", "k4":"v5"}
		]`, true, `{"k1": "v3", "k3": "v4"}`)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Resemble([]savedValue{
			value(t, "seed:0", `{"k1": "v3", "k3": "v4"}`),
			value(t, "seed:1", `{"k1": "v3", "k3": "v4"}`),
			value(t, "seed:2", `{"k1": "v3", "k3": "v4", "k4":"v5"}`),
		}))
	})

	ftt.Run("lines (with extra columns)", t, func(t *ftt.Test) {
		out, err := doReadInput(`
			{"k1": "v1", "k3": "v1"}
			{"k1": "v2"}
			{"k1": "v2", "k4":"v5"}
		`, false, `{"k1": "v3", "k3": "v4"}`)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Resemble([]savedValue{
			value(t, "seed:0", `{"k1": "v3", "k3": "v4"}`),
			value(t, "seed:1", `{"k1": "v3", "k3": "v4"}`),
			value(t, "seed:2", `{"k1": "v3", "k3": "v4", "k4":"v5"}`),
		}))
	})

	ftt.Run("Huge line", t, func(t *ftt.Test) {
		// Note: this breaks bufio.Scanner with "token too long" error.
		huge := fmt.Sprintf(`{"k": %q}`, strings.Repeat("x", 100000))
		vals, err := doReadInput(huge, false, "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, vals, should.Resemble([]savedValue{
			value(t, "seed:0", huge),
		}))
	})
}

type fakeInserter struct {
	calls            [][]*tableRow
	mu               sync.Mutex
	failingInsertIDs []string
}

func (i *fakeInserter) Put(_ context.Context, src interface{}) error {
	rows := src.([]*tableRow)
	i.mu.Lock()
	i.calls = append(i.calls, rows)
	i.mu.Unlock()

	var multiErr bigquery.PutMultiError
	for _, row := range rows {
		for _, id := range i.failingInsertIDs {
			if id == row.insertID {
				multiErr = append(multiErr, bigquery.RowInsertionError{InsertID: id})
			}
		}
	}
	if len(multiErr) > 0 {
		return multiErr
	}
	return nil
}

func TestDoInsert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	opts := uploadOpts{batchSize: 3}

	ftt.Run("One batch", t, func(t *ftt.Test) {
		rows := []*tableRow{
			{insertID: "1"},
			{insertID: "2"},
			{insertID: "3"},
		}
		var ins fakeInserter
		err := doInsert(ctx, io.Discard, &opts, &ins, rows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ins.calls, shouldMatchUnsorted([][]*tableRow{rows}))
	})

	ftt.Run("Multiple batches", t, func(t *ftt.Test) {
		rows := []*tableRow{
			{insertID: "1"},
			{insertID: "2"},
			{insertID: "3"},
			{insertID: "4"},
			{insertID: "5"},
			{insertID: "6"},
			{insertID: "7"},
		}
		var ins fakeInserter
		err := doInsert(ctx, io.Discard, &opts, &ins, rows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ins.calls, shouldMatchUnsorted([][]*tableRow{
			rows[0:3],
			rows[3:6],
			rows[6:7],
		}))
	})

	ftt.Run("Multiple batches with failures", t, func(t *ftt.Test) {
		rows := []*tableRow{
			{insertID: "1"},
			{insertID: "2"},
			{insertID: "3"},
			{insertID: "4"},
			{insertID: "5"},
			{insertID: "6"},
			{insertID: "7"},
		}
		ins := fakeInserter{
			failingInsertIDs: []string{"1", "5"},
		}
		err := doInsert(ctx, io.Discard, &opts, &ins, rows)
		multiErr := err.(bigquery.PutMultiError)
		// Sort in case insertions happened out-of-order.
		sort.Slice(multiErr, func(i, j int) bool {
			return multiErr[i].InsertID < multiErr[j].InsertID
		})
		assert.Loosely(t, err, should.Resemble(bigquery.PutMultiError{
			bigquery.RowInsertionError{InsertID: "1"},
			bigquery.RowInsertionError{InsertID: "5"},
		}))
		assert.Loosely(t, ins.calls, shouldMatchUnsorted([][]*tableRow{
			rows[0:3],
			rows[3:6],
			rows[6:7],
		}))
	})
}

// shouldMatchUnsorted is like should.Match, but operates on [][]*tableRow
// and ignores the ordering of the `actual` slice, since the ordering doesn't
// matter as long as all rows are uploaded. The `expected` slice must already be
// sorted by insert ID.
func shouldMatchUnsorted(expected [][]*tableRow) comparison.Func[[][]*tableRow] {
	return func(actual [][]*tableRow) *failure.Summary {
		sort.Slice(actual, func(i, j int) bool {
			return actual[i][0].insertID < actual[j][0].insertID
		})
		ret := should.Match(expected)(actual)
		if ret != nil {
			ret.Comparison.Name = "shouldMatchUnsorted"
		}
		return ret
	}
}
