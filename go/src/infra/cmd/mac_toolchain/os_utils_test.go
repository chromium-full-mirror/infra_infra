// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestParseOSVersion(t *testing.T) {
	t.Parallel()

	ftt.Run("check isMacOSVersionOrLater works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("check a version that is greater than 13", func(t *ftt.Test) {
			s.ReturnOutput = []string{
				"13.0.1",
			}
			os13OrLater, err := isMacOSVersionOrLater(ctx, "13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, os13OrLater, should.Equal(true))
		})

		t.Run("check a version that is equal to 13", func(t *ftt.Test) {
			s.ReturnOutput = []string{
				"13.0.0",
			}
			os13OrLater, err := isMacOSVersionOrLater(ctx, "13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, os13OrLater, should.Equal(true))
		})

		t.Run("check a version that less than 13", func(t *ftt.Test) {
			s.ReturnOutput = []string{
				"12.1.2",
			}
			os13OrLater, err := isMacOSVersionOrLater(ctx, "13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, os13OrLater, should.Equal(false))
		})

		t.Run("invalid output should return false", func(t *ftt.Test) {
			s.ReturnOutput = []string{
				"invalid",
			}
			os13OrLater, err := isMacOSVersionOrLater(ctx, "13")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, os13OrLater, should.Equal(false))
		})

		t.Run("error output should return false", func(t *ftt.Test) {
			s.ReturnError = []error{errors.Reason("random Error").Err()}
			os13OrLater, err := isMacOSVersionOrLater(ctx, "13")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, os13OrLater, should.Equal(false))
		})
	})
}
