// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schedulingunit

import (
	"fmt"

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

// UpdateSchedulingUnitCmd Update SchedulingUnit by given name.
var UpdateSchedulingUnitCmd = &subcommands.Command{
	UsageLine: "SchedulingUnit [Options...]",
	ShortDesc: "Update a SchedulingUnit",
	LongDesc:  cmdhelp.UpdateSchedulingUnitLongDesc,
	CommandRun: func() subcommands.CommandRun {
		c := &updateSchedulingUnit{
			pools:       []string{},
			duts:        []string{},
			tags:        []string{},
			removePools: []string{},
			removeDuts:  []string{},
			removeTags:  []string{},
		}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.newSpecsFile, "f", "", cmdhelp.SchedulingUnitUpdateFileText)

		c.Flags.StringVar(&c.name, "name", "", "name of the SchedulingUnit")
		c.Flags.Var(utils.CSVString(&c.pools), "pools", "append/clear comma separated pools. "+cmdhelp.ClearFieldHelpText)
		c.Flags.Var(utils.CSVString(&c.duts), "duts", "append/clear comma separated DUTs. "+cmdhelp.ClearFieldHelpText)
		c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of tag(s). Can be specified multiple times. "+cmdhelp.ClearFieldHelpText)
		c.Flags.Var(utils.CSVString(&c.removePools), "pools-to-remove", "remove comma separated pools.")
		c.Flags.Var(utils.CSVString(&c.removeDuts), "duts-to-remove", "remove comma separated DUTs.")
		c.Flags.Var(flag.StringSlice(&c.removeTags), "tag-to-remove", "Name(s) of tag(s) to be removed. Can be specified multiple times. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.schedulingUnitType, "type", "", "Type of SchedulingUnit. "+cmdhelp.SchedulingUnitTypesHelpText)
		c.Flags.StringVar(&c.description, "desc", "", "description for the SchedulingUnit")
		c.Flags.StringVar(&c.primaryDut, "primary-dut", "", "set primary dut. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.exposeType, "expose-type", "", "set type of labels to expose. "+cmdhelp.SchedulingUnitExposeTypesHelpText+" "+cmdhelp.ClearFieldHelpText)
		c.Flags.BoolVar(&c.wificell, "wificell", false, "adding this flag will specify if the scheduling unit is hosted in a wificell.")
		c.Flags.StringVar(&c.carrier, "carrier", "", "adding this flag will specify a carrier for scheduling units in cellular testbeds. "+cmdhelp.ClearFieldHelpText)
		return c
	},
}

type updateSchedulingUnit struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	newSpecsFile string

	name               string
	pools              []string
	duts               []string
	tags               []string
	removePools        []string
	removeDuts         []string
	removeTags         []string
	schedulingUnitType string
	description        string
	primaryDut         string
	exposeType         string
	carrier            string
	wificell           bool
}

func (c *updateSchedulingUnit) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateSchedulingUnit) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}

	mask := utils.GetUpdateMask(&c.Flags, map[string]string{
		"duts":            "machinelses",
		"pools":           "pools",
		"tags":            "tags",
		"duts-to-remove":  "machinelses.remove",
		"pools-to-remove": "pools.remove",
		"tags-to-remove":  "tags.remove",
		"type":            "type",
		"desc":            "description",
		"primary-dut":     "primary-dut",
		"expose-type":     "expose-type",
		"wificell":        "wificell",
		"carrier":         "carrier",
	})
	// Check if nothing is being updated. Updating with an empty mask overwrites everything.
	if len(mask.Paths) == 0 {
		return cmdlib.NewQuietUsageError(c.Flags, "Nothing to update")
	}

	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	var su ufspb.SchedulingUnit
	if c.newSpecsFile != "" {
		if err = utils.ParseJSONFile(c.newSpecsFile, &su); err != nil {
			return err
		}
	} else {
		c.parseArgs(&su)
	}
	if err := utils.PrintExistingSchedulingUnit(ctx, ic, su.Name); err != nil {
		return err
	}
	su.Name = ufsUtil.AddPrefix(ufsUtil.SchedulingUnitCollection, su.Name)

	res, err := ic.UpdateSchedulingUnit(ctx, &ufsAPI.UpdateSchedulingUnitRequest{
		SchedulingUnit: &su,
		UpdateMask:     mask,
	})
	if err != nil {
		return err
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	fmt.Println("The SchedulingUnit after update:")
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	fmt.Printf("Successfully updated the SchedulingUnit %s\n", res.Name)
	return nil
}

func (c *updateSchedulingUnit) parseArgs(su *ufspb.SchedulingUnit) {
	su.Name = c.name
	if len(c.removeDuts) > 0 {
		su.MachineLSEs = c.removeDuts
	} else if ufsUtil.ContainsAnyStrings(c.duts, utils.ClearFieldValue) {
		su.MachineLSEs = nil
	} else {
		su.MachineLSEs = c.duts
	}
	if len(c.removePools) > 0 {
		su.Pools = c.removePools
	} else if ufsUtil.ContainsAnyStrings(c.pools, utils.ClearFieldValue) {
		su.Pools = nil
	} else {
		su.Pools = c.pools
	}
	if len(c.removeTags) > 0 {
		su.Tags = c.removeTags
	} else if ufsUtil.ContainsAnyStrings(c.tags, utils.ClearFieldValue) {
		su.Tags = nil
	} else {
		su.Tags = c.tags
	}
	su.Type = ufsUtil.ToSchedulingUnitType(c.schedulingUnitType)
	if c.description == utils.ClearFieldValue {
		su.Description = ""
	} else {
		su.Description = c.description
	}
	if c.primaryDut != "" {
		if c.primaryDut == utils.ClearFieldValue {
			su.PrimaryDut = ""
		} else {
			su.PrimaryDut = c.primaryDut
		}
	}
	if c.exposeType != "" {
		su.ExposeType = ufsUtil.ToSchedulingUnitExposeType(c.exposeType)
	}
	su.Wificell = c.wificell
	if c.carrier == utils.ClearFieldValue {
		su.Carrier = ""
	} else {
		su.Carrier = c.carrier
	}
}

func (c *updateSchedulingUnit) validateArgs() error {
	if c.newSpecsFile != "" {
		if c.name != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-name' cannot be specified at the same time.")
		}
		if len(c.pools) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-pools' cannot be specified at the same time.")
		}
		if len(c.duts) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-duts' cannot be specified at the same time.")
		}
		if len(c.tags) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-tag' cannot be specified at the same time.")
		}
		if len(c.removePools) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-pools-to-remove' cannot be specified at the same time.")
		}
		if len(c.removeDuts) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-duts-to-remove' cannot be specified at the same time.")
		}
		if len(c.removeTags) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-tag-to-remove' cannot be specified at the same time.")
		}
		if c.schedulingUnitType != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-type' cannot be specified at the same time.")
		}
		if c.description != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-description' cannot be specified at the same time.")
		}
		if c.primaryDut != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-primary-dut' cannot be specified at the same time.")
		}
	}
	if c.newSpecsFile == "" {
		if c.name == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required, no mode ('-f') is specified.")
		}
		if c.schedulingUnitType != "" && !ufsUtil.IsSchedulingUnitType(c.schedulingUnitType) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid SchedulingUnitType name, please check help info for '-type'.", c.schedulingUnitType)
		}
		if len(c.duts) != 0 && len(c.removeDuts) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-duts' and '-duts-to-remove' cannot be specified at the same time.")
		}
		if len(c.pools) != 0 && len(c.removePools) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-pools' and '-pools-to-remove' cannot be specified at the same time.")
		}
		if len(c.tags) != 0 && len(c.removeTags) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-tag' and '-tag-to-remove' cannot be specified at the same time.")
		}
		if c.exposeType != "" && !ufsUtil.IsSchedulingUnitExposeType(c.exposeType) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not valid exposeType name, please check help info for 'expose-type'.", c.exposeType)
		}
	}
	return nil
}
