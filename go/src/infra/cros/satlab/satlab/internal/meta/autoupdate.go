// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package meta contains functionality around management of the Satlab CLI
// binary itself.
package meta

import (
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"infra/cros/satlab/common/utils/misc"
)

// SkipAutoUpdateEnvVar is the env var we look at to determine if we should not
// attempt to autoupdate.
var SkipAutoUpdateEnvVar = "SKIP_AUTO_UPDATE"

func shouldUpdate() bool {
	return !misc.BoolVal(os.Getenv(SkipAutoUpdateEnvVar))
}

// UpdateThenRun performs an upgrade of CLI tools (if applicable) and then
// executes the user's command.
func UpdateThenRun(app *cli.Application) int {
	if shouldUpdate() {
		_ = subcommands.Run(app, []string{"upgrade", "-silent"})
	}
	return subcommands.Run(app, nil)
}
