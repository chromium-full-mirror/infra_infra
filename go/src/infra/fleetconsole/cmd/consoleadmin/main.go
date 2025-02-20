// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/infra/fleetconsole/cmd/consoleadmin/clilib"
)

func main() {
	os.Exit(subcommands.Run(clilib.Application(), nil))
}
