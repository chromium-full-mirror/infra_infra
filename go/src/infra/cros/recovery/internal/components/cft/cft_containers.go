// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cft contains methods to work with CFT containers.
package cft

import (
	"infra/cros/internal/env"
	"infra/cros/recovery/tlw"
)

// Container names.
const (
	ADBBase       = "adb-base"
	ServoNexux    = "servo-nexus"
	CrosDUT       = "cros-dut"
	FoilProvision = "foil-provision"
)

// NetworkName generates predicable name for custom Docker network.
func NetworkName(dut *tlw.Dut) string {
	if env.IsCloudBot() {
		// CloubBot requires predicted name for docker network.
		// CloubBot supports a single testbed so no parallel task execution.
		return "adb-network"
	}
	// Create unique network to avoid issues when parallel task removed network.
	return "paris-" + dut.Name
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

// FoilProvisionName generates predicable container name for foil-provision container.
func FoilProvisionName(dut *tlw.Dut) string {
	return "foil-provision-" + dut.Name
}
