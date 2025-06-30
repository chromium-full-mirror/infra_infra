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

func TestSelectClauseBuilder(t *testing.T) {
	ftt.Run("SelectClauseBuilder", t, func(t *ftt.Test) {
		t.Run("SelectField", func(t *ftt.Test) {
			var fieldBinding any
			fieldClauseBuilder := Select("field", &fieldBinding)
			assert.Loosely(t, fieldClauseBuilder.fieldSelectClause, should.Equal("field"))
		})
		t.Run("CountAll", func(t *ftt.Test) {
			var field int
			fieldClauseBuilder := CountAll(&field)
			assert.Loosely(t, fieldClauseBuilder.fieldSelectClause, should.Equal("COUNT(*)"))
		})
		t.Run("CountWhereEquals", func(t *ftt.Test) {
			var field int
			fieldClauseBuilder := CountIf("field", &field).Equals("1")
			assert.Loosely(t, fieldClauseBuilder.fieldSelectClause, should.Equal("SUM(CASE WHEN field = \"1\" THEN 1 ELSE 0 END)"))
		})
	})
}
