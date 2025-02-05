// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	adbTool "infra/cros/recovery/internal/adb"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/components/cft/adb"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
	"infra/cros/recovery/tlw"
)

// ADBConnect connect DUT by ADB.
// If device already connected then it will be skipped, if forceReconnect set as false.
func ADBConnect(ctx context.Context, retryCount int, retryinterval time.Duration, forceReconnect bool, singleRunTimeout time.Duration, dut *tlw.Dut) error {
	if dut == nil {
		return errors.Reason("adb connect: dut is not provided").Err()
	}
	client, err := getADBClient(ctx, dut)
	if err != nil {
		return errors.Annotate(err, "adb connect").Err()
	}
	adbPort := adbTool.Port(ctx)
	deviceName := fmt.Sprintf("%s:%d", dut.Name, adbPort)

	if isConnected(ctx, deviceName, client, singleRunTimeout) {
		if forceReconnect {
			log.Infof(ctx, "Device is already listed so we disconnect it first")
			if _, err := adb.ExecCommand(ctx, client, singleRunTimeout, "disconnect", deviceName); err != nil {
				log.Debugf(ctx, "Fail to disconnect device: %s", err)
			}
		} else {
			log.Infof(ctx, "Device is already connected, no need to reconnect!")
			return nil
		}
	}

	// Only restart ADB server when run in a container for a single DUT,
	// as it is not safe to restart it when other devices are connected.
	restartServerIfNeed(ctx, client, singleRunTimeout)

	connect := func() error {
		log.Infof(ctx, "Try to connect to %q by adb", dut.Name)
		if _, err := adb.ExecCommand(ctx, client, singleRunTimeout, "connect", deviceName); err != nil {
			return errors.Annotate(err, "fail to connect").Err()
		}
		if _, err := adb.ExecCommand(ctx, client, singleRunTimeout, "root"); err != nil {
			return errors.Annotate(err, "fail to root service, event when expected").Err()
		}
		if res, err := adb.ExecCommand(ctx, client, singleRunTimeout, "devices"); err != nil {
			return errors.Annotate(err, "fail to read adb devices, after connection").Err()
		} else if out := string(res.GetStdout()); out != "" {
			if !strings.Contains(out, deviceName) {
				return errors.Reason("fail to find connected device %q in list of devices", deviceName).Err()
			}
		} else {
			return errors.Reason("fail to read adb devices, after connection").Err()
		}
		return nil
	}
	if retryErr := retry.LimitCount(ctx, retryCount, retryinterval, connect, "adb connect"); retryErr != nil {
		return errors.Annotate(retryErr, "adb connect").Err()
	}
	log.Infof(ctx, "Device: %q is connected!", deviceName)
	return nil
}

func restartServerIfNeed(ctx context.Context, client api.ADBServiceClient, timeout time.Duration) {
	if adbTool.UseLocal(ctx) {
		log.Infof(ctx, "Skip restart ADB server as using local shared version!")
	} else {
		if _, err := adb.ExecCommand(ctx, client, timeout, "kill-server"); err != nil {
			log.Debugf(ctx, "adb devices error: %s", err)
		}
		if _, err := adb.ExecCommand(ctx, client, timeout, "start-server"); err != nil {
			log.Debugf(ctx, "adb devices error: %s", err)
		}
	}
}

func getADBClient(ctx context.Context, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if !adbTool.UseLocal(ctx) {
		return cft.ADBClientFromScope(ctx, dut)
	}
	return nil, nil
}

func isConnected(ctx context.Context, deviceName string, client api.ADBServiceClient, timeout time.Duration) bool {
	log.Debugf(ctx, "Check if %q is already connected!", deviceName)
	if res, err := adb.ExecCommand(ctx, client, timeout, "devices"); err != nil {
		log.Debugf(ctx, "Fail to read adb devices, assume device isn't listed yet: %s", err)
		return false
	} else if out := string(res.GetStdout()); out != "" && strings.Contains(out, deviceName) {
		log.Debugf(ctx, "Device is listed.")
		return true
	} else {
		log.Debugf(ctx, "Device isn't listed yet: %s", err)
		return false
	}
}
