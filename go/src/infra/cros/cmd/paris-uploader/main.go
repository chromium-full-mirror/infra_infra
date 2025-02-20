// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/cmd/paris-uploader/internal/cmds"
)

// GetApplication returns the paris-uploader application.
func getApplication() *cli.Application {
	return &cli.Application{
		Name: "paris-uploader",
		Title: `Paris Uploader

Upload a directory according to Paris conventions`,
		Context: func(ctx context.Context) context.Context {
			return ctx
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			cmds.UploadCmd,
		},
	}
}

// Main is the entrypoint to paris-uploader.
func main() {
	os.Exit(subcommands.Run(getApplication(), nil))
}
