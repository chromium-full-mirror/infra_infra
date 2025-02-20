// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the cros-servod for starting/stopping servod daemon
// and sending commands to it to control and test DUTs via servo hardware by
// simulating user actions such as power on/off, flashing of firmware/OS,
// screen close, etc.
package main

import (
	"os"

	"go.chromium.org/infra/cros/cmd/cft/dut/cros-servod/cli"
)

func main() {
	os.Exit(cli.MainInternal())
}
