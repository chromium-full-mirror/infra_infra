// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
)

var (
	dumpToBQTick = metric.NewCounter(
		"chromeos/ufs/dumper/dump_to_bq",
		"dumpToBQ attempt",
		nil,
		field.Bool("success"), // If the attempt succeed
	)
	dumpToBQDailyTick = metric.NewCounter(
		"chromeos/ufs/dumper/dump_to_bq_daily",
		"dumpToBQ daily attempt",
		nil,
		field.Bool("success"),
		field.String("namespace"),
	)
	dumpToBQHourlyTick = metric.NewCounter(
		"chromeos/ufs/dumper/dump_to_bq_hourly",
		"dumpToBQ hourly attempt",
		nil,
		field.Bool("success"),
		field.String("namespace"),
	)
	dumpChangeEventTick = metric.NewCounter(
		"chromeos/ufs/dumper/dump_change_event",
		"dumpChangeEvent attempt",
		nil,
		field.Bool("success"),
	)
	dumpChangeSnapshotTick = metric.NewCounter(
		"chromeos/ufs/dumper/dump_change_snapshot",
		"dumpChangeSnapshot attempt",
		nil,
		field.Bool("success"),
	)
	dumpCrosNetworkTick = metric.NewCounter(
		"chromeos/ufs/dumper/import_cros_network",
		"import cros network attempt",
		nil,
		field.Bool("success"),
	)
	dumpPushToDroneQueenTick = metric.NewCounter(
		"chromeos/ufs/dumper/push_to_drone_queen",
		"push to drone queen attempt",
		nil,
		field.Bool("success"),
	)
	getGoldenEyeDataTick = metric.NewCounter(
		"chromeos/ufs/dumper/sync_goldeneye_data",
		"getGoldenEyeData attempt every 12 hours",
		nil,
		field.Bool("success"),
	)
	syncDeviceConfigsTick = metric.NewCounter(
		"chromeos/ufs/dumper/sync_device_configs",
		"sync device configs hourly attempt",
		nil,
		field.Bool("success"),
	)
	syncSecurityConfigsTick = metric.NewCounter(
		"chromeos/ufs/dumper/sync_security_configs",
		"sync security configs attempt",
		nil,
		field.Bool("success"),
	)
	fetchNlyteBigQueryDataTick = metric.NewCounter(
		"chromeos/ufs/dumper/sync_nlyte_bigquery_data",
		"getNlyteBigQueryData hourly attempt",
		nil,
		field.Bool("success"),
	)
)
