// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package camera contains utilities for auditing camera on DUTs.
package camera

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
)

const (
	cameraCountCmd         = "cros_config /camera count"
	cameraInterfaceTypeCmd = "cros_config /camera/devices/%d interface"
	cameraCaptureFrameCmd  = "yavta -c5 /dev/video%d"
)

// CountByConfig get the number of camera on DUT.
func CountByConfig(ctx context.Context, ha components.HostAccess) (int, error) {
	res, err := ha.Run(ctx, time.Minute, cameraCountCmd)
	if err != nil {
		return 0, errors.Annotate(err, "audit camera: unable to get camera count.").Err()
	}
	cameraCount, err := strconv.Atoi(strings.TrimSpace(res.GetStdout()))
	if err != nil {
		return 0, errors.Annotate(err, "audit camera: unable to convert camera count.").Err()
	}
	return cameraCount, nil
}

// InterfaceType tries to capture a frame.
func InterfaceType(ctx context.Context, ha components.HostAccess, cameraIndex int) (string, error) {
	res, err := ha.Run(ctx, time.Minute, fmt.Sprintf(cameraInterfaceTypeCmd, cameraIndex))
	if err != nil {
		return "", errors.Annotate(err, "audit camera: unable to get camera interface type. (camera index: %d)", cameraIndex).Err()
	}
	return res.GetStdout(), nil
}

// TryCaptureFrame tries to capture a frame.
// The capture takes maximum 10 second time and returns error if failed
func TryCaptureFrame(ctx context.Context, ha components.HostAccess, cameraIndex int) error {
	_, err := ha.Run(ctx, time.Minute*5, fmt.Sprintf(cameraCaptureFrameCmd, cameraIndex))
	if err != nil {
		return errors.Annotate(err, "audit camera: unable to capture frame for camera index: %d.", cameraIndex).Err()
	}
	return nil
}
