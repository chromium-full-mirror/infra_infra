// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestMigrate(t *testing.T) {
	t.Parallel()

	ftt.Run(`FilterEmptyLines`, t, func(t *ftt.Test) {
		t.Run(`Works`, func(t *ftt.Test) {
			actual := filterEmptyLines([]string{
				"",
				"",
				"joe@example.com",
				"",
				"",
				"",
				"doe@example.com",
				"",
				"",
			})
			assert.Loosely(t, actual, should.Resemble([]string{
				"joe@example.com",
				"",
				"doe@example.com",
				"",
			}))
		})
		t.Run(`Empty`, func(t *ftt.Test) {
			actual := filterEmptyLines([]string{
				"",
				"",
			})
			assert.Loosely(t, actual, should.Resemble([]string{}))
		})
	})
}
