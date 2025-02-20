// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devboard

import (
	"context"
	"fmt"

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

// GetDevboardLSECmd gets the devboard LSE for a given name.
var GetDevboardLSECmd = &subcommands.Command{
	UsageLine:  "devboard-lse ...",
	ShortDesc:  "Get devboard LSE details by filters",
	LongDesc:   cmdhelp.GetDevboardLSELongDesc,
	CommandRun: getDevboardLSECommandRun,
}

func getDevboardLSECommandRun() subcommands.CommandRun {
	c := &getDevboardLSE{}
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.outputFlags.Register(&c.Flags)

	c.Flags.IntVar(&c.pageSize, "n", 0, cmdhelp.ListPageSizeDesc)
	c.Flags.BoolVar(&c.keysOnly, "keys", false, cmdhelp.KeysOnlyText)

	c.Flags.Var(flag.StringSlice(&c.zones), "zone", "Name(s) of a zone to filter by. Can be specified multiple times."+cmdhelp.ZoneFilterHelpText)
	c.Flags.Var(flag.StringSlice(&c.racks), "rack", "Name(s) of a rack to filter by. Can be specified multiple times.")
	c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of a tag to filter by. Can be specified multiple times.")
	return c
}

type getDevboardLSE struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	// Filters
	zones []string
	racks []string
	tags  []string

	pageSize int
	keysOnly bool
}

func (c *getDevboardLSE) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getDevboardLSE) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UnifiedFleet service %s\n", e.UnifiedFleetService)
	}
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
		res, err = utils.BatchList(ctx, ic, listDevboardLSEs, c.formatFilters(), c.pageSize, c.keysOnly, full, nil)
	}
	if err != nil {
		return err
	}
	return utils.PrintEntities(ctx, ic, res, utils.PrintMachineLSEsJSON, printDevboardLSEFull, printDevboardLSENormal,
		c.outputFlags.JSON(), emit, full, c.outputFlags.Tsv(), c.keysOnly)
}

func (c *getDevboardLSE) getSingle(ctx context.Context, ic ufsAPI.FleetClient, name string) (proto.Message, error) {
	return ic.GetMachineLSE(ctx, &ufsAPI.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, name),
	})
}

func (c *getDevboardLSE) formatFilters() []string {
	filters := make([]string, 0)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.ZoneFilterName, c.zones)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.RackFilterName, c.racks)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.TagFilterName, c.tags)...)
	return filters
}

func printDevboardLSEFull(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message, tsv bool) error {
	return printDevboardLSENormal(msgs, tsv, false)
}

func printDevboardLSENormal(entities []proto.Message, tsv, keysOnly bool) error {
	if len(entities) == 0 {
		return nil
	}
	if tsv {
		utils.PrintTSVMachineLSEs(entities, keysOnly)
		return nil
	}
	utils.PrintTableTitle(utils.DevboardLSETitle, tsv, keysOnly)
	utils.PrintMachineLSEs(entities, keysOnly)
	return nil
}

func listDevboardLSEs(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]proto.Message, string, error) {
	req := &ufsAPI.ListMachineLSEsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
		Full:      full,
	}
	res, err := ic.ListMachineLSEs(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]proto.Message, 0)
	for _, m := range res.GetMachineLSEs() {
		if m.GetChromeosMachineLse().GetDeviceLse().GetDevboard() != nil {
			protos = append(protos, m)
		}
	}
	return protos, res.GetNextPageToken(), nil
}

func (c *getDevboardLSE) validateArgs() error {
	return nil
}
