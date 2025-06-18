// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drac

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
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// GetDracCmd get drac by given name.
var GetDracCmd = &subcommands.Command{
	UsageLine: "drac ...",
	ShortDesc: "Get drac details by filters",
	LongDesc: `Get drac details by filters.

Example:

shivas get drac {name1} {name2}

shivas get drac -zone atl97 -rack rack1 -rack rack2

Gets the drac and prints the output in the user-specified format.`,
	CommandRun: func() subcommands.CommandRun {
		c := &getDrac{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.outputFlags.Register(&c.Flags)

		c.Flags.IntVar(&c.pageSize, "n", 0, cmdhelp.ListPageSizeDesc)
		c.Flags.BoolVar(&c.keysOnly, "keys", false, cmdhelp.KeysOnlyText)

		c.Flags.Var(flag.StringSlice(&c.zones), "zone", "Name(s) of a zone to filter by. Can be specified multiple times."+cmdhelp.ZoneFilterHelpText)
		c.Flags.Var(flag.StringSlice(&c.racks), "rack", "Name(s) of a rack to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.machines), "machine", "Name(s) of a machine to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.switches), "switch", "Name(s) of a switch to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.switchPorts), "switch-port", "Name(s) of a switch port to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.macs), "mac", "Name(s) of a mac to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of a tag to filter by. Can be specified multiple times.")
		return c
	},
}

type getDrac struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	// Filters
	zones       []string
	racks       []string
	machines    []string
	switches    []string
	switchPorts []string
	macs        []string
	tags        []string

	pageSize int
	keysOnly bool
}

func (c *getDrac) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getDrac) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
		res, err = utils.BatchList(ctx, ic, listDracs, c.formatFilters(), c.pageSize, c.keysOnly, full, nil)
	}
	if err != nil {
		return err
	}
	return utils.PrintEntities(ctx, ic, res, utils.PrintDracsJSON, printDracFull, printDracNormal,
		c.outputFlags.JSON(), emit, full, c.outputFlags.Tsv(), c.keysOnly)
}

func (c *getDrac) formatFilters() []string {
	filters := make([]string, 0)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.ZoneFilterName, c.zones)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.RackFilterName, c.racks)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.MachineFilterName, c.machines)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.SwitchFilterName, c.switches)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.SwitchPortFilterName, c.switchPorts)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.MacAddressFilterName, c.macs)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.TagFilterName, c.tags)...)
	return filters
}

func (c *getDrac) getSingle(ctx context.Context, ic ufsAPI.FleetClient, name string) (proto.Message, error) {
	res, err := ic.GetDrac(ctx, &ufsAPI.GetDracRequest{
		Name: ufsUtil.AddPrefix(ufsUtil.DracCollection, name),
	})
	if err == nil {
		setNetwork(ctx, ic, []proto.Message{res})
	}
	return res, err
}

func listDracs(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]proto.Message, string, error) {
	req := &ufsAPI.ListDracsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
	}
	res, err := ic.ListDracs(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]proto.Message, len(res.GetDracs()))
	for i, m := range res.GetDracs() {
		protos[i] = m
	}
	setNetwork(ctx, ic, protos)
	return protos, res.GetNextPageToken(), nil
}

func setNetwork(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message) []*ufspb.Drac {
	entities := make([]*ufspb.Drac, len(msgs))
	names := make([]string, len(msgs))
	entityMap := make(map[string]*ufspb.Drac, len(msgs))
	for i, r := range msgs {
		if drac := r.(*ufspb.Drac); drac != nil {
			entities[i] = drac
			entities[i].Name = ufsUtil.RemovePrefix(drac.Name)
			names[i] = drac.GetName()
			entityMap[drac.GetName()] = drac
		}
	}
	if len(entityMap) == 0 {
		return entities
	}

	// Some DRACs don't have DHCP entries. If we try to get 100 entries at once,
	// the whole request might fail if even one entry is missing. So, if that
	// happens, we'll request each entry individually instead.
	const batchSize = 100
	do := func(batchNames []string) error {
		// Ignore errors: not all dracs has associated DHCP record.
		res, err := ic.BatchGetDHCPConfigs(ctx, &ufsAPI.BatchGetDHCPConfigsRequest{
			Names: batchNames,
		})
		if err != nil {
			return err
		}
		for _, d := range res.GetDhcpConfigs() {
			if drac, ok := entityMap[d.GetHostname()]; ok {
				drac.Ip = d.GetIp()
				drac.Vlan = d.GetVlan()
			}
		}
		return nil
	}
	for i := 0; i < len(entities); i += batchSize {
		end := i + batchSize
		if end > len(entities) {
			end = len(entities)
		}
		batch := names[i:end]
		if err := do(batch); err != nil {
			// failed as batch, not try each separate.
			for _, name := range batch {
				do([]string{name})
			}
		}
	}
	return entities
}

func printDracFull(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message, tsv bool) error {
	dracs := setNetwork(ctx, ic, msgs)
	if tsv {
		for _, d := range dracs {
			utils.PrintTSVDracFull(d)
		}
		return nil
	}
	utils.PrintTitle(utils.DracFullTitle)
	utils.PrintDracFull(dracs)
	return nil
}

func printDracNormal(msgs []proto.Message, tsv, keysOnly bool) error {
	if tsv {
		utils.PrintTSVDracs(msgs, keysOnly)
		return nil
	}
	utils.PrintTableTitle(utils.DracTitle, tsv, keysOnly)
	utils.PrintDracs(msgs, keysOnly)
	return nil
}
