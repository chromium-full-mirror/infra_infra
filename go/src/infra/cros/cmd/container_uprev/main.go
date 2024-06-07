// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"infra/cros/cmd/container_uprev/cli"
)

func main() {
	opt, err := cli.ParseInputs()
	if err != nil {
		fmt.Printf("unable to parse inputs: %s\n", err)
		os.Exit(2)
	}
	_ = opt.Run()
}
