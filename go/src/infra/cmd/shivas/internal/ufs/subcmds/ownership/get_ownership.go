// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ownership

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// GetOwnershipDataCmd gets the ownership by the given name.
var GetOwnershipDataCmd = &subcommands.Command{
	UsageLine: "ownership-data ...",
	ShortDesc: "Get ownership data by filters",
	LongDesc: `Get ownership data by filters.

Example:

shivas get ownership-data {name1}
shivas get ownership-data {name1} {name2}
shivas get ownership-data

Gets the ownership data and prints the output in the user-specified format.`,
	CommandRun: func() subcommands.CommandRun {
		c := &getOwnershipData{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.outputFlags.Register(&c.Flags)

		c.Flags.IntVar(&c.pageSize, "n", 0, cmdhelp.ListPageSizeDesc)
		c.Flags.BoolVar(&c.keysOnly, "keys", false, cmdhelp.KeysOnlyText)
		c.Flags.StringVar(&c.commitsh, "commitsh", "", "Commitsh to get ownership configs at a particular commit. Only one commitsh can be specified.")

		return c
	},
}

type getOwnershipData struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	pageSize int
	keysOnly bool

	// Filters
	commitsh string
}

func (c *getOwnershipData) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getOwnershipData) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
	if len(args) == 1 {
		res = utils.ConcurrentGet(ctx, ic, args, c.getSingle)
		return utils.PrintEntities(ctx, ic, res, utils.PrintOwnershipsJSON, printOwnershipFull, printOwnershipNormal,
			c.outputFlags.JSON(), emit, full, c.outputFlags.Tsv(), c.keysOnly)
	} else {
		if len(args) > 0 {
			res = utils.ConcurrentGet(ctx, ic, args, c.getSingleWithHostName)
		} else {
			res, err = utils.BatchList(ctx, ic, ListOwnerships, c.formatFilters(), c.pageSize, c.keysOnly, full, nil)
		}
		if err != nil {
			return err
		}
		return utils.PrintEntities(ctx, ic, res, utils.PrintOwnershipsJSONByHost, printOwnershipByHostFull, printOwnershipByHostNormal,
			c.outputFlags.JSON(), emit, full, c.outputFlags.Tsv(), c.keysOnly)
	}
}

// Get a single ownershipdata entry
func (c *getOwnershipData) getSingle(ctx context.Context, ic ufsAPI.FleetClient, name string) (proto.Message, error) {
	return ic.GetOwnershipData(ctx, &ufsAPI.GetOwnershipDataRequest{
		Hostname: name,
	})
}

// Get a single ownershipdata entry along with hostname
func (c *getOwnershipData) getSingleWithHostName(ctx context.Context, ic ufsAPI.FleetClient, name string) (proto.Message, error) {
	msg, err := ic.GetOwnershipData(ctx, &ufsAPI.GetOwnershipDataRequest{
		Hostname: name,
	})
	if err != nil {
		return nil, err
	}
	res := &ufsAPI.OwnershipByHost{
		Hostname:  name,
		Ownership: msg,
	}
	return res, err
}

// Formats the specified filters
func (c *getOwnershipData) formatFilters() []string {
	filters := make([]string, 0)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.CommittishFilterName, []string{c.commitsh})...)
	return filters
}

// ListHosts calls the list MachineLSE in UFS to get a list of MachineLSEs
func ListOwnerships(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]proto.Message, string, error) {
	req := &ufsAPI.ListOwnershipDataRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
	}
	res, err := ic.ListOwnershipData(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]proto.Message, len(res.GetOwnershipData()))
	for i, od := range res.GetOwnershipData() {
		protos[i] = od
	}
	return protos, res.GetNextPageToken(), nil
}

func printOwnershipFull(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message, tsv bool) error {
	return printOwnershipNormal(msgs, tsv, false)
}

func printOwnershipNormal(entities []proto.Message, tsv, keysOnly bool) error {
	if len(entities) == 0 {
		return nil
	}
	if tsv {
		utils.PrintTSVOwnerships(entities, keysOnly)
		return nil
	}
	utils.PrintTableTitle(utils.OwnershipDataTitle, tsv, keysOnly)
	utils.PrintOwnerships(entities, keysOnly)
	return nil
}

func printOwnershipByHostFull(ctx context.Context, ic ufsAPI.FleetClient, msgs []proto.Message, tsv bool) error {
	return printOwnershipNormal(msgs, tsv, false)
}

func printOwnershipByHostNormal(entities []proto.Message, tsv, keysOnly bool) error {
	if len(entities) == 0 {
		return nil
	}
	if tsv {
		utils.PrintTSVOwnershipsByHost(entities, keysOnly)
		return nil
	}
	utils.PrintTableTitle(utils.OwnershipDataByHostTitle, tsv, keysOnly)
	utils.PrintOwnershipsByHost(entities, keysOnly)
	return nil
}
