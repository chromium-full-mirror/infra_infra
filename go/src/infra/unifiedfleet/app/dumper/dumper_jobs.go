// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/server"

	"go.chromium.org/infra/unifiedfleet/app/cron"
)

// Jobs is a list of all the cron jobs that are currently available for running
var Jobs = []*cron.CronTab{
	{
		// Dump configs, registrations, inventory and states to BQ
		Name:     "ufs.dumper.daily",
		Time:     20 * time.Minute,
		TrigType: cron.DAILY,
		Job:      dumpDaily,
	},
	{
		// Dump configs, registrations, inventory and states to BQ
		Name:     "ufs.dumper.hourly",
		Time:     30 * time.Minute,
		TrigType: cron.HOURLY,
		Job:      dumpHourly,
	},
	{
		// Dump change events to BQ
		Name:     "ufs.change_event.BqDump",
		Time:     10 * time.Minute,
		TrigType: cron.EVERY,
		Job:      dumpChangeEvent,
	},
	{
		// Dump snapshots to BQ
		Name:     "ufs.snapshot_msg.BqDump",
		Time:     10 * time.Minute,
		TrigType: cron.EVERY,
		Job:      dumpChangeSnapshots,
	},
	{
		// Dump network configs to BQ
		Name:     "ufs.cros_network.dump",
		Time:     60 * time.Minute,
		TrigType: cron.EVERY,
		Job:      dumpCrosNetwork,
	},
	{
		// Sync asset info from HaRT
		Name:     "ufs.sync_devices.sync",
		TrigType: cron.HOURLY,
		Job:      SyncAssetInfoFromHaRT,
	},
	{
		// Push changes to dron queen
		Name:     "ufs.push_to_drone_queen",
		Time:     10 * time.Minute,
		TrigType: cron.EVERY,
		Job:      pushToDroneQueen,
	},
	{
		// Report UFS metrics
		Name:     "ufs.report_inventory",
		Time:     5 * time.Minute,
		TrigType: cron.EVERY,
		Job:      reportUFSInventoryCronHandler,
	},
	{
		// Report Swarming bots
		Name:     "ufs.report_bots",
		Time:     5 * time.Minute,
		TrigType: cron.EVERY,
		Job:      reportSwarmingBotsCronHandler,
	},
	{
		// Sync Goldeneye Data
		Name:     "ufs.sync_goldeneye_devices.sync",
		Time:     12 * time.Hour,
		TrigType: cron.EVERY,
		Job:      getGoldenEyeData,
	},
	{
		// Compare differences between Swarming label generators
		Name:     "ufs.swarming_labels_diff",
		Time:     5 * time.Minute,
		TrigType: cron.DAILY,
		Job:      swarmingLabelsDiffHandler,
	},
	{
		// Sync ENC bot and security configs
		Name:     "ufs.sync_bot_config.sync",
		Time:     10 * time.Minute,
		TrigType: cron.EVERY,
		Job:      getBotConfigs,
	},
	{
		// Sync maintenance configs
		Name:     "ufs.sync_maintenance_config.sync",
		Time:     10 * time.Minute,
		TrigType: cron.EVERY,
		Job:      getMaintenanceConfigs,
	},
	{
		Name:     "ufs.device_config.sync",
		Time:     1 * time.Hour,
		TrigType: cron.EVERY,
		Job:      syncDeviceConfigs,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.indexer.asset",
		Time:     200 * 365 * 24 * time.Hour, // when I'm gone carry on, don't mourn
		TrigType: cron.EVERY,
		Job:      IndexAssets,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.indexer.machine",
		Time:     200 * 365 * 24 * time.Hour,
		TrigType: cron.EVERY,
		Job:      IndexMachines,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.indexer.rack",
		Time:     200 * 365 * 24 * time.Hour,
		TrigType: cron.EVERY,
		Job:      indexRacks,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.indexer.machinelse",
		Time:     200 * 365 * 24 * time.Hour, // Wake me up when September ends
		TrigType: cron.EVERY,
		Job:      indexMachineLSEs,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.indexer.dutstate",
		Time:     200 * 365 * 24 * time.Hour,
		TrigType: cron.EVERY,
		Job:      indexDutStates,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.backfill.devicelabels_machinelse",
		Time:     200 * 365 * 24 * time.Hour,
		TrigType: cron.EVERY,
		Job:      backfillDeviceLabelsMachineLSEs,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.backfill.devicelabels_vm",
		Time:     200 * 365 * 24 * time.Hour,
		TrigType: cron.EVERY,
		Job:      backfillDeviceLabelsVMs,
	},
	{
		// This job is not meant to be run by the cron. This will
		// be triggered by shivas at whatever time oncall deems
		// appropriate. There is a potential for this to block
		// other jobs as it updates ~500 rows every time. Only run
		// during low-traffic non-critical times. The long wait time
		// ensures that this will not be run under normal circumstances
		// (unless we manage to not update/reboot UFS for 200 years)
		Name:     "ufs.backfill.devicelabels_schedulingunit",
		Time:     200 * 365 * 24 * time.Hour,
		TrigType: cron.EVERY,
		Job:      backfillDeviceLabelsSchedulingUnits,
	},
	// TODO(b/378926955): Enable dumper job automation when prod data ready
	// {
	// 	// Sync Nlyte device information
	// 	Name:     "ufs.nlyte_devices.sync",
	// 	Time:     1 * time.Hour,
	// 	TrigType: cron.EVERY,
	// 	Job:      fetchNlyteBigQueryData,
	// },
}

// InitServer initializes a cron server.
func InitServer(srv *server.Server) error {
	// Perform any initialization for the cron jobs
	if err := initializeUFSInventoryTsmonState(srv); err != nil {
		return errors.Annotate(err, "Could not initalize tsmon state for reporting UFS inventory").Err()
	}

	// Schedule the cron jobs
	for _, job := range Jobs {
		// make a copy of the job to avoid race condition.
		t := job
		// Start all the cron jobs in background.
		srv.RunInBackground(job.Name, func(ctx context.Context) {
			cron.Run(ctx, t)
		})
	}
	return nil
}

// TriggerJob triggers a job by name. Returns error if the job is not found.
func TriggerJob(name string) error {
	for _, job := range Jobs {
		if job.Name == name {
			return cron.Trigger(job)
		}
	}
	return status.Errorf(codes.NotFound, "Invalid cron job %s. Not found", name)
}
