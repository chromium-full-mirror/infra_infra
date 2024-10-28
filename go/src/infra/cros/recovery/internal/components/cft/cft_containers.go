// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cft contains methods to work with CFT containers.
package cft

import (
	"infra/cros/recovery/tlw"
)

// NetworkName generates predicable name for custom Docker network.
func NetworkName(dut *tlw.Dut) string {
	// Only use one network for all containers.
	return "adb-network"
}

// ADBName generates predicable container name for ADB container.
func ADBName(dut *tlw.Dut) string {
	return "adb-" + dut.Name
}

// ServoNexusName generates predicable container name for servo-nexux container.
func ServoNexusName(dut *tlw.Dut) string {
	return "servo-nexus-" + dut.Name
}

// CrosDUTName generates predicable container name for cros-dut container.
func CrosDUTName(dut *tlw.Dut) string {
	return "cros-dut-" + dut.Name
}
