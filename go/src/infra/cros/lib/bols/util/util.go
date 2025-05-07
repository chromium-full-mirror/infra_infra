// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package util contains general utilities for BOLS.
package util

import (
	"fmt"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
)

// CheckFutilityParams makes sure parameters in futility request is valid.
func CheckFutilityParams(req *bols.RunFutilityRequest) error {
	port := req.GetStationId().GetServodPort()
	hasPortParam := false
	// Make sure that the port argument is valid.
	for _, a := range req.GetParams() {
		if hasPortParam {
			// Check if the port argument is valid.
			num, err := strconv.Atoi(a)
			if err != nil {
				return fmt.Errorf("invalid port number %s: %w", a, err)
			}
			if num != int(port) {
				return fmt.Errorf("port number %s does not match port %d in station id: %w", a, port, err)
			}
			break
		}
		if a == "--servo_port" {
			hasPortParam = true
		}
	}
	return nil
}

// CheckFlashECParams makes sure parameters in flash ec request is valid.
func CheckFlashECParams(req *bols.RunFlashECRequest) error {
	port := req.GetStationId().GetServodPort()
	// Make sure that the port argument is valid.
	for _, a := range req.GetParams() {
		if strings.HasPrefix(a, "--port=") {
			// Check if the port argument is valid.
			num, err := strconv.Atoi(a[len("--port="):])
			if err != nil {
				return fmt.Errorf("invalid port number %s: %w", a, err)
			}
			if num != int(port) {
				return fmt.Errorf("port number %s does not match port %d in station id: %w", a, port, err)
			}
			break
		}
	}
	return nil
}
