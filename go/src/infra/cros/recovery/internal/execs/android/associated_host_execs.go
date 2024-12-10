// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/android/adb"
	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

const (
	flagsDir   = "/var/lib/servod/"
	inUseFlag  = flagsDir + "adb_%s_in_use"
	rebootFlag = flagsDir + "%s_reboot"
)

func newPinger(info *execs.ExecInfo) components.Pinger {
	hostName := info.GetAndroid().GetAssociatedHostname()
	return info.NewPinger(hostName)
}

func newRunner(info *execs.ExecInfo) components.Runner {
	hostName := info.GetAndroid().GetAssociatedHostname()
	return info.NewRunner(hostName)
}

// pingAssociatedHostExec verifies that associated host of the DUT is pingable.
func pingAssociatedHostExec(ctx context.Context, info *execs.ExecInfo) error {
	return cros.WaitUntilPingable(ctx, info.GetExecTimeout(), cros.PingRetryInterval, 2, newPinger(info))
}

// sshAssociatedHostExec verifies ssh access to the associated host of the DUT.
func sshAssociatedHostExec(ctx context.Context, info *execs.ExecInfo) error {
	return cros.WaitUntilSSHable(ctx, info.GetExecTimeout(), cros.SSHRetryInterval, newRunner(info))
}

// isAssociatedHostLabstationExec verifies that adb is installed at the DUT associated host.
func isAssociatedHostLabstationExec(ctx context.Context, info *execs.ExecInfo) error {
	board, err := cros.ReleaseBoard(ctx, newRunner(info), info.NewLogger())
	if err != nil {
		return errors.Annotate(err, "associated host is labstation").Err()
	}
	if !strings.Contains(board, "labstation") {
		return errors.Reason("associated host is not labstation").Err()
	}
	return nil
}

// hasADBVendorKeyExec verifies that adb vendor key is provisioned at the DUT associated host.
func hasADBVendorKeyExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	if !actionArgs.Has("adb_vendor_key") {
		return errors.Reason("invalid number of arguments: adb vendor key is required").Err()
	}
	vendorKey := actionArgs.AsString(ctx, "adb_vendor_key", "")
	return adb.CheckADBVendorKey(ctx, newRunner(info), info.NewLogger(), vendorKey)
}

// hasADBInstalledExec verifies that adb is installed at the DUT associated host.
func hasADBInstalledExec(ctx context.Context, info *execs.ExecInfo) error {
	path, err := adb.ADBInstallPath(ctx, newRunner(info), info.NewLogger())
	if err != nil {
		return err
	}
	log.Debugf(ctx, "Adb path at the associated host: %q", path)
	if path == "" {
		return errors.Reason("adb is not installed at the dut associated host").Err()
	}
	return nil
}

// startADBServerExec ensures that adb server is running on the DUT associated host.
func startADBServerExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	if !actionArgs.Has("adb_vendor_key") {
		return errors.Reason("invalid number of arguments: adb vendor key is required").Err()
	}
	vendorKey := actionArgs.AsString(ctx, "adb_vendor_key", "")
	return adb.StartADBServer(ctx, newRunner(info), info.NewLogger(), vendorKey)
}

// killADBServerExec kills adb server if it is running on the DUT associated host.
func killADBServerExec(ctx context.Context, info *execs.ExecInfo) error {
	return adb.KillADBServer(ctx, newRunner(info), info.NewLogger())
}

// isFileSystemWritable checks whether the stateful file systems are writable.
func isFileSystemWritableExec(ctx context.Context, info *execs.ExecInfo) error {
	// N.B. Order matters here:  Encrypted stateful is loop-mounted from a file in unencrypted stateful,
	// so we don't test for errors in encrypted stateful if unencrypted fails.
	testDirs := []string{"/mnt/stateful_partition", "/var/tmp"}
	return cros.IsFileSystemWritable(ctx, newRunner(info), testDirs)
}

// createInUseFlagExec creates in-use flag file.
func createInUseFlagExec(ctx context.Context, info *execs.ExecInfo) error {
	const createInUseFlagCmd = "touch " + inUseFlag
	run := newRunner(info)
	serialNumber := info.GetAndroid().GetSerialNumber()
	if _, err := run(ctx, time.Minute, fmt.Sprintf(createInUseFlagCmd, serialNumber)); err != nil {
		log.Debugf(ctx, "Create in-use flag file: %s", err)
	}
	// Ignore errors.
	return nil
}

// hasNoOtherInUseFlagsExec fails if associated host has other adb in-use flag files.
func hasNoOtherInUseFlagsExec(ctx context.Context, info *execs.ExecInfo) error {
	const (
		findOtherInUseFlagsCmd  = "find " + flagsDir + "adb_*_in_use -type f ! -name 'adb_%s_in_use' -mmin -%d"
		inUseFlagExpirationMins = 10
	)
	run := newRunner(info)
	serialNumber := info.GetAndroid().GetSerialNumber()
	output, _ := run(ctx, time.Minute, fmt.Sprintf(findOtherInUseFlagsCmd, serialNumber, inUseFlagExpirationMins))
	if output == "" {
		log.Debugf(ctx, "Does not have other adb in-use flags.")
		return nil
	}
	return errors.Reason("has other adb in-use flags\n%s", output).Err()
}

// removeInUseFlagExec removes in-use flag file.
func removeInUseFlagExec(ctx context.Context, info *execs.ExecInfo) error {
	const removeInUseFlagCmd = "rm -f " + inUseFlag
	run := newRunner(info)
	serialNumber := info.GetAndroid().GetSerialNumber()
	if _, err := run(ctx, time.Minute, fmt.Sprintf(removeInUseFlagCmd, serialNumber)); err != nil {
		log.Debugf(ctx, "Remove in-use file flag: %s", err)
	}
	// Ignore errors.
	return nil
}

// scheduleRebootExec creates a file to request reboot of the DUT associated host.
func scheduleRebootExec(ctx context.Context, info *execs.ExecInfo) error {
	const createRebootFlagCmd = "touch " + rebootFlag
	run := newRunner(info)
	serialNumber := info.GetAndroid().GetSerialNumber()
	if _, err := run(ctx, time.Minute, fmt.Sprintf(createRebootFlagCmd, serialNumber)); err != nil {
		log.Debugf(ctx, "Schedule a reboot request: %s", err)
	}
	// Ignore errors.
	return nil
}

// restoreADBVendorKeyExec create adb vendor private key used for testing on the associated host.
func restoreADBVendorKeyExec(ctx context.Context, info *execs.ExecInfo) error {
	run := newRunner(info)
	actionArgs := info.GetActionArgs(ctx)
	vendorKeyFile := actionArgs.AsString(ctx, "vendor_key_file", "")
	if vendorKeyFile == "" {
		return errors.Reason("restore adb vendor key: vendor_key_file is not provided in action args").Err()
	}
	vendorKeyContent := actionArgs.AsString(ctx, "vendor_key_content", "")
	if vendorKeyContent == "" {
		return errors.Reason("restore adb vendor key: vendor_key_content is not provided in action args").Err()
	}
	if _, err := run(ctx, time.Minute, fmt.Sprintf("mkdir -p %s", filepath.Dir(vendorKeyFile))); err != nil {
		return errors.Annotate(err, "restore adb vendor key").Err()
	}
	if _, err := run(ctx, time.Minute, fmt.Sprintf("echo \"%s\" > %s", vendorKeyContent, vendorKeyFile)); err != nil {
		return errors.Annotate(err, "restore adb vendor key").Err()
	}
	if _, err := run(ctx, time.Minute, fmt.Sprintf("chmod 600 %s", vendorKeyFile)); err != nil {
		return errors.Annotate(err, "restore adb vendor key").Err()
	}
	return nil
}

func init() {
	execs.Register("android_associated_host_ping", pingAssociatedHostExec)
	execs.Register("android_associated_host_ssh", sshAssociatedHostExec)
	execs.Register("android_associated_host_is_labstation", isAssociatedHostLabstationExec)
	execs.Register("android_associated_host_has_vendor_key", hasADBVendorKeyExec)
	execs.Register("android_associated_host_has_adb", hasADBInstalledExec)
	execs.Register("android_associated_host_start_adb", startADBServerExec)
	execs.Register("android_associated_host_stop_adb", killADBServerExec)
	execs.Register("android_associated_host_fs_is_writable", isFileSystemWritableExec)
	execs.Register("android_associated_host_lock", createInUseFlagExec)
	execs.Register("android_associated_host_has_no_other_locks", hasNoOtherInUseFlagsExec)
	execs.Register("android_associated_host_unlock", removeInUseFlagExec)
	execs.Register("android_associated_host_schedule_reboot", scheduleRebootExec)
	execs.Register("android_associated_host_restore_vendor_key", restoreADBVendorKeyExec)
}
