// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/tpm"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/vpd"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
)

// isEnrollmentInCleanState confirms that the device's enrollment state is clean
func isEnrollmentInCleanStateExec(ctx context.Context, info *execs.ExecInfo) error {
	ha := info.NewHostAccess(info.GetDut().Name)
	isClean, err := vpd.IsEnrollmentInClean(ctx, ha, time.Minute)
	if err != nil {
		// In any case it returns a non zero value, it means we can't verify enrollment state, but we cannot say the device is enrolled
		// Only trigger the enrollment in clean state when we can confirm the device is enrolled.
		log.Errorf(ctx, "Unexpected error occurred during verify enrollment state in VPD cache, skipping verify process.")
		return nil
	}
	if !isClean {
		return errors.Reason("enrollment in clean state: failed, The device is enrolled, it may interfere with some tests").Err()
	}
	return nil
}

// enrollmentCleanupExec cleans up the enrollment state on the
// ChromeOS device.
func enrollmentCleanupExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	ha := info.NewHostAccess(info.GetDut().Name)
	// 1. Reset VPD enrollment state
	log.Debugf(ctx, "First: Try to reset VPD enrollment state!")
	repairTimeout := argsMap.AsDuration(ctx, "repair_timeout", 120, time.Second)
	log.Debugf(ctx, "enrollment cleanup: using repair timeout :%s", repairTimeout)
	if _, err := ha.Run(ctx, repairTimeout, "/usr/sbin/update_rw_vpd check_enrollment", "0"); err != nil {
		log.Debugf(ctx, "Fail to reset VPD enrollment state: %w", err)
	}
	// 2. clear tpm owner state
	log.Debugf(ctx, "Second: Try to clean TPM owner state!")
	clearTpmOwnerTimeout := argsMap.AsDuration(ctx, "clear_tpm_owner_timeout", 60, time.Second)
	log.Debugf(ctx, "enrollment cleanup: using clear tpm owner timeout :%s", clearTpmOwnerTimeout)
	if _, err := ha.Run(ctx, clearTpmOwnerTimeout, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		log.Debugf(ctx, "enrollment cleanup: unable to clear TPM.")
		return errors.Annotate(err, "enrollment cleanup").Err()
	}
	filesToRemove := []string{
		"/home/chronos/.oobe_completed",
		"/home/chronos/Local\\ State",
		"/var/cache/shill/default.profile",
	}
	dirsToRemove := []string{
		"/home/.shadow/*",
		filepath.Join("/var/cache/shill/default.profile", "*"),
		"/var/lib/whitelist/*", // nocheck
		"/var/cache/app_pack",
		"/var/lib/tpm",
	}
	// We do not care about any errors that might be returned by the
	// following two command executions.
	fileDeletionTimeout := argsMap.AsDuration(ctx, "file_deletion_timeout", 120, time.Second)
	if _, err := ha.Run(ctx, fileDeletionTimeout, "sudo", "rm", "-rf", strings.Join(filesToRemove, " "), strings.Join(dirsToRemove, " ")); err != nil {
		log.Debugf(ctx, "Fail to remove dirs and files from the host: %s", err)
	}
	if _, err := ha.Run(ctx, fileDeletionTimeout, "sync"); err != nil {
		log.Debugf(ctx, "Fail to run FS sync: %s", err)
	}
	rebootTimeoutToDown := argsMap.AsDuration(ctx, "reboot_timeout_down", 120, time.Second)
	rebootTimeoutToUp := argsMap.AsDuration(ctx, "reboot_timeout_up", 240, time.Second)
	if err := cros.RebootWithCheck(ctx, ha, rebootTimeoutToDown, rebootTimeoutToUp); err != nil {
		return errors.Annotate(err, "enrollment cleanup").Err()
	}
	// Finally, we will read the TPM status, and will check whether it
	// has been cleared or not.
	tpmTimeout := argsMap.AsDuration(ctx, "tpm_timeout", 150, time.Second)
	log.Debugf(ctx, "enrollment cleanup: using tpm timeout :%s", tpmTimeout)
	if err := retry.WithTimeout(ctx, time.Second, tpmTimeout, func() error {
		isClean, err := vpd.IsEnrollmentInClean(ctx, ha, time.Minute)
		if err != nil {
			return err
		}
		if isClean {
			// We can stop as we are good.
			return nil
		}
		return errors.Reason("state is not clean").Err()
	}, "wait to read tpm status"); err != nil {
		return errors.Annotate(err, "enrollment cleanup").Err()
	}
	if argsMap.AsBool(ctx, "fwmp_cleanup_enabled", false) {
		skipError := argsMap.AsBool(ctx, "fwmp_skip_error", false)
		ownerDetectTimeout := argsMap.AsDuration(ctx, "owner_detect_timeout", 10, time.Second)
		skipRebootFWMP := argsMap.AsBool(ctx, "fwmp_skip_reboot", true)
		fwmpCleaner := tpm.NewFWMPCleaner(ctx, ha, 10*time.Second)
		if isCLean, err := fwmpCleaner.IsClean(ctx, ha); err != nil {
			if skipError {
				log.Debugf(ctx, "enrollment cleanup: fail to read FWMP flags: %s", err)
			} else {
				return errors.Annotate(err, "enrollment cleanup: fwmp").Err()
			}
		} else if isCLean {
			log.Debugf(ctx, "FWMP flag is clean!")
		} else if err := fwmpCleaner.Clean(ctx, ha, skipRebootFWMP, ownerDetectTimeout); err != nil {
			if skipError {
				log.Debugf(ctx, "enrollment cleanup: fail to clean up FWMP flags: %s", err)
			} else {
				return errors.Annotate(err, "enrollment cleanup: fwmp").Err()
			}
		}
	}
	return nil
}

func init() {
	execs.Register("cros_is_enrollment_in_clean_state", isEnrollmentInCleanStateExec)
	execs.Register("cros_enrollment_cleanup", enrollmentCleanupExec)
}
