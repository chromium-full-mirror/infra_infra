// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	provcli "go.chromium.org/infra/cros/cmd/provision/foil-provision/cli"
)

func main() {
	opt, err := provcli.ParseInputs()
	if err != nil {
		fmt.Printf("unable to parse inputs: %s", err)
		os.Exit(2)
	}
	opt.Run()
}
