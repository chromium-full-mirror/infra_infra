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
	return "network-" + dut.Name
}

// ADBName generates predicable container name for ADB container.
func ADBName(dut *tlw.Dut) string {
	return "adb-" + dut.Name
}
