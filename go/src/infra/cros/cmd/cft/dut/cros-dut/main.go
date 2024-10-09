// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the cros-dut for interfacing with the DUT.
package main

import (
	"os"

	"go.chromium.org/chromiumos/test/dut/cmd/cros-dut/cli"
)

func main() {
	os.Exit(cli.MainInternal())
}
