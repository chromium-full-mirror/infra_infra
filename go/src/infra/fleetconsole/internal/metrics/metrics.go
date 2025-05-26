// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package metrics defines custom tsmon metrics exported by Device Manager.
package metrics

import (
	"go.chromium.org/luci/common/tsmon/distribution"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/types"
)

var (
	UpdateCacheDevicesPerAction = metric.NewNonCumulativeDistribution(
		"fleet_console/cache_update/devices_per_action",
		"Counts of devices affected by cache update actions",
		&types.MetricMetadata{Units: "devices per action"},
		distribution.FixedWidthBucketer(10, 10000),
		field.String("project"),
		field.String("action"),
	)
)
