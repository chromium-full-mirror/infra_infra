// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utilization provides functions to report DUT utilization metrics.
package utilization

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/cros/dutstate"
	invV1 "go.chromium.org/infra/libs/skylab/inventory"
)

var dutmonMetric = metric.NewInt(
	"chromeos/skylab/dut_mon/swarming_dut_count",
	"The number of DUTs in a given bucket and status",
	nil,
	field.String("board"),
	field.String("model"),
	field.String("pool"),
	field.String("zone"),
	field.String("status"),
	field.Bool("is_locked"),
)

// ReportMetrics reports DUT utilization metrics akin to dutmon in Autotest
//
// The reported fields closely match those reported by dutmon, but the metrics
// path is different.
func ReportMetrics(ctx context.Context, bis []*swarmingv2.BotInfo) {
	c := make(counter)
	for _, bi := range bis {
		b := getBucketForBotInfo(bi)
		s := getStatusForBotInfo(bi)
		c.Increment(b, s)
	}
	c.Report(ctx)
}

// bucket contains static DUT dimensions.
//
// These dimensions do not change often. If all DUTs with a given set of
// dimensions are removed, the related metric is not automatically reset. The
// metric will get reset eventually.
type bucket struct {
	board string
	model string
	pool  string
	zone  string
}

func (b bucket) String() string {
	return fmt.Sprintf("board: %s, model: %s, pool: %s, zone: %s", b.board, b.model, b.pool, b.zone)
}

// status is a dynamic DUT dimension.
//
// This dimension changes often. If no DUTs have a particular status value,
// the corresponding metric is immediately reset.
type status string

var allStatuses = []status{
	"[None]",
	"Ready",
	"RepairFailed",
	"NeedsRepair",
	"NeedsReset",
	"Running",
	"NeedsDeploy",
	"Deploying",
	"Reserved",
	"ManualRepair",
	"NeedsManualRepair",
	"NeedsReplacement",
	"Unknown",
}

// counter collects number of DUTs per bucket and status.
type counter map[bucket]map[status]int

func (c counter) Increment(b bucket, s status) {
	sc := c[b]
	if sc == nil {
		sc = make(map[status]int)
		c[b] = sc
	}
	sc[s]++
}

func (c counter) Report(ctx context.Context) {
	for b, counts := range c {
		for _, s := range allStatuses {
			// TODO(crbug/929872) Report locked status once DUT leasing is
			// implemented in Skylab.
			dutmonMetric.Set(ctx, int64(counts[s]), b.board, b.model, b.pool, b.zone, string(s), false)
		}
	}
}

func getBucketForBotInfo(bi *swarmingv2.BotInfo) bucket {
	b := bucket{
		board: "[None]",
		model: "[None]",
		pool:  "[None]",
		zone:  "[None]",
	}
	for _, d := range bi.Dimensions {
		switch d.Key {
		case "label-board":
			b.board = summarizeValues(d.Value)
		case "label-model":
			b.model = summarizeValues(d.Value)
		case "label-pool":
			b.pool = getReportPool(d.Value)
		case "label-zone":
			b.zone = summarizeValues(d.Value)
		default:
			// Ignore other dimensions.
		}
	}
	return b
}

func getStatusForBotInfo(bi *swarmingv2.BotInfo) status {
	dutState := ""
	for _, d := range bi.Dimensions {
		switch d.Key {
		case "dut_state":
			dutState = summarizeValues(d.Value)
		default:
			// Ignore other dimensions.
		}
	}

	// Order matters: a bot may be dead and still have a task associated with it.
	if !isBotHealthy(bi) {
		return "[None]"
	}

	botBusy := bi.TaskId != ""

	switch dutState {
	case dutstate.Ready.String():
		if botBusy {
			return "Running"
		}
		return "Ready"
	case "running":
		return "Running"
	case dutstate.NeedsReset.String():
		// We count time spent waiting for a reset task to be assigned as time
		// spent Resetting.
		return "NeedsReset"
	case dutstate.NeedsRepair.String():
		// We count time spent waiting for a repair task to be assigned as time
		// spent Repairing.
		return "NeedsRepair"
	case dutstate.RepairFailed.String():
		return "RepairFailed"
	case dutstate.NeedsDeploy.String():
		return "NeedsDeploy"
	case dutstate.Deploying.String():
		return "Deploying"
	case dutstate.Reserved.String():
		return "Reserved"
	case dutstate.ManualRepair.String():
		return "ManualRepair"
	case dutstate.NeedsManualRepair.String():
		return "NeedsManualRepair"
	case dutstate.NeedsReplacement.String():
		return "NeedsReplacement"
	case dutstate.Unknown.String():
		return "Unknown"

	default:
		return "[None]"
		// We should never see this state
	}
}

func isBotHealthy(bi *swarmingv2.BotInfo) bool {
	return !bi.Deleted && !bi.IsDead && !bi.Quarantined
}

func summarizeValues(vs []string) string {
	switch len(vs) {
	case 0:
		return "[None]"
	case 1:
		return vs[0]
	default:
		return "[Multiple]"
	}
}

func isManagedPool(p string) bool {
	_, ok := invV1.SchedulableLabels_DUTPool_value[p]
	return ok
}

func getReportPool(pools []string) string {
	p := summarizeValues(pools)
	if isManagedPool(p) {
		return fmt.Sprintf("managed:%s", p)
	}
	return p
}
