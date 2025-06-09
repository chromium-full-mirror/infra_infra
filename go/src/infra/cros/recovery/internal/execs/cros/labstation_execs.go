// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

const (
	// Threshold of messages log size we can keep. It should use expression(bcwkMG) that supported by `find` cli.
	currentMessagesLogSizeThreshold = "300M"
	oldMessagesLogSizeThreshold     = "50M"
	// A specific firmware for servo dock, applies to servo_v4.1 only.
	genesysLogicFirmwarePath = "/usr/share/fwupd/remotes.d/vendor/firmware/be2c9146ff4cfac5d647376c39ce0b78151e9f1a785a287e93ac3968aff2ed50-GenesysLogic_GL3590_64.17.cab"
)

// cleanTmpOwnerRequestExec cleans tpm owner requests.
func cleanTmpOwnerRequestExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	_, err := run(ctx, time.Minute, "crossystem clear_tpm_owner_request=1")
	return errors.Annotate(err, "clear tpm owner request").Err()
}

// validateUptime validate that host is up for more than a threshold
// number of hours.
func validateUptime(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	maxDuration := argsMap.AsDuration(ctx, "max_duration", 0, time.Hour)
	minDuration := argsMap.AsDuration(ctx, "min_duration", 0, time.Hour)
	if maxDuration == 0 && minDuration == 0 {
		return errors.Reason("validate uptime: neither min nor max duration is specified").Err()
	}
	dur, err := cros.Uptime(ctx, info.DefaultRunner())
	if err != nil {
		return errors.Annotate(err, "validate uptime").Err()
	}
	if maxDuration != 0 && *dur >= maxDuration {
		return errors.Reason("validate uptime: uptime %s equals or exceeds the expected maximum threshold %s", dur, maxDuration).Err()
	}
	if minDuration != 0 && *dur < minDuration {
		return errors.Reason("validate uptime: uptime %s is less than the expected minimum threshold %s", dur, minDuration).Err()
	}
	log.Debugf(ctx, "Validate Uptime: current uptime: %s, min threshold: %s, max threshold: %s: all good.", dur, minDuration, maxDuration)
	return nil
}

const (
	// The flag-file indicates the host should not to be rebooted.
	noRebootFlagFile = "/tmp/no_reboot"
)

// allowedRebootExec checks if DUT is allowed to reboot.
// If system has /tmp/no_reboot file then reboot is not allowed.
func allowedRebootExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	cmd := fmt.Sprintf("test %s", noRebootFlagFile)
	_, err := run(ctx, time.Minute, cmd)
	if err != nil {
		return errors.Annotate(err, "has no-reboot request").Err()
	}
	log.Debugf(ctx, "No-reboot request file found.")
	return nil
}

// filesystemIoNotBlockedExec check if the labstation's filesystem IO is blocked.
func filesystemIoNotBlockedExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	cmd := "ps axl | awk '$10 ~ /D/'"
	output, err := run(ctx, info.GetExecTimeout(), cmd)
	if err != nil {
		return errors.Annotate(err, "filesystem is not blocked").Err()
	}
	// Good labstation may occasionally have an process in uninterruptible
	// sleep state transiently, so we look for these who have 2+ processes
	// stuck in such a state.
	if len(output) > 1 {
		return errors.Reason("filesystem is not blocked: more than one processes in uninterruptible sleep state, I/O is likely blocked.").Err()
	}
	return nil
}

// logCleanupExec rotate and cleanup stale log files on labstation.
func logCleanupExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()

	// Clean up stale(> 3 days) servod logs.
	run(ctx, info.GetExecTimeout(), "find /var/log/servo* -type f -mtime +3 | xargs rm")

	// Clean up stale logs that preserved during provision.
	run(ctx, info.GetExecTimeout(), "rm -rf /mnt/stateful_partition/unencrypted/preserve/log")

	// First we want to check if the current messages log larger than the threshold, and if it is
	// we need rotate logs before we can safely remove it as other process may still writing logs into it.
	checkCurrentCmd := fmt.Sprintf("find /var/log/messages -size +%s", currentMessagesLogSizeThreshold)
	if out, _ := run(ctx, info.GetExecTimeout(), checkCurrentCmd); out != "" {
		log.Debugf(ctx, "Log cleanup: current messages log larger than %s, will rotate logs.", currentMessagesLogSizeThreshold)
		if _, err := run(ctx, info.GetExecTimeout(), "/usr/sbin/chromeos-cleanup-logs"); err != nil {
			log.Debugf(ctx, "Log cleanup: failed to execute chromeos-cleanup-logs, %v", err)
		}
	}

	// Checking if there are any old logs that larger than the threshold, and if true remove all old logs.
	checkOldCmd := fmt.Sprintf("find /var/log/messages.* -size +%s", oldMessagesLogSizeThreshold)
	if out, _ := run(ctx, info.GetExecTimeout(), checkOldCmd); out != "" {
		log.Debugf(ctx, "Log cleanup: detected old messages log that larger than %s", oldMessagesLogSizeThreshold)
		if _, err := run(ctx, info.GetExecTimeout(), "rm /var/log/messages.*"); err != nil {
			return errors.Reason("log cleanup: failed to remove old messages log.").Err()
		}
		log.Debugf(ctx, "Log cleanup: successfully removed old messages log.")
	}

	// Remove anything in /var/log if large than 500M.
	run(ctx, info.GetExecTimeout(), "find /var/log/ -type f -size +500M | xargs rm")

	return nil
}

// updateGenesysLogicFirmwareForServos updates a specific version of GenesysLogic firmware for all
// servo_v4p1 on the labstation. The update is a no-op for servos that already updated to the given
// firmware, and servos that doesn't have the applicable chip(e.g. servo_v4).
func updateGenesysLogicFirmwareForServos(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	if _, err := run(ctx, info.GetExecTimeout(), fmt.Sprintf("fwupdtool install --filter=\"updatable\" %s", genesysLogicFirmwarePath)); err != nil {
		errorCode, ok := components.ErrCodeTag.Value(err)
		if !ok {
			return errors.Annotate(err, "update GenesysLogic firmware for servos: cannot find error code").Err()
		}
		// The tool would complete with exit code 2 when all servos already updated or no applicate servos.
		if errorCode != 2 {
			return errors.Annotate(err, "update GenesysLogic firmware for servos").Err()
		}
	}
	return nil
}

// checkUsedInodePercentageLowerThanThreshold compare used Inode percentage of a specific path with a specific threshold.
// It only fails if we can get a reading from system and the reading is greater than the threshold, all error that
// prevent us from get a valid reading will be warning only.
func checkUsedInodePercentageLowerThanThreshold(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	argsMap := info.GetActionArgs(ctx)
	targetPath := argsMap.AsString(ctx, "targetPath", "/mnt/stateful_partition")
	if _, err := run(ctx, 20*time.Second, fmt.Sprintf("test -e %s", targetPath)); err != nil {
		log.Warningf(ctx, "(Non-critical) path: %s does not exists", targetPath)
		return nil
	}
	output, err := run(ctx, 20*time.Second, fmt.Sprintf("df -Pi %s | tail -1", targetPath))
	if err != nil {
		log.Warningf(ctx, "(Non-critical) failed to get valid reading from df, %s", err.Error())
		return nil
	}
	parts := strings.Fields(output)
	// An example of expected output would includes value of Filesystem, Inodes, IUsed, IFree, IUse%, Mounted on.
	if len(parts) != 6 {
		log.Warningf(ctx, "(Non-critical) failed to parse reading from df")
		return nil
	}
	percentageStr := strings.TrimSuffix(parts[4], "%")
	usePercentage, err := strconv.Atoi(percentageStr)
	if err != nil {
		log.Warningf(ctx, "(Non-critical) failed to convert IUse to an integer, %s", err.Error())
	}
	threshold := argsMap.AsInt(ctx, "threshold", 50)
	if usePercentage > threshold {
		metrics.NewInt64Observation("usedInodePercentage", int64(usePercentage))
		return errors.Reason("ensure used inode percentage: IUse: %d is greater than threshold: %d", usePercentage, threshold).Err()
	}
	return nil
}

func init() {
	execs.Register("cros_clean_tmp_owner_request", cleanTmpOwnerRequestExec)
	execs.Register("cros_validate_uptime", validateUptime)
	execs.Register("cros_allowed_reboot", allowedRebootExec)
	execs.Register("cros_filesystem_io_not_blocked", filesystemIoNotBlockedExec)
	execs.Register("cros_log_clean_up", logCleanupExec)
	execs.Register("cros_update_genesys_logic_firmware", updateGenesysLogicFirmwareForServos)
	execs.Register("cros_check_used_inode_percentage_lower_than_threshold", checkUsedInodePercentageLowerThanThreshold)
}
