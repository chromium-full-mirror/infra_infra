// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/internal/site"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	"go.chromium.org/infra/unifiedfleet/api/ufsclients"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// GetLabelsUFSCommand pings ufs, via the Console UI server by default.
var GetLabelsUFSCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "get-labels-ufs [options...]",
	ShortDesc: "Get labels UFS shows the ufs labels of a dut device",
	LongDesc:  "Get labels UFS shows the ufs labels of a dut device",
	CommandRun: func() subcommands.CommandRun {
		c := &getLabelsUFSCommand{}
		c.Init()
		c.Flags.StringVar(&c.dutName, "dut-name", "", `The dut_name of the device`)
		return c
	},
}

type getLabelsUFSCommand struct {
	site.Subcommand
	dutName string
}

// Run is the main entrypoint to the ping.
func (c *getLabelsUFSCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *getLabelsUFSCommand) innerRun(ctx context.Context, a subcommands.Application, _ []string, _ subcommands.Env) error {
	if c.dutName == "" {
		return errors.New("GetLabelsUFS missing dut name")
	}

	ctx = ufsclient.SetUfsNameSpace(ctx, ufsUtil.OSNamespace)

	ufsClient, err := ufsclients.NewUFSClientFromCLI(ctx, ufsclient.UfsProdURL, &c.AuthFlags, nil)
	if err != nil {
		return err
	}

	resp, err := ufsClient.GetDeviceLabels(ctx, &ufsAPI.GetDeviceLabelsRequest{
		Hostname: fmt.Sprintf("devicelabels/machineLSEs/%s", c.dutName),
	})
	if err != nil {
		return errors.Annotate(err, "GetLabelsUFS").Err()
	}
	showProto(a.GetOut(), resp)
	return nil
}
