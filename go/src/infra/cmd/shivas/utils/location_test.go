// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestIsLocation(t *testing.T) {
	ftt.Run("test standard chromeos location", t, func(t *ftt.Test) {
		location := []string{
			"chromeos1-row2-rack3-host4",
			"chromeos1-row2-rack3-hostxxx",
			"ROW1-RACK2-HOST3",
			"chromeos6-floor",
			"chromeos6-rack1",
			"chromeos6-storage1",
			"2081-storage1",
			"em25-desk-stagenut",
			"chromeos6-row2-rack23-labstation1",
			"chromeos6-row2-rack23-labstation",
		}
		for _, l := range location {
			assert.Loosely(t, IsLocation(l), should.BeTrue)
		}
	})

	ftt.Run("test invalid chromeos location", t, func(t *ftt.Test) {
		location := "chromeos1-row2-rack3"
		assert.Loosely(t, IsLocation(location), should.BeFalse)
	})
}
