// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utilization

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestReportMetrics(t *testing.T) {
	ftt.Run("with fake tsmon context", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx, _ = tsmon.WithDummyInMemory(ctx)

		t.Run("ReportMetric for single bot should report 0 for unknown statuses", func(t *ftt.Test) {
			ReportMetrics(ctx, []*swarmingv2.BotInfo{
				{State: "", Dimensions: []*swarmingv2.StringListPair{}},
			})
			assert.Loosely(t, dutmonMetric.Get(ctx, "[None]", "[None]", "[None]", "[None]", "[None]", false), should.Equal(1))

			assert.Loosely(t, dutmonMetric.Get(ctx, "[None]", "[None]", "[None]", "[None]", "NeedsRepair", false), should.BeZero)
			assert.Loosely(t, dutmonMetric.Get(ctx, "[None]", "[None]", "[None]", "[None]", "Running", false), should.BeZero)
			assert.Loosely(t, dutmonMetric.Get(ctx, "[None]", "[None]", "[None]", "[None]", "RepairFailed", false), should.BeZero)
			assert.Loosely(t, dutmonMetric.Get(ctx, "[None]", "[None]", "[None]", "[None]", "Ready", false), should.BeZero)
			assert.Loosely(t, dutmonMetric.Get(ctx, "[None]", "[None]", "[None]", "[None]", "NeedsReset", false), should.BeZero)
		})

		t.Run("ReportMetric for multiple bots with same fields should count up", func(t *ftt.Test) {
			bi := &swarmingv2.BotInfo{State: "IDLE", Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{"ready"}},
				{Key: "label-board", Value: []string{"reef"}},
				{Key: "label-model", Value: []string{"electro"}},
				{Key: "label-pool", Value: []string{"some_random_pool"}},
				{Key: "label-zone", Value: []string{"some_random_zone"}},
			}}
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi, bi, bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "some_random_pool", "some_random_zone", "Ready", false), should.Equal(3))
		})

		t.Run("ReportMetric should report dut_state as Running when dut_state is ready and task id is not null", func(t *ftt.Test) {
			bi := &swarmingv2.BotInfo{State: "BUSY", TaskId: "foobar", Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{"ready"}},
				{Key: "label-board", Value: []string{"reef"}},
				{Key: "label-model", Value: []string{"electro"}},
				{Key: "label-pool", Value: []string{"some_random_pool"}},
				{Key: "label-zone", Value: []string{"some_random_zone"}},
			}}
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi, bi, bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "some_random_pool", "some_random_zone", "Running", false), should.Equal(3))
		})

		t.Run("ReportMetric with managed pool should report pool correctly", func(t *ftt.Test) {
			bi := &swarmingv2.BotInfo{State: "IDLE", Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{"ready"}},
				{Key: "label-board", Value: []string{"reef"}},
				{Key: "label-model", Value: []string{"electro"}},
				{Key: "label-pool", Value: []string{"DUT_POOL_CQ"}},
				{Key: "label-zone", Value: []string{"ZONE_CHROMEOS6"}},
			}}
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "managed:DUT_POOL_CQ", "ZONE_CHROMEOS6", "Ready", false), should.Equal(1))
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "DUT_POOL_CQ", "ZONE_CHROMEOS6", "Ready", false), should.BeZero)
		})

		t.Run("Multiple calls to ReportMetric keep metric unchanged", func(t *ftt.Test) {
			bi := &swarmingv2.BotInfo{State: "IDLE", Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{"ready"}},
				{Key: "label-board", Value: []string{"reef"}},
				{Key: "label-model", Value: []string{"electro"}},
				{Key: "label-pool", Value: []string{"some_random_pool"}},
				{Key: "label-zone", Value: []string{"some_random_zone"}},
			}}
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi, bi, bi})
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi, bi, bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "some_random_pool", "some_random_zone", "Ready", false), should.Equal(3))
		})

		t.Run("ReportMetric should stop counting bots that disappear", func(t *ftt.Test) {
			bi := &swarmingv2.BotInfo{State: "IDLE", Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{"ready"}},
				{Key: "label-board", Value: []string{"reef"}},
				{Key: "label-model", Value: []string{"electro"}},
				{Key: "label-pool", Value: []string{"some_random_pool"}},
				{Key: "label-zone", Value: []string{"some_random_zone"}},
			}}
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi, bi, bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "some_random_pool", "some_random_zone", "Ready", false), should.Equal(3))
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "some_random_pool", "some_random_zone", "Ready", false), should.Equal(1))
		})

		t.Run("ReportMetric should report repair_failed bots as RepairFailed", func(t *ftt.Test) {
			bi := &swarmingv2.BotInfo{State: "IDLE", Dimensions: []*swarmingv2.StringListPair{
				{Key: "dut_state", Value: []string{"repair_failed"}},
				{Key: "label-board", Value: []string{"reef"}},
				{Key: "label-model", Value: []string{"electro"}},
				{Key: "label-pool", Value: []string{"some_random_pool"}},
				{Key: "label-zone", Value: []string{"some_random_zone"}},
			}}
			ReportMetrics(ctx, []*swarmingv2.BotInfo{bi})
			assert.Loosely(t, dutmonMetric.Get(ctx, "reef", "electro", "some_random_pool", "some_random_zone", "RepairFailed", false), should.Equal(1))
		})

	})
}

func strPtr(s string) *string { return &s }
