// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// isCrosAndroidBasedExec checks if ChromeOS is based on Android.
func isCrosAndroidBasedExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetIsAndroidBased() {
		log.Infof(ctx, "DUT is Android based device!")
		return nil
	}
	log.Infof(ctx, "DUT is Chrome based device!")
	return errors.Reason("is cros android based: OS based on Chrome").Err()
}

// isNotCrosAndroidBasedExec checks if device is not Android based but based on ChromeOS.
func isNotCrosAndroidBasedExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetIsAndroidBased() {
		log.Infof(ctx, "DUT is Android based device!")
		return errors.Reason("is cros chromeos based: OS based on Android").Err()
	}
	log.Infof(ctx, "DUT is ChromeOS based device!")
	return nil
}

func setCrosAsAndroidBasedExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetChromeos().IsAndroidBased = true
	log.Infof(ctx, "DUT marked as Android based device!")
	return nil
}

func setCrosAsChromeBasedExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetChromeos().IsAndroidBased = false
	log.Infof(ctx, "DUT marked as Chrome based device!")
	return nil
}

func isPreviousAndroidOSTypeExec(ctx context.Context, info *execs.ExecInfo) error {
	osType := info.GetDut().GetVersionInfo().GetOsType()
	if osType == tlw.VersionInfo_ANDROID {
		log.Infof(ctx, "Previous OS type is Android!")
		return nil
	}
	log.Infof(ctx, "OS type based on Version info: %s", osType)
	return errors.Reason("is AndroidOS type: provision info missed").Err()
}

func isAndroidBasedOrPreviousOSTypeExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetIsAndroidBased() {
		log.Infof(ctx, "DUT is Android based device!")
		return nil
	}
	if info.GetDut().GetVersionInfo().GetOsType() == tlw.VersionInfo_ANDROID {
		log.Infof(ctx, "Previous OS type is Android!")
		return nil
	}
	return errors.Reason("is android based or previous on android OS: non of it").Err()
}

func init() {
	execs.Register("cros_is_android_based", isCrosAndroidBasedExec)
	execs.Register("cros_is_not_android_based", isNotCrosAndroidBasedExec)
	execs.Register("cros_is_previous_android_os_type", isPreviousAndroidOSTypeExec)
	execs.Register("cros_is_previous_android_based_or_os_type", isAndroidBasedOrPreviousOSTypeExec)
	execs.Register("cros_set_as_android_based", setCrosAsAndroidBasedExec)
	execs.Register("cros_set_as_chrome_based", setCrosAsChromeBasedExec)
}
