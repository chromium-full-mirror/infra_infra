// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros/camera"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/logger/metrics"
	"infra/cros/recovery/tlw"
)

const (
	interfaceTypeErrorMsg   = "audit camera: failed to get interface type. (camera index: %d)"
	tryCaptureFrameErrorMsg = "audit camera: failed to capture frame. (camera index: %d)"
)

// auditCameraExec audit the camera of DUT and updates the camera state.
// The state is based on the condition as follows:
// - if there is no camera: "not detected"
// -  if all usb camera can capture frame: "normal"
// - else set as "need replacement"
func auditCameraExec(ctx context.Context, info *execs.ExecInfo) (rErr error) {
	ha := info.NewHostAccess(info.GetDut().Name)

	cameraInfo := info.GetChromeos().GetCamera()
	if cameraInfo == nil {
		// initialize a new camera info if not exist in tlw
		log.Debugf(ctx, "audit camera: the camera was not initialized, so initializing now.")
		cameraInfo = &tlw.Camera{}
		info.GetChromeos().Camera = cameraInfo
	}

	argsMap := info.GetActionArgs(ctx)
	auditIntervalHours := argsMap.AsDuration(ctx, "audit_interval_hours", 7*24, time.Hour)

	shouldRunAudit, err := camera.ShouldRunAudit(ctx, info.GetMetrics(), info.GetDut(), auditIntervalHours)
	if err != nil {
		return errors.Annotate(err, "audit camera: unable to fetch metric.").Err()
	}

	if !shouldRunAudit {
		log.Debugf(ctx, "audit camera: camera audit skipped.")
		return nil
	}

	karteAction := info.NewMetric(metrics.AuditCameraKind)

	defer func() {
		// update status for action, needed for ShouldRunAudit function.
		karteAction.UpdateStatus(rErr)
	}()

	cameraInfo.State = tlw.HardwareState_HARDWARE_NOT_DETECTED

	cameraCount, err := camera.CountByConfig(ctx, ha)
	if err != nil {
		return errors.Annotate(err, "audit camera: failed to get camera count.").Err()
	}
	if cameraCount == 0 {
		log.Infof(ctx, "audit camera: no camera is detected.")
		return nil
	}

	var errs []error
	for cameraIndex := 0; cameraIndex < cameraCount; cameraIndex++ {
		interfaceType, err := camera.InterfaceType(ctx, ha, cameraIndex)
		if err != nil {
			err = errors.Annotate(err, interfaceTypeErrorMsg, cameraIndex).Err()
			errs = append(errs, err)
			continue
		}
		switch interfaceType {
		case "usb":
			if err := camera.TryCaptureFrame(ctx, ha, cameraIndex); err != nil {
				err = errors.Annotate(err, tryCaptureFrameErrorMsg, cameraIndex).Err()
				errs = append(errs, err)
				continue
			}
		default:
			log.Infof(ctx, "audit camera: ignoring non-usb interface type %s. (camera index: %d)", interfaceType, cameraIndex)
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		log.Infof(ctx, "audit camera: setting the camera state from %s to %s", cameraInfo.State.String(),
			tlw.HardwareState_HARDWARE_NEED_REPLACEMENT.String())
		cameraInfo.State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
		return err
	}

	cameraInfo.State = tlw.HardwareState_HARDWARE_NORMAL
	return nil
}

func init() {
	execs.Register("cros_audit_camera", auditCameraExec)
}
