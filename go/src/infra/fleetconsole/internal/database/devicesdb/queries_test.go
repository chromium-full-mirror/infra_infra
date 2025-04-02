// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestBuildListDevicesQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tests := []struct {
		name               string
		offset             int
		pageSize           int
		filter             string
		orderby            string
		ids                []string
		realms             []string
		expectedStatement  string
		expectedParameters []any
	}{
		{
			name:               "list the first page with page size 10",
			offset:             0,
			pageSize:           10,
			filter:             "",
			orderby:            "",
			ids:                []string{},
			realms:             []string{},
			expectedStatement:  "SELECT id, dut_id, host, port, type, state, labels\nFROM \"Devices\"\nWHERE (realm IS NULL)\nORDER BY id\nLIMIT 10\nOFFSET 0;",
			expectedParameters: nil,
		},
		{
			name:               "list all devices",
			offset:             0,
			pageSize:           -1,
			filter:             "",
			orderby:            "",
			ids:                []string{},
			realms:             []string{},
			expectedStatement:  "SELECT id, dut_id, host, port, type, state, labels\nFROM \"Devices\"\nWHERE (realm IS NULL)\nORDER BY id\n;",
			expectedParameters: nil,
		},
		{
			name:               "list devices with particular ids and with user having particular realm",
			offset:             0,
			pageSize:           -1,
			filter:             "",
			orderby:            "",
			ids:                []string{"device1", "device2"},
			realms:             []string{"default"},
			expectedStatement:  "SELECT id, dut_id, host, port, type, state, labels\nFROM \"Devices\"\nWHERE (id IN ($1,$2)) AND (realm IN ($3) OR realm IS NULL)\nORDER BY id\n;",
			expectedParameters: []any{"device1", "device2", "default"},
		},
		{
			name:               "order by label's field",
			offset:             0,
			pageSize:           -1,
			filter:             "",
			orderby:            "labels.label-pool",
			ids:                []string{},
			realms:             []string{},
			expectedStatement:  "SELECT id, dut_id, host, port, type, state, labels\nFROM \"Devices\"\nWHERE (realm IS NULL)\nORDER BY labels -> $1 -> $2, id\n;",
			expectedParameters: []any{"label-pool", "Values"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q, err := buildListDevicesQuery(ctx, tt.offset, tt.pageSize, tt.filter, tt.orderby, tt.ids, tt.realms)

			assert.Loosely(t, err, should.BeNil)

			if strings.TrimSpace(q.Statement) != tt.expectedStatement {
				t.Errorf("Statement\ngot: %v\n\nexpected: %v", q.Statement, tt.expectedStatement)
			}
			if !reflect.DeepEqual(q.Parameters, tt.expectedParameters) {
				t.Errorf("Parameters\ngot: %v\nexpected: %v", q.Parameters, tt.expectedParameters)
			}
		})
	}
}
