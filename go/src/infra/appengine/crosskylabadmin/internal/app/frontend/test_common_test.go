// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestBotForDUT(t *testing.T) {
	ftt.Run("empty dimensions", t, func(t *ftt.Test) {
		assert.Loosely(t, BotForDUT("dut1", "", ""), should.Resemble(&swarmingv2.BotInfo{
			BotId: "bot_dut1",
			Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{""}},
				{Key: "dut_id", Value: []string{"dut1"}},
				{Key: "dut_name", Value: []string{"dut1-host"}},
			},
		}))
	})

	ftt.Run("non-trivial dimensions with whitespace", t, func(t *ftt.Test) {
		assert.Loosely(t, BotForDUT("dut1", "fake_state", "a: x, y ; b :z"), should.Resemble(&swarmingv2.BotInfo{
			BotId: "bot_dut1",
			Dimensions: []*swarmingv2.StringListPair{
				{Key: "a", Value: []string{"x", "y"}},
				{Key: "b", Value: []string{"z"}},
				{Key: "dut_state", Value: []string{"fake_state"}},
				{Key: "dut_id", Value: []string{"dut1"}},
				{Key: "dut_name", Value: []string{"dut1-host"}},
			},
		},
		))
	})
}
