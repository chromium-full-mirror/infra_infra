// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/scopes"
	"infra/cros/recovery/version"
)

// hasStableVersionServicePathExec checks the path of the stable version service in the context params.
func hasStableVersionServicePathExec(ctx context.Context, info *execs.ExecInfo) error {
	if _, ok := scopes.GetParam(ctx, scopes.ParamKeyStableVersionServicePath); ok {
		return nil
	}
	return errors.Reason("has stable version path: not specified").Err()
}

// hasCrosImageStableVersionActionExec verifies that DUT provides ChromeOS image name as part of stable version.
// Example: board-release/R90-13816.47.0.
func hasCrosImageStableVersionActionExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() != nil {
		sv, err := version.ByDut(ctx, info.GetDut())
		if err != nil {
			return errors.Annotate(err, "cros has stable version").Err()
		}
		osImage := sv.GetOsImagePath()
		log.Debugf(ctx, "Stable version for cros: %q", osImage)
		if osImage != "" && strings.Contains(osImage, "/") {
			return nil
		}
	}
	return errors.Reason("cros has stable version: not found").Err()
}

// hasFwVersionStableVersionActionExec verifies that DUT provides ChromeOS firmware version name as part of stable version.
// Example: Google_Board.13434.261.0.
func hasFwVersionStableVersionActionExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() != nil {
		sv, err := version.ByDut(ctx, info.GetDut())
		if err != nil {
			return errors.Annotate(err, "cros has stable firmware version").Err()
		}
		log.Debugf(ctx, "Stable version for firmware version: %q", sv.GetFirmwareRoVersion())
		if sv.GetFirmwareRoVersion() != "" {
			return nil
		}
	}
	return errors.Reason("cros has stable firmware version: not found").Err()
}

// hasFwImageStableVersionActionExec verifies that DUT provides ChromeOS firmware image name as part of stable version.
// Example: board-firmware/R87-13434.261.0
func hasFwImageStableVersionActionExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() != nil {
		sv, err := version.ByDut(ctx, info.GetDut())
		if err != nil {
			return errors.Annotate(err, "cros has stable firmware image version").Err()
		}
		fwImage := sv.GetFirmwareRoImagePath()
		log.Debugf(ctx, "Stable version for firmware image: %q", fwImage)
		if fwImage != "" && strings.Contains(fwImage, "/") {
			return nil
		}
	}
	return errors.Reason("cros has stable firmware image version: not found").Err()
}

func init() {
	execs.Register("has_stable_version_service_path", hasStableVersionServicePathExec)
	execs.Register("has_stable_version_cros_image", hasCrosImageStableVersionActionExec)
	execs.Register("has_stable_version_fw_version", hasFwVersionStableVersionActionExec)
	execs.Register("has_stable_version_fw_image", hasFwImageStableVersionActionExec)
}
