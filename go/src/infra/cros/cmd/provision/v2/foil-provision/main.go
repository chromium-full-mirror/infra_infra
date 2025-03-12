// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/cli"
)

func main() {
	opt, err := cli.ParseInputs()
	if err != nil {
		fmt.Printf("unable to parse inputs: %s", err)
		os.Exit(2)
	}
	opt.Run()
}
