// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"go.chromium.org/chromiumos/test/provision/v2/cros-fw-provision/cli"
)

func main() {
	opt, err := cli.ParseInputs()
	if err != nil {
		fmt.Printf("unable to parse inputs: %s\n", err)
		os.Exit(2)
	}
	err = opt.Run()
	if err != nil {
		fmt.Printf("cros-fw-provision failed: %v\n", err)
		os.Exit(1)
	}
}
