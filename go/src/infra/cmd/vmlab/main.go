// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging/gologger"

	"go.chromium.org/infra/cmd/vmlab/internal/cmd"
)

var application = &cli.Application{
	Name:  "vmlab",
	Title: ``,
	Context: func(ctx context.Context) context.Context {
		return gologger.StdConfig.Use(ctx)
	},
	Commands: []*subcommands.Command{
		subcommands.CmdHelp,
		cmd.LeaseCmd,
		cmd.ReleaseCmd,
		cmd.CleanupInstancesCmd,
		cmd.ImageCmd,
		cmd.CleanImagesCmd,
	},
}

func main() {
	os.Exit(subcommands.Run(application, nil))
}
