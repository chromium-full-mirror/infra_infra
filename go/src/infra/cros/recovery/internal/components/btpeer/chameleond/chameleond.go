// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	// installDir is a temporary install/download directory used on the btpeer.
	installDir = "/tmp/chameleond_bundle_update"

	// chameleondCommitLogFilePath is the path to the log file on the btpeer that
	// contains the history of installed chameleond bundles' commits.
	//
	// Each line contains the commit and install timestamp separated by a space,
	// ordered from earliest to latest installed.
	chameleondCommitLogFilePath = "/var/log/chameleon_commits"
)

// PrepareEmptyInstallDir prepares the temporary install directory on the
// host that is used for chameleond installation. Deletes and recreates the dir
// if it already exists on the host.
func PrepareEmptyInstallDir(ctx context.Context, sshRunner ssh.Runner) error {
	return ssh.RecreateDir(ctx, sshRunner, installDir)
}

// RemoveInstallDir deletes the temporary install directory on the host that is
// used for chameleond installation.
func RemoveInstallDir(ctx context.Context, sshRunner ssh.Runner) error {
	_, err := sshRunner.Run(ctx, 30*time.Second, "rm", "-r", installDir)
	return err
}

// FetchInstalledChameleondBundleCommit checks the chameleond commit log file
// host to identify the and return the chameleond commit of the chameleond
// bundle currently installed on the btpeer.
func FetchInstalledChameleondBundleCommit(ctx context.Context, sshRunner ssh.Runner) (string, error) {
	logFileExists, err := ssh.TestFileExists(ctx, sshRunner, chameleondCommitLogFilePath)
	if err != nil {
		return "", errors.Annotate(err, "failed to check for chameleond commit log file %q on btpeer", chameleondCommitLogFilePath).Err()
	}
	if !logFileExists {
		return "", errors.Reason("chameleond commit log file not found at %q on btpeer", chameleondCommitLogFilePath).Err()
	}
	lastLogLine, err := sshRunner.Run(ctx, 5*time.Second, "tail", "-1", chameleondCommitLogFilePath)
	if err != nil {
		return "", errors.Annotate(err, "failed to get last line of chameleond commit log file %q on btpeer", chameleondCommitLogFilePath).Err()
	}
	lastLogLineParts := strings.Split(strings.TrimSpace(lastLogLine), " ")
	if len(lastLogLineParts) != 2 {
		return "", errors.Reason("failed to parse chameleond commit and update timestamp from chameleond commit log file line %q", lastLogLine).Err()
	}
	chameleondLastCommit := lastLogLineParts[0]
	chameleondUpdatedAt := lastLogLineParts[1]
	log.Debugf(ctx, "Btpeer has chameleond installed at commit %q since %q", chameleondLastCommit, chameleondUpdatedAt)
	return chameleondLastCommit, nil
}

// InstallChameleondBundle installs chameleond on the btpeer using the bundle
// archive at the provided path on the host.
func InstallChameleondBundle(ctx context.Context, sshRunner ssh.Runner, pathToBundleOnHost string, installBluetoothGRPC bool) error {
	makeInstallCommand := "install"
	sshTimeOut := 10 * time.Minute

	if installBluetoothGRPC {
		makeInstallCommand = "rpi-local-install"
		sshTimeOut = 30 * time.Minute
	}

	if _, err := sshRunner.Run(
		ctx,
		sshTimeOut,

		// Enter install dir and extract bundle contents to it.
		"cd", fmt.Sprintf("'%s'", installDir), "&&",
		"tar", "zxf", fmt.Sprintf("'%s'", pathToBundleOnHost), "-C", ".", "&&",

		// Enter extracted bundle folder.
		"cd", "\"$(find -mindepth 1 -maxdepth 1 -type d)\"", "&&",

		// Install chameleond.
		"make", makeInstallCommand,
		"REMOTE_INSTALL=TRUE",
		fmt.Sprintf("HOST_NOW=%q", time.Now().Format("2006-01-02 03:04:05")),
		fmt.Sprintf("INSTALL_GRPC=%t", installBluetoothGRPC),
	); err != nil {
		return errors.Annotate(err, "failed to install chameleond bundle").Err()
	}

	return nil
}

// AssertChameleondServiceIsRunning will check the status of the chameleond
// service on the host and return a non-nil error if it is not running.
//
// Supports both the legacy init.d chameleond service installed by chameleond
// bundles and the newer systemd chameleond service present on custom images.
//
// The status check timeout is set to 30s to account for a delay in sending the
// ssh command, but the status check normally takes less than a second. It is
// not expected to ever exceed this timeout.
func AssertChameleondServiceIsRunning(ctx context.Context, sshRunner ssh.Runner) error {
	const getStatusTimeout = 30 * time.Second
	hasLegacyService, err := ssh.TestFileExists(ctx, sshRunner, "/etc/init.d/chameleond")
	if err != nil {
		return errors.Annotate(err, "assert chameleond service is running: failed to check if host has legacy init.d chameleond service").Err()
	}
	if hasLegacyService {
		status, err := sshRunner.Run(ctx, getStatusTimeout, "/etc/init.d/chameleond", "status")
		if err != nil {
			return errors.Annotate(err, "assert chameleond service is running: failed to check chameleond init.d service status").Err()
		}
		if !strings.Contains(status, "chameleond is running") {
			return errors.Reason("assert chameleond service is running: chameleond init.d service is not running, got status %q", status).Err()
		}
	} else {
		status, err := sshRunner.Run(ctx, getStatusTimeout, "systemctl", "show", "chameleond.service", "-p", "ActiveState")
		if err != nil {
			return errors.Annotate(err, "assert chameleond service is running: failed to check chameleond systemd service status").Err()
		}
		if strings.TrimSpace(status) != "ActiveState=active" {
			return errors.Reason("assert chameleond service is running: chameleond systemd service is not active, got %q", status).Err()
		}
	}
	return nil
}
