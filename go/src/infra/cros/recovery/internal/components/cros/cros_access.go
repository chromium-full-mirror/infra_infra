// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/linux"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
)

const (
	DefaultPingCount = 3
	// Default timeout for simple timeout check
	DefaultSSHTimeout = 10 * time.Second
)

// IsPingable checks whether the resource is pingable
func IsPingable(ctx context.Context, count int, ping components.Pinger) error {
	err := ping(ctx, count)
	return errors.Annotate(err, "is pingable").Err()
}

// IsNotPingable checks whether the resource is not pingable
func IsNotPingable(ctx context.Context, count int, ping components.Pinger) error {
	if err := ping(ctx, count); err != nil {
		log.Debugf(ctx, "Resource is not pingable, but expected!")
		return nil
	}
	return errors.Reason("not pingable: is pingable").Err()
}

// IsSSHable checks whether the resource is sshable
func IsSSHable(ctx context.Context, run components.Runner, timeout time.Duration) error {
	_, err := run(ctx, timeout, "true")
	return errors.Annotate(err, "is sshable").Err()
}

// IsFileSystemWritable confirms the stateful file systems are writable.
//
// The standard linux response to certain unexpected file system errors
// (including hardware errors in block devices) is to change the file
// system status to read-only.  This checks that it hasn't happened.
//
// The test doesn't check various bind mounts; those are expected to
// fail the same way as their underlying main mounts.  Whether the
// Linux kernel can guarantee that is untested...
func IsFileSystemWritable(ctx context.Context, run components.Runner, testDirs []string) error {
	for _, testDir := range testDirs {
		if err := linux.IsPathWritable(ctx, run, testDir); err != nil {
			return errors.Annotate(err, "file system writable").Err()
		}
		log.Debugf(ctx, "Directory %s is writable.", testDir)
	}
	return nil
}

const (
	PingRetryInterval = 5 * time.Second
	SSHRetryInterval  = 10 * time.Second
)

// WaitUntilPingable waiting resource to be pingable.
func WaitUntilPingable(ctx context.Context, waitTime, waitInterval time.Duration, countPerAttempt int, ping components.Pinger) error {
	log.Debugf(ctx, "Start ping for the next %s.", waitTime)
	return retry.WithTimeout(ctx, waitInterval, waitTime, func() error {
		return IsPingable(ctx, countPerAttempt, ping)
	}, "wait to ping")
}

// WaitUntilNotPingable waiting resource to be not pingable.
func WaitUntilNotPingable(ctx context.Context, waitTime, waitInterval time.Duration, countPerAttempt int, ping components.Pinger) error {
	return retry.WithTimeout(ctx, waitInterval, waitTime, func() error {
		return IsNotPingable(ctx, countPerAttempt, ping)
	}, "wait to be not pingable")
}

// WaitUntilSSHable waiting resource to be sshable.
func WaitUntilSSHable(ctx context.Context, waitTime, waitInterval time.Duration, run components.Runner) error {
	log.Debugf(ctx, "Start SSH check for the next %s.", waitTime)
	return retry.WithTimeout(ctx, waitInterval, waitTime, func() error {
		return IsSSHable(ctx, run, DefaultSSHTimeout)
	}, "wait to ssh access")
}
