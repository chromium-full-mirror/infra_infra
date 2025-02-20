// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dns

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/dns"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type readContentsFunc func(context.Context, executor.IExecCommander) (string, error)

// GetDNSCmd is the command to print the current DNS entries for local satlab network
var GetDNSCmd = &subcommands.Command{
	UsageLine: "dns",
	ShortDesc: "get DNS entries for local satlab network",
	LongDesc:  "Get DNS entries for local satlab network",
	CommandRun: func() subcommands.CommandRun {
		c := &getDNSRun{}
		return c
	},
}

// getDNSRun struct contains the arguments needed to run GetDNSCmd
type getDNSRun struct {
	subcommands.CommandRunBase
}

// Run is what is called when a user inputs the getDNSRun command
func (c *getDNSRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun contains business logic
func (c *getDNSRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	return c.runCmdInjected(ctx, os.Stdout, dns.ReadContents)
}

func (c *getDNSRun) runCmdInjected(ctx context.Context, w io.Writer, readContents readContentsFunc) error {
	contents, err := readContents(ctx, &executor.ExecCommander{})
	if err != nil {
		return errors.Annotate(err, "read contents").Err()
	}

	fmt.Fprintf(w, "Satlab internal DNS:\n%s\n", contents)
	return nil
}
