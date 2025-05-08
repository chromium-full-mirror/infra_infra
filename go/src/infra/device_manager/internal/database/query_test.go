// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package database

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestBuildQueryFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tests := []struct {
		name      string
		filter    string
		wantQuery string
		wantArgs  []any
	}{
		{
			name:      "empty filter",
			filter:    "",
			wantQuery: "",
			wantArgs:  nil,
		},
		{
			name:      "simple equality",
			filter:    "name = dut-name",
			wantQuery: "WHERE name = $1",
			wantArgs:  []any{"dut-name"},
		},
		{
			name:      "multiple equalities",
			filter:    "name = dut-name AND is_active = true",
			wantQuery: "WHERE name = $1 AND is_active = $2",
			wantArgs:  []any{"dut-name", "true"},
		},
		{
			name:      "inequality gt",
			filter:    "created_time > 2024-05-01",
			wantQuery: "WHERE created_time > $1",
			wantArgs:  []any{"2024-05-01"},
		},
		{
			name:      "inequality gte",
			filter:    "created_time >= 2024-05-01",
			wantQuery: "WHERE created_time >= $1",
			wantArgs:  []any{"2024-05-01"},
		},
		{
			name:      "inequality lt",
			filter:    "created_time < 2024-05-01",
			wantQuery: "WHERE created_time < $1",
			wantArgs:  []any{"2024-05-01"},
		},
		{
			name:      "inequality lte",
			filter:    "created_time <= 2024-05-01",
			wantQuery: "WHERE created_time <= $1",
			wantArgs:  []any{"2024-05-01"},
		},
		{
			name:      "inequality ne",
			filter:    "created_time != 2024-05-01",
			wantQuery: "WHERE created_time != $1",
			wantArgs:  []any{"2024-05-01"},
		},
		{
			name:      "mixed operators",
			filter:    "name = dut-name AND created_time > 2024-05-01",
			wantQuery: "WHERE name = $1 AND created_time > $2",
			wantArgs:  []any{"dut-name", "2024-05-01"},
		},
		{
			name:      "single operator with multiple operands",
			filter:    "created_time IS NOT NULL",
			wantQuery: "WHERE created_time IS NOT NULL",
			wantArgs:  nil,
		},
		{
			name:      "mixed operators with multiple operands",
			filter:    "name = dut-name AND created_time IS NOT NULL",
			wantQuery: "WHERE name = $1 AND created_time IS NOT NULL",
			wantArgs:  []any{"dut-name"},
		},
		{
			name:      "extra AND",
			filter:    "name = dut-name AND ",
			wantQuery: "WHERE name = $1",
			wantArgs:  []any{"dut-name"},
		},
		{
			name:      "unsupported operator",
			filter:    "name XOR test-value",
			wantQuery: "",
			wantArgs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotQuery, gotArgs := BuildQueryFilter(ctx, tt.filter)

			if strings.TrimSpace(gotQuery) != tt.wantQuery {
				t.Errorf("BuildQueryFilter() gotQuery = %v, want %v", gotQuery, tt.wantQuery)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("BuildQueryFilter() gotArgs = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}
