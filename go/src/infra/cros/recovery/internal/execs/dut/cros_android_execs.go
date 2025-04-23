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
	if info.GetChromeos().GetOsRestriction() == tlw.ChromeOS_OSR_ANDROID_ONLY {
		log.Infof(ctx, "Device restricted to Android only!")
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
	if info.GetChromeos().GetOsRestriction() == tlw.ChromeOS_OSR_ANDROID_ONLY {
		log.Infof(ctx, "Device restricted to Android only!")
		return errors.Reason("is cros chromeos based: OS restricted to Android").Err()
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
	if info.GetChromeos().GetOsRestriction() == tlw.ChromeOS_OSR_ANDROID_ONLY {
		log.Infof(ctx, "Device restricted to Android only!")
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
	if info.GetChromeos().GetOsRestriction() == tlw.ChromeOS_OSR_ANDROID_ONLY {
		log.Infof(ctx, "Device restricted to Android only!")
		return nil
	}
	return errors.Reason("is android based or previous on android OS: non of it").Err()
}

func matchOsRestrictionExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	restrictionStr := actionMap.AsString(ctx, "restriction", "")
	restrictionId, ok := tlw.ChromeOS_OSRestruction_value[restrictionStr]
	if !ok {
		return errors.Reason("match OS restriction: incorrect value %q", restrictionStr).Err()
	}
	restriction := tlw.ChromeOS_OSRestruction(restrictionId)
	dutRestriction := info.GetChromeos().GetOsRestriction()
	if dutRestriction != restriction {
		return errors.Reason("match OS restriction: restriction does not matched %q with %q", dutRestriction.String(), restriction.String()).Err()
	}
	return nil
}

func doesNotMatchOsRestrictionExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	for _, restrictionStr := range actionMap.AsStringSlice(ctx, "restrictions", nil) {
		restrictionId, ok := tlw.ChromeOS_OSRestruction_value[restrictionStr]
		if !ok {
			return errors.Reason("does not match OS restriction: incorrect value %q", restrictionStr).Err()
		}
		restriction := tlw.ChromeOS_OSRestruction(restrictionId)
		if info.GetChromeos().GetOsRestriction() == restriction {
			return errors.Reason("does not match OS restriction: restriction matched with %q", restriction.String()).Err()
		}
	}
	return nil
}

func setFromOsRestrictionExec(ctx context.Context, info *execs.ExecInfo) error {
	restriction := info.GetChromeos().GetOsRestriction()
	log.Debugf(ctx, "OS restriction: %s", restriction.String())
	switch restriction {
	case tlw.ChromeOS_OSR_ANDROID_ONLY:
		info.GetChromeos().IsAndroidBased = true
		log.Infof(ctx, "DUT marked as Android based device!")
	case tlw.ChromeOS_OSR_CHROMEOS_ONLY:
		log.Infof(ctx, "DUT marked as Chrome based device!")
	default:
		log.Infof(ctx, "No changes to the type of the device!")
	}
	return nil
}

func init() {
	execs.Register("cros_is_android_based", isCrosAndroidBasedExec)
	execs.Register("cros_is_not_android_based", isNotCrosAndroidBasedExec)
	execs.Register("cros_is_previous_android_os_type", isPreviousAndroidOSTypeExec)
	execs.Register("cros_is_previous_android_based_or_os_type", isAndroidBasedOrPreviousOSTypeExec)
	execs.Register("cros_set_as_android_based", setCrosAsAndroidBasedExec)
	execs.Register("cros_set_as_chrome_based", setCrosAsChromeBasedExec)
	execs.Register("cros_match_os_restriction", matchOsRestrictionExec)
	execs.Register("cros_not_match_os_restriction", doesNotMatchOsRestrictionExec)
	execs.Register("cros_set_from_os_restriction", setFromOsRestrictionExec)
}
