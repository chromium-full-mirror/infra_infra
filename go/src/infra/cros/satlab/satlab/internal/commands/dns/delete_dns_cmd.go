// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dns

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/dns"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// DeleteDNSCmd is the command to delete a hostname from the hostsfile of the DNS container
var DeleteDNSCmd = &subcommands.Command{
	UsageLine: "dns -host <hostname>",
	ShortDesc: "delete DNS entry in local satlab network",
	LongDesc:  "Delete DNS entry in local satlab network",
	CommandRun: func() subcommands.CommandRun {
		c := &deleteDNSRun{}
		c.Flags.StringVar(&c.host, "host", "", "hostname to delete")
		return c
	},
}

// deleteDNSRun struct contains the arguments needed to run DeleteDNSCmd
type deleteDNSRun struct {
	subcommands.CommandRunBase
	host string
}

// Run is what is called when a user inputs the deleteDNSRun command
func (c *deleteDNSRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun calls underlying business logic with appropriate functions and interfaces injected
// extra abstraction layer allows us to test `runCmdInjected` with fake implementations
func (c *deleteDNSRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	return c.runCmdInjected(ctx, args, satlabcommands.GetDockerHostBoxIdentifier)
}

// runCmdInjected executes business logic
func (c *deleteDNSRun) runCmdInjected(ctx context.Context, args []string, dhbIDFunc DockerHostBoxIdentifierGetter) error {
	satlabID, err := dhbIDFunc(ctx, &executor.ExecCommander{})
	if err != nil {
		return err
	}

	err = c.validate(args, satlabID)
	if err != nil {
		return err
	}

	_, err = dns.DeleteRecord(dns.EnsureRecords, func() (string, error) {
		return dns.ReadContents(ctx, &executor.ExecCommander{})
	}, c.host)
	return err
}

// validate checks for required and unexpected args + formats hostname
func (c *deleteDNSRun) validate(args []string, satlabId string) error {
	if c.host == "" {
		return errors.Reason("host must be specified").Err()
	}
	if len(args) > 0 {
		return errors.Reason("unrecognized positional argument(s): %+v", args).Err()
	}

	c.host = site.MaybePrepend(site.Satlab, satlabId, c.host)
	return nil
}
