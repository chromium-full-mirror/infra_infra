// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/logger"
)

// EnableWiFi enables WiFi on DUT.
func EnableWiFi(ctx context.Context, retryInterval, timeout time.Duration, run components.Runner, log logger.Logger, serialNumber string) error {
	const adbEnableWiFiCmd = "adb -s %s shell svc wifi enable"
	// TODO(b/259746452): use shell.QuoteUnix for quoting
	cmd := fmt.Sprintf(adbEnableWiFiCmd, serialNumber)
	err := retry.WithTimeout(ctx, retryInterval, timeout, func() error {
		if _, err := run(ctx, time.Minute, cmd); err != nil {
			return errors.Annotate(err, "try to enable wifi").Err()
		}
		return nil
	}, "try to enable wifi")
	if err != nil {
		return errors.Annotate(err, "enable wifi").Err()
	}
	log.Debugf("wifi is enabled on the device: %q", serialNumber)
	return nil
}

// ConnectToWiFiNetwork connects DUT to WiFi network.
func ConnectToWiFiNetwork(ctx context.Context, retryInterval, timeout time.Duration, run components.Runner, log logger.Logger, serialNumber string, wifiSSID string, wifiSecurityType string, wifiPassword string) error {
	const adbConnectNetworkCmd = "adb -s %s shell cmd wifi connect-network"
	// TODO(b/259746452): use shell.QuoteUnix for quoting
	cmd := fmt.Sprintf(adbConnectNetworkCmd, serialNumber)
	err := retry.WithTimeout(ctx, retryInterval, timeout, func() error {
		if _, err := run(ctx, time.Minute, cmd, wifiSSID, wifiSecurityType, wifiPassword); err != nil {
			return errors.Annotate(err, "try to connect to wifi network").Err()
		}
		return nil
	}, "try to connect to wifi network")
	if err != nil {
		return errors.Annotate(err, "connect wifi network").Err()
	}
	log.Debugf("device %q is connected to wifi network", serialNumber)
	return nil
}
