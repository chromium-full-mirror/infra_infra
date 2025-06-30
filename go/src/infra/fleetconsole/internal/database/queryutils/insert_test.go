// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestInsertBuilder(t *testing.T) {
	ftt.Run("InsertBuilder", t, func(t *ftt.Test) {
		idColumn := NewColumn("id").Build()
		dutStateColumn := NewColumn("dut_state").Build()
		dutNameColumn := NewColumn("dut_name").Build()
		table := NewTableBuilder("Devices").WithColumns(
			idColumn,
			dutStateColumn,
			dutNameColumn,
		).Build()

		t.Run("insert with all columns", func(t *ftt.Test) {
			builder := NewInsertBuilder(table).Values("dev1", "ready", "name1")
			q, err := builder.Build()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{"dev1", "ready", "name1"}))
			assert.Loosely(t, q.Statement, should.Equal(`INSERT INTO "Devices" ( id, dut_state, dut_name ) VALUES ($1, $2, $3);`))
		})

		t.Run("insert with specific columns", func(t *ftt.Test) {
			builder := NewInsertBuilder(table).Columns(idColumn, dutNameColumn).Values("dev2", "name2")
			q, err := builder.Build()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{"dev2", "name2"}))
			assert.Loosely(t, q.Statement, should.Equal(`INSERT INTO "Devices" ( id, dut_name ) VALUES ($1, $2);`))
		})

		t.Run("insert with on conflict do nothing", func(t *ftt.Test) {
			builder := NewInsertBuilder(table).Values("dev3", "busy", "name3").OnConflict(ConflictOn(idColumn))
			q, err := builder.Build()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{"dev3", "busy", "name3"}))
			assert.Loosely(t, q.Statement, should.Equal(`INSERT INTO "Devices" ( id, dut_state, dut_name ) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING;`))
		})

		t.Run("insert with on conflict do update", func(t *ftt.Test) {
			builder := NewInsertBuilder(table).Values("dev4", "repair", "name4").OnConflict(ConflictOn(idColumn).Replace(dutStateColumn))
			q, err := builder.Build()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{"dev4", "repair", "name4"}))
			assert.Loosely(t, q.Statement, should.Equal(`INSERT INTO "Devices" ( id, dut_state, dut_name ) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET dut_state = EXCLUDED.dut_state;`))
		})

		t.Run("error on mismatched columns and values", func(t *ftt.Test) {
			builder := NewInsertBuilder(table).Columns(idColumn).Values("dev5", "mismatch")
			_, err := builder.Build()
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
