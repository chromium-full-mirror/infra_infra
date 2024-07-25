// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package docker

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// Tests the function which escapes special characters(ex: $, ` etc`) for a command input in the form of a string array
func TestEscapeSpecialCharacters(t *testing.T) {
	t.Parallel()
	ftt.Run("Excape Special Characters", t, func(t *ftt.Test) {
		t.Run("Escapes $, \\, ` and double quote", func(t *ftt.Test) {
			err := escapeSpecialChars([]string{"\\hello$", "`testString\""})
			assert.Loosely(t, err[0], should.Equal("\\\\hello\\$"))
			assert.Loosely(t, err[1], should.Equal("\\`testString\\\""))
		})

		t.Run("Does not escape anything other than $, \\, ` and double quote", func(t *ftt.Test) {
			err := escapeSpecialChars([]string{"\\hello$^", "%`testString\""})
			assert.Loosely(t, err[0], should.Equal("\\\\hello\\$^"))
			assert.Loosely(t, err[1], should.Equal("%\\`testString\\\""))
		})

		t.Run("Empty array input - return empty array", func(t *ftt.Test) {
			err := escapeSpecialChars([]string{})
			assert.Loosely(t, err, should.HaveLength(0))
		})
	})
}
