// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging/gologger"
)

// application returns the consoleadmin command line application.
func application() *cli.Application {
	return &cli.Application{
		Name:  "console admin",
		Title: "console admin command line tool",
		Context: func(ctx context.Context) context.Context {
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
		},
	}
}

func main() {
	os.Exit(subcommands.Run(application(), nil))
}
