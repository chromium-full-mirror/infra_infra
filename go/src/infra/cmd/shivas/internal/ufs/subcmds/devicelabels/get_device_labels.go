// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabels

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// GetDeviceLabelsCmd subcommand: Get Swarming dimensions for a DUT.
var GetDeviceLabelsCmd = &subcommands.Command{
	UsageLine: "devicelabels ...",
	ShortDesc: "Get device labels",
	LongDesc: `Get device labels.

Example:

shivas get devicelabels machineLSEs/lse-1 schedulingunits/su-1

shivas get devicelabels vms/vm-1

shivas get devicelabels -n 10

Get the device labels and prints the output.
Expects fully qualified names based on the underlying entity.`,
	CommandRun: func() subcommands.CommandRun {
		c := &getDeviceLabelsRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.outputFlags.Register(&c.Flags)
		c.Flags.IntVar(&c.pageSize, "n", 0, cmdhelp.ListPageSizeDesc)
		c.Flags.Var(flag.StringSlice(&c.resourcetypes), "resourcetype", "Name(s) of a resource type to filter by. Can be specified multiple times."+cmdhelp.ResourceTypeFilterHelpText)
		return c
	},
}

type getDeviceLabelsRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	outputFlags site.OutputFlags

	// Filters
	resourcetypes []string

	pageSize int
	keysOnly bool
}

func (c *getDeviceLabelsRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getDeviceLabelsRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ns, err := c.envFlags.Namespace(nil, "")
	if err != nil {
		return err
	}
	ctx = utils.SetupContext(ctx, ns)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	emit := !utils.NoEmitMode(c.outputFlags.NoEmit())
	full := utils.FullMode(c.outputFlags.Full())
	var res []proto.Message
	if len(args) > 0 {
		res = utils.ConcurrentGet(ctx, ic, args, c.getSingle)
	} else {
		res, err = utils.BatchList(ctx, ic, listDeviceLabels, c.formatFilters(), c.pageSize, c.keysOnly, full, nil)
	}
	if err != nil {
		return err
	}
	return utils.PrintEntities(ctx, ic, res, utils.PrintDeviceLabelsJSON, printDeviceLabelsFull, printDeviceLabelsNormal,
		c.outputFlags.JSON(), emit, full, c.outputFlags.Tsv(), c.keysOnly)
}

func (c *getDeviceLabelsRun) formatFilters() []string {
	filters := make([]string, 0)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.ResourceTypeFilterName, c.resourcetypes)...)
	return filters
}

func (c *getDeviceLabelsRun) getSingle(ctx context.Context, ic ufsAPI.FleetClient, name string) (proto.Message, error) {
	return ic.GetDeviceLabels(ctx, &ufsAPI.GetDeviceLabelsRequest{
		Hostname: ufsUtil.AddPrefix(ufsUtil.DeviceLabelsCollection, name),
	})
}

func listDeviceLabels(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]proto.Message, string, error) {
	req := &ufsAPI.ListDeviceLabelsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
	}
	res, err := ic.ListDeviceLabels(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]proto.Message, len(res.GetLabels()))
	for i, m := range res.GetLabels() {
		protos[i] = m
	}
	return protos, res.GetNextPageToken(), nil
}

func printDeviceLabelsFull(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message, tsv bool) error {
	return printDeviceLabelsNormal(msgs, tsv, false)
}

func printDeviceLabelsNormal(entities []proto.Message, tsv, keysOnly bool) error {
	if len(entities) == 0 {
		return nil
	}
	if tsv {
		utils.PrintTSVDeviceLabels(entities, keysOnly)
		return nil
	}
	utils.PrintTableTitle(utils.DeviceLabelsTitle, tsv, keysOnly)
	utils.PrintDeviceLabels(entities, keysOnly)
	return nil
}
