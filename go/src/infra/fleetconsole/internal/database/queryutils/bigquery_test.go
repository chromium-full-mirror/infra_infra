// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"testing"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func Test_convertQueryParameters(t *testing.T) {
	t.Parallel()

	t.Run("empty parameters", func(t *testing.T) {
		t.Parallel()
		params := []any{}
		expected := []bigquery.QueryParameter{}
		actual := convertQueryParameters(params)
		assert.Loosely(t, actual, should.Resemble(expected))
	})

	t.Run("single parameter", func(t *testing.T) {
		t.Parallel()
		params := []any{"test"}
		expected := []bigquery.QueryParameter{
			{
				Value: "test",
			},
		}
		actual := convertQueryParameters(params)
		assert.Loosely(t, actual, should.Resemble(expected))
	})

	t.Run("multiple parameters", func(t *testing.T) {
		t.Parallel()
		params := []any{"'test1'", "test2"}
		expected := []bigquery.QueryParameter{
			{
				Value: "'test1'",
			},
			{
				Value: "test2",
			},
		}
		actual := convertQueryParameters(params)
		assert.Loosely(t, actual, should.Resemble(expected))
	})
}
