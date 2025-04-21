// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/cli"
)

func main() {
	opt, err := cli.ParseInputs()
	if err != nil {
		fmt.Printf("unable to parse inputs: %s\n", err)
		os.Exit(2)
	}
	err = opt.Run()
	if err != nil {
		fmt.Printf("ash-chrome-provision failed: %v\n", err)
		os.Exit(1)
	}
}
