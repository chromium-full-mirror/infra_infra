// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"os"
	"strconv"
	"strings"

	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/components/mh"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

var (
	// Path to ADB to use when run adb commands.
	adbPath = "adb"
	// Store cache of decision to use local adb or not.
	localUseDecision = ""
)

const (
	// Decision answers.
	decisionYes = "yes"
	decisionNo  = "no"
)

// ADBPath provides path to ADB
func ADBPath(ctx context.Context) string {
	return adbPath
}

// UseLocal decide if local ADB need to be used.
func UseLocal(ctx context.Context) bool {
	if localUseDecision == "" {
		if ap := pathFromEnv(); ap != "" {
			localUseDecision = decisionYes
		} else if ctr.IsUp(ctx) {
			localUseDecision = decisionNo
		} else if mh.IsMH() {
			localUseDecision = decisionYes
		}
	}
	return localUseDecision == decisionYes
}

// Port provides port number for ADB connect.
func Port(ctx context.Context) int {
	val := strings.TrimSpace(os.Getenv("ADB_CONNECTION_PORT"))
	if val != "" {
		if port, err := strconv.Atoi(val); err != nil {
			log.Infof(ctx, "Fail to parse ADB port from environment: %q, will use default port 5555", val)
		} else {
			return port
		}
	}
	// Default ADB port for connection.
	return 5555
}

func init() {
	if mh.IsMH() {
		adbPath = mh.ADBPath
	}
	if ap := pathFromEnv(); ap != "" {
		adbPath = ap
	}
}

func pathFromEnv() string {
	return strings.TrimSpace(os.Getenv("ADB_PATH"))
}
