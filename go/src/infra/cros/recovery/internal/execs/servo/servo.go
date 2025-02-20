// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/servo/topology"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// IsContainerizedServoHost checks if the servohost is using servod container.
func IsContainerizedServoHost(ctx context.Context, servoHost *tlw.ServoHost) bool {
	if servoHost == nil || servoHost.ContainerName == "" {
		return false
	}
	log.Debugf(ctx, "Servo uses servod container with the name: %s", servoHost.ContainerName)
	return true
}

// ResetUsbkeyAuthorized resets usb-key detected under labstation.
//
// This is work around to address issue found for servo_v4p1.
// TODO(b/197647872): Remove as soon issue will be addressed.
func ResetUsbkeyAuthorized(ctx context.Context, run execs.Runner, servoSerial string, servoType string) error {
	if !strings.HasPrefix(servoSerial, "SERVOV4P1") {
		log.Debugf(ctx, "Authorized flag reset only for servo_v4p1.")
		return nil
	}
	log.Debugf(ctx, "Start reset authorized flag for servo_v4p1.")
	rootServoPath, err := topology.GetRootServoPath(ctx, run, servoSerial)
	if err != nil {
		return errors.Annotate(err, "reset usbkey authorized").Err()
	}
	pathDir := filepath.Dir(rootServoPath)
	pathTail := filepath.Base(rootServoPath)
	// For usb-path path looks like '/sys/bus/usb/devices/1-4.2.5' we need
	// remove last number, to make it as path to the servo-hub.
	pathTailElements := strings.Split(pathTail, ".")
	pathTail = strings.Join(pathTailElements[:(len(pathTailElements)-1)], ".")
	// Replace the first number '1' to '2 (usb3). Finally it will look like
	// '/sys/bus/usb/devices/2-4.2'
	pathTail = strings.Replace(pathTail, "1-", "2-", 1)
	const authorizedFlagName = "authorized"
	authorizedPath := filepath.Join(pathDir, pathTail, authorizedFlagName)
	log.Infof(ctx, "Authorized flag file path: %s", authorizedPath)
	// Setting flag to 0.
	if _, err := run(ctx, 30*time.Second, fmt.Sprintf("echo 0 > %s", authorizedPath)); err != nil {
		log.Debugf(ctx, `Attempt to reset %q flag to 0 for servo-hub failed`, authorizedFlagName)
		return errors.Annotate(err, "reset usbkey authorized: set to 0").Err()
	}
	time.Sleep(time.Second)
	// Setting flag to 1.
	if _, err := run(ctx, 30*time.Second, fmt.Sprintf("echo 1 > %s", authorizedPath)); err != nil {
		log.Debugf(ctx, `Attempt to reset %q flag to 1 for servo-hub failed`, authorizedFlagName)
		return errors.Annotate(err, "reset usbkey authorized: set to 1").Err()
	}
	time.Sleep(time.Second)
	log.Infof(ctx, "Attempt to reset %q succeed", authorizedFlagName)
	return nil
}
