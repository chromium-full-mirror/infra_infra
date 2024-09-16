// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

// collectFingerprintMcu read fingerprint_mcu from dut to inventory.
//
// Find the fingerprint_mcu with command and set it on ChromeOS struct
func collectFingerprintMcu(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()

	cros := info.GetChromeos()
	if cros == nil {
		return errors.Reason("collect fingerprint_mcu: only for chromeos devices").Err()
	}

	log.Debugf(ctx, "fingerprint_mcu before update: %s", cros.GetFingerprintMcu())

	// command to grab the fingerprint_mcu from dut
	const fingerprintMcuCmd = `ectool --name=cros_fp chipinfo | grep "name: " | awk '{print $2}'`
	fingerprintMcu, err := r(ctx, info.GetExecTimeout(), fingerprintMcuCmd)
	if err != nil {
		return errors.Annotate(err, "collect fingerprint_mcu").Err()
	}

	log.Debugf(ctx, "fingerprint_mcu to set: %q", fingerprintMcu)
	if fingerprintMcu == "" {
		fingerprintMcu = "None"
	}
	cros.FingerprintMcu = strings.ToUpper(fingerprintMcu)
	return nil
}

func init() {
	execs.Register("cros_collect_fingerprint_mcu", collectFingerprintMcu)
}
