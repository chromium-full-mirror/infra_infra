// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// Regexp that match to output of `crosid` from a given DUT.
// Below is an example output of `crosid`:
// SKU='163840'
// CONFIG_INDEX='88'
// FIRMWARE_MANIFEST_KEY='nirwen_ufs'
var crosIDSkuRegexp = regexp.MustCompile("SKU='(.*)'")
var crosIDFirmwareManifestRegexp = regexp.MustCompile("FIRMWARE_MANIFEST_KEY='(.*)'")

func readCrosID(ctx context.Context, run components.Runner, reg *regexp.Regexp) (string, error) {
	out, err := run(ctx, time.Second*15, "crosid")
	if err != nil {
		return "", errors.Annotate(err, "read crosid").Err()
	}
	if reg == nil {
		return out, nil
	}
	line := reg.FindStringSubmatch(out)
	if len(line) == 0 || line[1] == "" {
		return "", errors.Reason("read crosid: empty content").Err()
	}
	return line[1], nil
}

// ReadFirmwareManifestKeyFromCrosID read FIRMWARE_MANIFEST_KEY of crosid output from the DUT.
func ReadFirmwareManifestKeyFromCrosID(ctx context.Context, run components.Runner) (string, error) {
	v, err := readCrosID(ctx, run, crosIDFirmwareManifestRegexp)
	if err != nil {
		return "", errors.Annotate(err, "read firmware manifest key from crosid").Err()
	}
	return v, nil
}

// ReadSkuKeyFromCrosID read SKU of crosid output from the DUT.
func ReadSkuKeyFromCrosID(ctx context.Context, run components.Runner) (string, error) {
	v, err := readCrosID(ctx, run, crosIDSkuRegexp)
	if err != nil {
		return "", errors.Annotate(err, "read sku key from crosid").Err()
	}
	return v, nil
}
