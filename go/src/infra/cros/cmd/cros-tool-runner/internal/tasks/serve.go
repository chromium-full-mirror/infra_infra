// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/server"
)

type runServeCmd struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	port      int
	exportTo  string
}

func Serve(authOpts auth.Options) *subcommands.Command {
	const serveDesc = `serve,

Tool used to start cros-tool-runner v2 services. When --export-metadata is
used, service metadata will be exported into the specified folder following
the same convention as http://go/cft-port-discovery

Example:
cros-tool-runner serve
cros-tool-runner serve --port 0 --export-metadata=/tmp
	`

	c := &runServeCmd{}
	return &subcommands.Command{
		UsageLine: "serve",
		ShortDesc: "serve starts CTRv2 services",
		LongDesc:  serveDesc,
		CommandRun: func() subcommands.CommandRun {
			c.authFlags.Register(&c.Flags, authOpts)
			c.Flags.IntVar(&c.port, "port", 8082, "port number server listens to")
			c.Flags.StringVar(&c.exportTo, "export-metadata", "", "folder path to export CTRv2 server metadata into")
			return c
		},
	}
}

// Run executes the tool.
func (c *runServeCmd) Run(subcommands.Application, []string, subcommands.Env) int {
	return server.StartServer(c.port, c.exportTo)
}
