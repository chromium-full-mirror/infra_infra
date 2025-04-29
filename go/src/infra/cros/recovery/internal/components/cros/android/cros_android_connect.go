// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/adb"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// ADBConnect connect DUT by ADB.
// If device already connected then it will be skipped, if forceReconnect set as false.
func ADBConnect(ctx context.Context, retryCount int, retryinterval time.Duration, forceReconnect bool, singleRunTimeout time.Duration, dut *tlw.Dut) error {
	if dut == nil {
		return errors.Reason("adb connect: dut is not provided").Err()
	}
	log.Debugf(ctx, "Check if %q is already connected!", dut.Name)
	if adb.IsConnected(ctx, dut, singleRunTimeout) {
		// If the device is connected and there is no request to reconnect,
		// then we need to root service to run commands as the root user.
		if !forceReconnect {
			log.Infof(ctx, "Device is already connected, no need to reconnect just root it!")
			if _, err := adb.Root(ctx, dut, singleRunTimeout); err != nil {
				log.Debugf(ctx, "Fail to root device: %s", err)
				forceReconnect = true
			} else if adb.IsConnected(ctx, dut, singleRunTimeout) {
				log.Infof(ctx, "Device rooted!")
				return nil
			} else {
				log.Infof(ctx, "Device disconnected after rooting!")
			}
		}
		if forceReconnect {
			log.Infof(ctx, "Device is already listed so we disconnect it first")
			if _, err := adb.Disconnect(ctx, dut, singleRunTimeout); err != nil {
				log.Debugf(ctx, "Fail to disconnect device: %s", err)
			}
		}
	}

	connect := func() error {
		log.Infof(ctx, "Try to connect to %q by adb", dut.Name)
		if _, err := adb.Connect(ctx, dut, singleRunTimeout); err != nil {
			return errors.Annotate(err, "fail to connect").Err()
		}
		if _, err := adb.Root(ctx, dut, singleRunTimeout); err != nil {
			return errors.Annotate(err, "fail to root service, event when expected").Err()
		}
		if adb.IsConnected(ctx, dut, singleRunTimeout) {
			return nil
		}
		return errors.Reason("failed to find device").Err()
	}
	if retryErr := retry.LimitCount(ctx, retryCount, retryinterval, connect, "adb connect"); retryErr != nil {
		return errors.Annotate(retryErr, "adb connect").Err()
	}
	log.Infof(ctx, "Device: %q is connected!", dut.Name)
	return nil
}
