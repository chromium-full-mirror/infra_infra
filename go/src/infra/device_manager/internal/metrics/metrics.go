// Copyright 2024 The Chromium Authors
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
	UFSActionsPerJob = metric.NewNonCumulativeDistribution(
		"device_manager/ufs_updates/actions_per_job",
		"Device actions per UFS update sent to Scheduke from DM",
		&types.MetricMetadata{Units: "actions per job"},
		distribution.FixedWidthBucketer(10, 10000),
		field.String("project"),
		field.String("action"),
	)
	UFSJobRuntime = metric.NewNonCumulativeDistribution(
		"device_manager/ufs_updates/job_runtime",
		"Runtime for each UFS update sent to Scheduke from DM",
		&types.MetricMetadata{Units: types.Seconds},
		distribution.FixedWidthBucketer(10, 360),
		field.String("project"),
	)
	UFSJobErrorCount = metric.NewNonCumulativeDistribution(
		"device_manager/ufs_updates/job_error_count",
		"Error count for each UFS update sent to Scheduke from DM",
		&types.MetricMetadata{Units: "errors per batch"},
		distribution.FixedWidthBucketer(10, 10000),
		field.String("project"),
		field.String("error"),
	)
)
