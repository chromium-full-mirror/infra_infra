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

	adbTool "go.chromium.org/infra/cros/recovery/internal/adb"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/components/cft/adb"
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
	client, err := getADBClient(ctx, dut)
	if err != nil {
		return errors.Annotate(err, "adb connect").Err()
	}
	adbPort := adbTool.Port(ctx)
	deviceName := fmt.Sprintf("%s:%d", dut.Name, adbPort)

	adbRoot := func() error {
		_, err := adb.RunCommand(ctx, client, singleRunTimeout, "-s", deviceName, "root")
		return err
	}

	log.Debugf(ctx, "Check if %q is already connected!", deviceName)
	if isConnected(ctx, deviceName, client, singleRunTimeout) {
		// If the device is connected and there is no request to reconnect,
		// then we need to root service to run commands as the root user.
		if !forceReconnect {
			log.Infof(ctx, "Device is already connected, no need to reconnect just root it!")
			if err := adbRoot(); err != nil {
				log.Debugf(ctx, "Fail to root device: %s", err)
				forceReconnect = true
			} else if isConnected(ctx, deviceName, client, singleRunTimeout) {
				log.Infof(ctx, "Device rooted!")
				return nil
			} else {
				log.Infof(ctx, "Device disconnected after rooting!")
			}
		}
		if forceReconnect {
			log.Infof(ctx, "Device is already listed so we disconnect it first")
			if _, err := adb.ExecCommand(ctx, client, singleRunTimeout, "disconnect", deviceName); err != nil {
				log.Debugf(ctx, "Fail to disconnect device: %s", err)
			}
		}
	}

	connect := func() error {
		log.Infof(ctx, "Try to connect to %q by adb", dut.Name)
		if _, err := adb.ExecCommand(ctx, client, singleRunTimeout, "connect", deviceName); err != nil {
			return errors.Annotate(err, "fail to connect").Err()
		}
		if err := adbRoot(); err != nil {
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

func getADBClient(ctx context.Context, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if !adbTool.UseLocal(ctx) {
		return cft.ADBClientFromScope(ctx, dut)
	}
	return nil, nil
}

func isConnected(ctx context.Context, deviceName string, client api.ADBServiceClient, timeout time.Duration) bool {
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
