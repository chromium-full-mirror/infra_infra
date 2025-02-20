// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"go.chromium.org/infra/cros/cmd/ctpv2/cli"
)

func main() {
	opt, err := cli.ParseInputs()
	if err != nil {
		fmt.Printf("unable to parse inputs: %s\n", err)
		os.Exit(2)
	}
	_ = opt.Run()
}
