// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
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

// isCrosChromeBasedExec checks if ChromeOS is based on Chrome.
func isCrosChromeBasedExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetIsAndroidBased() {
		log.Infof(ctx, "DUT is Android based device!")
		return errors.Reason("is cros chrome based: OS based on Android").Err()
	}
	log.Infof(ctx, "DUT is Chrome based device!")
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

func init() {
	execs.Register("cros_is_android_based", isCrosAndroidBasedExec)
	execs.Register("cros_is_chrome_based", isCrosChromeBasedExec)
	execs.Register("cros_set_as_android_based", setCrosAsAndroidBasedExec)
	execs.Register("cros_set_as_chrome_based", setCrosAsChromeBasedExec)
}
