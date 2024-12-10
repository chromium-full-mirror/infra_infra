// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/log"
)

// wgetStderrHTTPResponseCodeRegex is regex that matches the HTTP error code
// in wget's stderr response. For example, the error would be "HTTP error 404"
// and this would match it and have a submatch of "404".
var wgetStderrHTTPResponseCodeRegex = regexp.MustCompile(`HTTP error (\d+)`)

// TestPath runs `test <testFlag> <remotePath>` on the host and returns the
// result of the test.
//
// Returns:
//   - (true, nil) if the run exit code is 0, meaning the test passed
//   - (false, nil) if the run exit code is 1, meaning the test ran successfully
//     but did not pass
//   - (false, non-nil error) if the exit code was any other value, meaning that
//     the test did not execute successfully on the remote host
func TestPath(ctx context.Context, sshRunner Runner, testFlag, remotePath string) (bool, error) {
	runResult := sshRunner.RunForResult(ctx, 0, false, "test", testFlag, remotePath)
	exitCode := runResult.GetExitCode()
	if exitCode == 0 {
		return true, nil
	}
	if exitCode == 1 {
		return false, nil
	}
	return false, errors.Reason("failed to run %q on host: got unexpected exit code %d: %s", runResult.GetCommand(), runResult.GetExitCode(), runResult.GetStderr()).Err()
}

// TestFileExists checks the remote host for a file's existence at remoteFilePath
// by running `test -f <remoteFilePath>` on the remote host.
//
// Returns:
// - (true, nil) if a regular file is found at the given path on the host
// - (false, nil) if no file was found
// - (false, non-nil error) if it is unable to determine if the file exists or not
func TestFileExists(ctx context.Context, sshRunner Runner, remoteFilePath string) (bool, error) {
	return TestPath(ctx, sshRunner, "-f", remoteFilePath)
}

// CatFile runs `cat <remoteFilePath>` on the host and returns the
// stdout (the file contents) as a string.
func CatFile(ctx context.Context, sshRunner Runner, remoteFilePath string) (string, error) {
	return sshRunner.Run(ctx, 0, "cat", remoteFilePath)
}

// Reboot runs `reboot` on the host and then waits for the reboot to
// complete. The reboot is seen as finished when the device is ssh-able again.
func Reboot(ctx context.Context, sshRunner Runner, reconnectDelay time.Duration, reconnectInterval time.Duration, reconnectTimeout time.Duration) error {
	// Start reboot over ssh.
	log.Debugf(ctx, "Rebooting host over ssh")
	if err := sshRunner.RunInBackground(ctx, 0, "reboot"); err != nil {
		return err
	}

	// Wait to give it time to shut down ssh until verifying it is back up.
	log.Debugf(ctx, "Waiting %s for host to start rebooting before reconnecting", reconnectDelay)
	time.Sleep(reconnectDelay)
	log.Debugf(ctx, "Attempting to reconnect to host after reboot (%s interval, %s timeout)", reconnectInterval, reconnectTimeout)
	return cros.WaitUntilSSHable(ctx, reconnectTimeout, reconnectInterval, sshRunner.Run)
}

// TryAccess will attempt to run a simple bash command, `true`, on the host over
// ssh to validate that it can connect to the device and run a command.
func TryAccess(ctx context.Context, sshRunner Runner) error {
	_, err := sshRunner.Run(ctx, 0, "true")
	return err
}

// WgetURL will run "wget <downloadURL> [additionalWgetArgs...]" on the host.
//
// On wget run success, just the stdout of wget is returned with an empty
// stdout, 0 HTTP response code (wget does not provide it on success), and a
// nil error.
//
// On wget run failure, the stdout, the stderr, the HTTP error response code,
// and an err is returned. The HTTP error response code will be attempted to
// be parsed from stderr, but if it is not found it will be -1.
func WgetURL(ctx context.Context, sshRunner Runner, timeout time.Duration, downloadURL string, additionalWgetArgs ...string) (stdout, stderr string, httpErrorResponseCode int, err error) {
	wgetArgs := append([]string{downloadURL}, additionalWgetArgs...)
	runResult := sshRunner.RunForResult(ctx, timeout, false, "wget", wgetArgs...)
	stdout = runResult.GetStdout()
	stderr = runResult.GetStderr()
	exitCode := runResult.GetExitCode()
	if exitCode != 0 {
		if stderr != "" {
			// Try to parse the HTTP response code from stderr.
			match := wgetStderrHTTPResponseCodeRegex.FindStringSubmatch(stderr)
			if len(match) == 2 {
				if code, err := strconv.Atoi(match[1]); err != nil {
					httpErrorResponseCode = -1
				} else {
					httpErrorResponseCode = code
				}
			} else {
				httpErrorResponseCode = -1
			}
		} else {
			httpErrorResponseCode = -1
		}
		return stdout, stderr, httpErrorResponseCode, errors.Reason("command %q returned non-zero exit code", runResult.GetCommand()).Err()
	}
	return stdout, "", 0, nil
}

// RecreateDir will delete (if it exists) and then recreate the directory
// at remoteDirPath on the remote host.
func RecreateDir(ctx context.Context, sshRunner Runner, remoteDirPath string) error {
	exists, err := TestPath(ctx, sshRunner, "-d", remoteDirPath)
	if err != nil {
		return errors.Annotate(err, "failed to check if remote dir %q exists", remoteDirPath).Err()
	}
	if exists {
		if _, err := sshRunner.Run(ctx, 10*time.Second, "rm", "-r", remoteDirPath); err != nil {
			return errors.Annotate(err, "failed to remove existing remote dir %q", remoteDirPath).Err()
		}
	}
	if _, err := sshRunner.Run(ctx, time.Second, "mkdir", "-p", remoteDirPath); err != nil {
		return errors.Annotate(err, "failed to create new remote dir %q", remoteDirPath).Err()
	}
	return nil
}
