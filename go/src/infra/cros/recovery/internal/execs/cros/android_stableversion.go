// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/version"
)

func isOnAndroidOSStableVersionExec(ctx context.Context, info *execs.ExecInfo) error {
	stableVersion, err := version.ByResource(ctx, version.AndroidOSType, info.GetDut(), info.GetDut().Name)
	if err != nil {
		return errors.Annotate(err, "device stable version not defined").Err()
	}
	expected := stableVersion.GetOsVersion()
	if expected == "" {
		return errors.Reason("device does not have OS stable version").Err()
	}
	log.Debugf(ctx, "Expected version: %s", stableVersion)

	run := info.NewRunner(info.GetDut().Name)
	argsMap := info.GetActionArgs(ctx)
	timeout := argsMap.AsDuration(ctx, "timeout", 10, time.Second)
	fromDevice, err := run(ctx, timeout, "getprop", "ro.build.version.incremental")
	if err != nil {
		return errors.Annotate(err, "read android build incremental version").Err()
	}
	log.Infof(ctx, "ro.build.version.incremental: %s", fromDevice)

	if fromDevice != expected {
		return errors.Reason("match os version: mismatch, expected %q, found %q", expected, fromDevice).Err()
	}
	return nil
}

func init() {
	execs.Register("android_is_on_os_stable_version", isOnAndroidOSStableVersionExec)
}
