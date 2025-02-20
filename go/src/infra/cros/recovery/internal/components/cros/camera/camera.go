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

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
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

// InterfaceType get the type of the camera interface ie. "usb"
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

// ShouldRunAudit determines if audit should run.
// The audit should run if any of the following is true:
// - last audit is done auditInterval duration ago
func ShouldRunAudit(ctx context.Context, metric metrics.Metrics, dut *tlw.Dut, auditInterval time.Duration) (bool, error) {

	if dut == nil {
		return false, errors.New("should run camera audit: dut not provided")
	}

	karteQuery := &metrics.Query{
		Hostname:   dut.Name,
		ActionKind: metrics.AuditCameraKind,
		Limit:      1,
	}
	queryRes, err := metric.Search(ctx, karteQuery)
	if err != nil {
		return false, errors.Annotate(err, "should run camera audit: metric search failed").Err()
	}
	if len(queryRes.Actions) == 0 {
		// If no records for a previous badblock execution are
		// detected, we need to allow execution.
		log.Debugf(ctx, "should run camera audit: no previous run found")
		return true, nil
	}

	previousAction := queryRes.Actions[0]

	// last audit is done auditInterval duration ago
	mostRecentExecutionTime := previousAction.StartTime
	now := time.Now()
	nextExecutionTime := mostRecentExecutionTime.Add(auditInterval)
	if now.After(nextExecutionTime) {
		log.Debugf(ctx, "should run camera audit: last audit was done %s ago, which is more than %s ago", now.Sub(mostRecentExecutionTime), auditInterval)
		return true, nil
	}

	return false, nil
}
