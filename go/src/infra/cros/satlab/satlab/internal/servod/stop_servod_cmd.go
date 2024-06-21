// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package servod

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"infra/cmd/shivas/utils"
	"infra/cros/recovery/docker"
	"infra/cros/satlab/common/satlabcommands"
	"infra/cros/satlab/common/services/ufs"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/executor"
)

// StartServodCmd is the command that will start a servod container
var StopServodCmd = &subcommands.Command{
	UsageLine: "stop -host <hostname> [options ...]",
	ShortDesc: "starts servod container",
	LongDesc:  "Starts servod container",
	CommandRun: func() subcommands.CommandRun {
		c := &stopServodRun{}
		c.commonFlags.Register(&c.Flags)
		c.envFlags.Register(&c.Flags)
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.Flags.StringVar(&c.host, "host", "", "Hostname of DUT")
		c.Flags.StringVar(&c.servodContainerName, "servod-container-name", "", "Optional: name of servod container; will be fetched from UFS if not provided")
		return c
	},
}

// stopServodRun struct contains the arguments for the servod command
type stopServodRun struct {
	subcommands.CommandRunBase
	commonFlags         site.CommonFlags
	envFlags            site.EnvFlags
	authFlags           authcli.Flags
	host                string
	servodContainerName string
}

// Run is what is called when a user inputs the stopServodRun command
func (c *stopServodRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun contains the actual logic of the stopServodRun command
func (c *stopServodRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, c.envFlags.GetNamespace())
	dhbSatlabID := c.commonFlags.SatlabID
	if dhbSatlabID == "" {
		var err error
		dhbSatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, &executor.ExecCommander{})
		if err != nil {
			return err
		}
	}
	if err := c.validate(dhbSatlabID, args); err != nil {
		return err
	}

	ufs, err := ufs.NewUFSClient(ctx, c.envFlags.GetUFSService(), &c.authFlags)
	if err != nil {
		return err
	}
	d, err := docker.NewClient(ctx)
	if err != nil {
		return err
	}

	return c.runCmdWithClients(ctx, d, ufs)
}

// runCmdWithClients uses given docker, ufs clients to execute business logic
func (c *stopServodRun) runCmdWithClients(ctx context.Context, d DockerClient, ufsClient ufs.UFSClient) error {
	// if user specifies container name, trust them, otherwise go to UFS to check expected container name
	containerName := c.servodContainerName
	if containerName == "" {
		ufsMetadata, err := fetchMetadataFromUFS(ctx, ufsClient, c.host, &c.authFlags)
		if err != nil {
			return err
		}
		containerName = ufsMetadata.servodContainerName
	}

	err := d.Remove(ctx, containerName, true)
	return err
}

// validate validates input arguments
// We primarily care that a) host is not empty and b) host is in the right format (satlab-<dhb_id>-<host>)
func (c *stopServodRun) validate(dhbSatlabID string, positionalArgs []string) error {
	// ensures we did not receive positional args
	if len(positionalArgs) > 0 {
		return errors.Reason("Got unexpected positional args, for usage see: satlab servo help stop").Err()
	}

	// Ensures the host or required field is provided.
	if c.host == "" && c.servodContainerName == "" {
		return errors.Reason(fmt.Sprintf("-host <hostname> is required")).Err()
	}

	c.host = site.GetFullyQualifiedHostname(c.commonFlags.SatlabID, dhbSatlabID, site.Satlab, c.host)

	return nil
}
