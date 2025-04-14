// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/camera"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	interfaceTypeErrorMsg          = "audit camera: failed to get interface type. (camera index: %d)"
	tryCaptureFrameErrorMsg        = "audit camera: failed to capture frame. (camera device: %s)"
	usbCameraCountNotMatchErrorMsg = "audit camera: number of usb camera device files does not match the number of usb cros cameras. (deviceFilesCount: %d, crosCameraCount: %d)"
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
	func() {
		usbCameraCount := 0
		for cameraIndex := range cameraCount {
			interfaceType, err := camera.InterfaceType(ctx, ha, cameraIndex)
			if err != nil {
				err = errors.Annotate(err, interfaceTypeErrorMsg, cameraIndex).Err()
				errs = append(errs, err)
				continue
			}
			switch interfaceType {
			case "usb":
				usbCameraCount += 1
			default:
				log.Infof(ctx, "audit camera: ignoring non-usb interface type %s. (camera index: %d)", interfaceType, cameraIndex)
			}
		}

		if usbCameraCount == 0 {
			log.Infof(ctx, "audit camera: device does not have a USB camera.")
			return
		}

		usbCameraDeviceFiles, err := camera.GetUsbDeviceFiles(ctx, ha)
		if err != nil {
			err = errors.Annotate(err, "audit camera: failed to get USB camera device files.").Err()
			errs = append(errs, err)
			return
		}

		if len(usbCameraDeviceFiles) != usbCameraCount {
			log.Errorf(ctx, "audit camera: number of USB camera device files %d", len(usbCameraDeviceFiles))
			log.Errorf(ctx, "audit camera: number of USB camera count from cros config %d", usbCameraCount)
			errs = append(errs, errors.New(
				fmt.Sprintf(usbCameraCountNotMatchErrorMsg, len(usbCameraDeviceFiles), usbCameraCount),
			))
			return
		}

		for _, usbCameraDeviceFile := range usbCameraDeviceFiles {
			if err := camera.TryCaptureFrame(ctx, ha, usbCameraDeviceFile); err != nil {
				err = errors.Annotate(err, tryCaptureFrameErrorMsg, usbCameraDeviceFile).Err()
				errs = append(errs, err)
				continue
			}
		}
	}()

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
