// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestSchedulingUnitDutState(t *testing.T) {
	ftt.Run("Test when all child DUTs in ready.", t, func(t *ftt.Test) {
		s := []string{"ready", "ready", "ready", "ready", "ready"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("ready"))
	})

	ftt.Run("Test when where one child DUT in needs_repair.", t, func(t *ftt.Test) {
		s := []string{"ready", "ready", "ready", "ready", "needs_repair"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("needs_repair"))
	})

	ftt.Run("Test when where one child DUT in repair_failed.", t, func(t *ftt.Test) {
		s := []string{"ready", "ready", "ready", "needs_repair", "repair_failed"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("repair_failed"))
	})

	ftt.Run("Test when where one child DUT in needs_manual_repair.", t, func(t *ftt.Test) {
		s := []string{"ready", "ready", "needs_manual_repair", "needs_repair", "repair_failed"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("needs_manual_repair"))
	})

	ftt.Run("Test when where one child DUT in needs_replacement.", t, func(t *ftt.Test) {
		s := []string{"ready", "needs_deploy", "needs_replacement", "needs_repair", "needs_manual_repair"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("needs_replacement"))
	})

	ftt.Run("Test when where one child DUT in needs_deploy.", t, func(t *ftt.Test) {
		s := []string{"ready", "ready", "needs_deploy", "needs_manual_repair", "repair_failed"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("needs_deploy"))
	})

	ftt.Run("Test when where one child DUT in reserved.", t, func(t *ftt.Test) {
		s := []string{"ready", "reserved", "needs_deploy", "needs_repair", "needs_replacement"}
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("reserved"))
	})

	ftt.Run("Test when input is an empty slice", t, func(t *ftt.Test) {
		var s []string
		assert.Loosely(t, SchedulingUnitDutState(s), should.Equal("unknown"))
	})
}
