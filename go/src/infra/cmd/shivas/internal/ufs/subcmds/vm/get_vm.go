// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// GetVMCmd get VM by given name.
var GetVMCmd = &subcommands.Command{
	UsageLine: "vm ...",
	ShortDesc: "Get VM details by filters",
	LongDesc: `Get VM details by filters.

Example:

shivas get vm {name1} {name2}

shivas get vm -zone atl97 -vlan browser:1

Gets the vm and prints the output in the specified format.`,
	CommandRun: func() subcommands.CommandRun {
		c := &getVM{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.outputFlags.Register(&c.Flags)

		c.Flags.IntVar(&c.pageSize, "n", 0, cmdhelp.ListPageSizeDesc)
		c.Flags.BoolVar(&c.keysOnly, "keys", false, cmdhelp.KeysOnlyText)

		c.Flags.Var(flag.StringSlice(&c.zones), "zone", "Name(s) of a zone to filter by. Can be specified multiple times."+cmdhelp.ZoneFilterHelpText)
		c.Flags.Var(flag.StringSlice(&c.vlans), "vlan", "Name(s) of a vlan to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.hosts), "host", "Name(s) of a host to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.oses), "os", "Name(s) of an os to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of a tag to filter by. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.states), "state", "Name(s) of a state to filter by. Can be specified multiple times."+cmdhelp.StateFilterHelpText)
		c.Flags.Var(flag.StringSlice(&c.cpuCores), "cpu-cores", "Number of cpu cores. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.memory), "memory", "Amount of memory in bytes assigned. Can be specified multiple times. Assumed GB if no units specified. "+cmdhelp.ByteUnitsAcceptedText)
		c.Flags.Var(flag.StringSlice(&c.storage), "storage", "Disk storage capacity in bytes assigned. Can be specified multiple times. Assumed GB if no units specified. "+cmdhelp.ByteUnitsAcceptedText)
		c.Flags.StringVar(&c.byteFormat, "byte-format", "GB", "Output format of memory and storage fields. Ignored with -json flag. "+cmdhelp.ByteUnitsAcceptedText)
		return c
	},
}

type getVM struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	outputFlags site.OutputFlags

	// Filters
	zones    []string
	vlans    []string
	hosts    []string
	oses     []string
	tags     []string
	states   []string
	cpuCores []string
	memory   []string
	storage  []string

	byteFormat string

	pageSize int
	keysOnly bool
}

func (c *getVM) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getVM) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
	if !c.outputFlags.JSON() {
		if err = utils.VerifyByteUnit(c.byteFormat); err != nil {
			return err
		}
	}
	var res []proto.Message
	if len(args) > 0 {
		res = utils.ConcurrentGet(ctx, ic, args, c.getSingle)
	} else {
		var filters, err = c.formatFilters()
		if err != nil {
			return err
		}
		res, err = utils.BatchList(ctx, ic, ListVMs, filters, c.pageSize, c.keysOnly, full, nil)
	}
	if err != nil {
		return err
	}
	return utils.PrintEntities(ctx, ic, res, utils.PrintVMsJSON, c.printVMFull, c.printVMNormal,
		c.outputFlags.JSON(), emit, full, c.outputFlags.Tsv(), c.keysOnly)
}

func (c *getVM) formatFilters() (filters []string, err error) {
	c.memory, err = utils.ConvertFiltersToBytes(c.memory)
	if err != nil {
		return nil, err
	}
	c.storage, err = utils.ConvertFiltersToBytes(c.storage)
	if err != nil {
		return nil, err
	}

	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.ZoneFilterName, c.zones)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.StateFilterName, c.states)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.HostFilterName, c.hosts)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.VlanFilterName, c.vlans)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.OSFilterName, c.oses)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.TagFilterName, c.tags)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.CpuCoresFilterName, c.cpuCores)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.MemoryFilterName, c.memory)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.StorageFilterName, c.storage)...)
	return filters, nil
}

func (c *getVM) getSingle(ctx context.Context, ic ufsAPI.FleetClient, name string) (proto.Message, error) {
	return ic.GetVM(ctx, &ufsAPI.GetVMRequest{
		Name: ufsUtil.AddPrefix(ufsUtil.VMCollection, name),
	})
}

func ListVMs(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]proto.Message, string, error) {
	req := &ufsAPI.ListVMsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
	}
	res, err := ic.ListVMs(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]proto.Message, len(res.GetVms()))
	for i, m := range res.GetVms() {
		protos[i] = m
	}
	return protos, res.GetNextPageToken(), nil
}

func (c *getVM) printVMFull(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message, tsv bool) error {
	return c.printVMNormal(msgs, tsv, false)
}

func (c *getVM) printVMNormal(msgs []proto.Message, tsv, keysOnly bool) error {
	if tsv {
		utils.PrintTSVVMs(msgs, keysOnly, c.byteFormat)
		return nil
	}
	utils.PrintTableTitle(utils.VMTitle, tsv, keysOnly)
	utils.PrintVMs(msgs, keysOnly, c.byteFormat)
	return nil
}
