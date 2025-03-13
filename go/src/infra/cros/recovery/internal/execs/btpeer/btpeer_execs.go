// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/recovery/internal/components/btpeer"
	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// setStateBrokenExec sets state as BROKEN.
func setStateBrokenExec(ctx context.Context, info *execs.ExecInfo) error {
	if h, err := activeHost(info.GetActiveResource(), info.GetChromeos()); err != nil {
		return errors.Annotate(err, "set state broken").Err()
	} else {
		h.State = tlw.BluetoothPeer_BROKEN
	}
	return nil
}

// setStateWorkingExec sets state as WORKING.
func setStateWorkingExec(ctx context.Context, info *execs.ExecInfo) error {
	if h, err := activeHost(info.GetActiveResource(), info.GetChromeos()); err != nil {
		return errors.Annotate(err, "set state working").Err()
	} else {
		h.State = tlw.BluetoothPeer_WORKING
	}
	return nil
}

// rebootExec reboots the device over ssh and waits for the device to become
// ssh-able again.
func rebootExec(ctx context.Context, info *execs.ExecInfo) error {
	sshRunner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	if err := ssh.Reboot(ctx, sshRunner, 10*time.Second, 10*time.Second, 3*time.Minute); err != nil {
		return errors.Annotate(err, "failed to reboot btpeer").Err()
	}
	return nil
}

// assertUptimeIsLessThanDurationExec checks the uptime of the device and
// fails if the uptime is not less than the duration in minutes provided in
// the "duration_min" action arg.
func assertUptimeIsLessThanDurationExec(ctx context.Context, info *execs.ExecInfo) error {
	// Parse duration arg.
	actionArgs := info.GetActionArgs(ctx)
	const durationMinArgKey = "duration_min"
	durationArg := actionArgs.AsDuration(ctx, durationMinArgKey, 24*60, time.Minute)

	// Get uptime from device.
	sshRunner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	uptime, err := cros.Uptime(ctx, sshRunner.Run)
	if err != nil {
		return errors.Annotate(err, "assert uptime is less than duration: failed to get uptime from device").Err()
	}

	// Evaluate assertion.
	if !(*uptime < durationArg) {
		return errors.Reason("assert uptime is less than duration: device uptime of %s is not less than %s", *uptime, durationArg).Err()
	}
	log.Debugf(ctx, "Device uptime of %s is less than %s", *uptime, durationArg)
	return nil
}

// assertReleaseProcessMatchesExec passes if the release process this specific
// btpeer should use ("chameleond" or "image") matches the value passed in the
// required "expected_release_process" arg.
//
// The btpeer will be chosen to use the "chameleond" process if it is not chosen
// to use the new "image" release process.
//
// The btpeer will be chosen to use the "image" process if it currently has a
// custom image installed (i.e. existence of an image UUID in scope) or the
// hostname of the primary dut in this testbed is present in the image release
// config's NextImageVerificationDutPool.
//
// Note: When we are ready to fully switch over to the new "image" release
// process for all btpeers this exec and its usages should be removed, as all
// btpeers would use the "image" process.
func assertReleaseProcessMatchesExec(ctx context.Context, info *execs.ExecInfo) error {
	// Process exec arg.
	const imageProcess = "image"
	const chameleondProcess = "chameleond"
	const expectedReleaseProcessArg = "expected_release_process"
	argsMap := info.GetActionArgs(ctx)
	expectedReleaseProcess := argsMap.AsString(ctx, expectedReleaseProcessArg, "")
	if expectedReleaseProcess == "" {
		return errors.Reason("assert release process matches: missing required exec arg %q", expectedReleaseProcessArg).Err()
	}
	if expectedReleaseProcess != imageProcess && expectedReleaseProcess != chameleondProcess {
		return errors.Reason(
			"assert release process matches: invalid exec arg %q value %q, must be either %q or %q",
			expectedReleaseProcessArg,
			expectedReleaseProcess,
			imageProcess,
			chameleondProcess,
		).Err()
	}

	// Collect and validate state.
	btpeerScopeState, err := getBtpeerScopeState(ctx, info)
	if err != nil {
		return errors.Annotate(err, "assert release process matches: failed to get btpeer scope state").Err()
	}
	if btpeerScopeState.GetRaspiosCrosBtpeerImage().GetReleaseConfig() == nil {
		return errors.Reason("assert release process matches: invalid btpeer scope state: RaspiosCrosBtpeerImage.ReleaseConfig is nil").Err()
	}

	// Determine actual release process this btpeers should use.
	actualReleaseProcess := chameleondProcess
	selectionReason := "btpeer not selected for new %q release process"
	if btpeerScopeState.GetRaspiosCrosBtpeerImage().GetInstalledImageUuid() != "" {
		// Already on a custom image, so we must continue to use custom images to
		// prevent breaking chameleond on the btpeer.
		actualReleaseProcess = imageProcess
		selectionReason = fmt.Sprintf("btpeer has custom image installed with image UUID %q", btpeerScopeState.GetRaspiosCrosBtpeerImage().GetInstalledImageUuid())
	} else {
		dut := info.GetDut()
		if dut == nil {
			return errors.Reason("assert release process matches: dut is nil").Err()
		}
		for _, dutHostname := range btpeerScopeState.GetRaspiosCrosBtpeerImage().GetReleaseConfig().GetNextImageVerificationDutPool() {
			if strings.EqualFold(dut.Name, dutHostname) {
				actualReleaseProcess = imageProcess
				selectionReason = fmt.Sprintf("primary dut hostname %q present in image release config next image verification dut pool", dut.Name)
			}
		}
	}
	logging.Infof(
		ctx,
		"Selected the %q release process for btpeer resource %q: %s",
		actualReleaseProcess,
		info.GetActiveResource(),
		selectionReason,
	)

	// Evaluate expectation.
	if actualReleaseProcess != expectedReleaseProcess {
		return errors.Reason("assert release process matches: expected %q != actual %q", expectedReleaseProcess, actualReleaseProcess).Err()
	}
	return nil
}

// assertIgnoreFilePresented asserts the chameleond update process if the ignore_btpeer file presents in a btpeer device.
//
// This is a temporary function to prevent chameleond updates on btpeers.
// It should be removed when the image-based release becomes stable.
func assertIgnoreFilePresent(ctx context.Context, info *execs.ExecInfo) error {
	sshRunner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	pathToFile := "/etc/chromiumos/ignore_btpeer_update"
	exists, err := ssh.TestFileExists(ctx, sshRunner, pathToFile)
	if err != nil {
		return errors.Annotate(err, "failed to verify if ignore_btpeer_update exists or not").Err()
	}

	if exists {
		return errors.Reason("assert btpeer from running as %s is presented", pathToFile).Err()
	}

	return nil
}

func restartNetworkingServiceExec(ctx context.Context, info *execs.ExecInfo) error {
	sshRunner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	if _, err := sshRunner.Run(ctx, time.Minute, "service networking restart"); err != nil {
		return errors.Annotate(err, "failed to restart networking service").Err()
	}

	log.Debugf(ctx, "Networking service were restarted")
	return nil
}

func init() {
	execs.Register("btpeer_state_broken", setStateBrokenExec)
	execs.Register("btpeer_state_working", setStateWorkingExec)
	execs.Register("btpeer_reboot", rebootExec)
	execs.Register("btpeer_assert_uptime_is_less_than_duration", assertUptimeIsLessThanDurationExec)
	execs.Register("btpeer_assert_release_process_matches", assertReleaseProcessMatchesExec)
	execs.Register("btpeer_assert_ignore_file_present", assertIgnoreFilePresent)
	execs.Register("btpeer_restart_networking_service", restartNetworkingServiceExec)
}
