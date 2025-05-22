// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cft contains methods to work with CFT containers.
package cft

import (
	"go.chromium.org/infra/cros/recovery/tlw"
)

// Container names.
const (
	ADBBase       = "adb-base"
	ServoNexux    = "servo-nexus"
	CrosDUT       = "cros-dut"
	FoilProvision = "foil-provision"
	Passport      = "cros-passport"
)

// NetworkName generates predicable name for custom Docker network.
func NetworkName(_ *tlw.Dut) string {
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

// FoilProvisionName generates predicable container name for foil-provision container.
func FoilProvisionName(dut *tlw.Dut) string {
	return "foil-provision-" + dut.Name
}

// PassportName generates predicable container name for cros-passport container.
func PassportName(dut *tlw.Dut) string {
	return "cros-passport-" + dut.Name
}

// PassportSwitchName generates the predicable name for cros-passport switch service.
func PassportSwitchName(dut *tlw.Dut) string {
	return "cros-passport_switch-" + dut.Name
}
