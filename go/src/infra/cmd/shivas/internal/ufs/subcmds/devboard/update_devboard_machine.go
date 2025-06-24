// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devboard

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

// UpdateDevboardMachineCmd updates the devboard machine for a given name.
var UpdateDevboardMachineCmd = &subcommands.Command{
	UsageLine:  "devboard-machine ...",
	ShortDesc:  "Update devboard machine details by filters",
	LongDesc:   cmdhelp.UpdateDevboardMachineText,
	CommandRun: updateADMCommandRun,
}

func updateADMCommandRun() subcommands.CommandRun {
	c := &updateDevboardMachine{}
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.commonFlags.Register(&c.Flags)

	c.Flags.StringVar(&c.name, "name", "", "The name of the devboard machine to update.")
	c.Flags.StringVar(&c.zone, "zone", "", cmdhelp.ZoneHelpText)
	c.Flags.StringVar(&c.rackName, "rack", "", "The rack to add the devboard machine to. "+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of tag(s). Can be specified multiple times. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.state, "state", "", cmdhelp.StateHelp)
	c.Flags.StringVar(&c.ultradebug, "ultradebug", "", "UltraDebug serial.")
	return c
}

type updateDevboardMachine struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	newSpecsFile string

	name       string
	zone       string
	rackName   string
	tags       []string
	state      string
	ultradebug string
}

func (c *updateDevboardMachine) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateDevboardMachine) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	var machine ufspb.Machine
	c.parseArgs(&machine)

	_, err = utils.PrintExistingDevboardMachine(ctx, ic, machine.Name)
	if err != nil {
		return err
	}

	machine.Name = ufsUtil.AddPrefix(ufsUtil.MachineCollection, machine.Name)
	if !ufsUtil.ValidateTags(machine.Tags) {
		return fmt.Errorf(ufsAPI.InvalidTags)
	}
	res, err := ic.UpdateMachine(ctx, &ufsAPI.UpdateMachineRequest{
		Machine: &machine,
		UpdateMask: utils.GetUpdateMask(&c.Flags, map[string]string{
			"zone":       ufsUtil.ZonePath,
			"rack":       ufsUtil.LocationRackPath,
			"tag":        ufsUtil.TagsPath,
			"state":      ufsUtil.ResourceStatePath,
			"ultradebug": ufsUtil.AndreiboardUltradebugSerialPath,
		}),
	})
	if err != nil {
		return err
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	fmt.Println("The devboard machine after update:")
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	fmt.Println("Successfully updated the devboard machine: ", res.Name)
	return nil
}

func (c *updateDevboardMachine) parseArgs(machine *ufspb.Machine) {
	devboard := &ufspb.Devboard{}
	machine.Device = &ufspb.Machine_Devboard{
		Devboard: devboard,
	}
	machine.Name = c.name
	machine.Location = &ufspb.Location{}
	if c.zone == utils.ClearFieldValue {
		machine.GetLocation().Zone = ufsUtil.ToUFSZone("")
	} else {
		machine.GetLocation().Zone = ufsUtil.ToUFSZone(c.zone)
	}
	if c.rackName == utils.ClearFieldValue {
		machine.GetLocation().Rack = ""
	} else {
		machine.GetLocation().Rack = c.rackName
	}
	if ufsUtil.ContainsAnyStrings(c.tags, utils.ClearFieldValue) {
		machine.Tags = nil
	} else {
		machine.Tags = c.tags
	}
	machine.ResourceState = ufsUtil.ToUFSState(c.state)

	t, err := c.boardType()
	if err != nil {
		// This case should already have been checked in validateArgs.
		panic(err)
	}
	switch t {
	case "andreiboard":
		b := &ufspb.Andreiboard{
			UltradebugSerial: c.ultradebug,
		}
		devboard.Board = &ufspb.Devboard_Andreiboard{
			Andreiboard: b,
		}
		if c.ultradebug == utils.ClearFieldValue {
			b.UltradebugSerial = ""
		} else {
			b.UltradebugSerial = c.ultradebug
		}
	case "icetower":
		devboard.Board = &ufspb.Devboard_Icetower{
			Icetower: &ufspb.Icetower{},
		}
	default:
		panic(fmt.Sprintf("unexpected board type %q", t))
	}
	machine.Realm = ufsUtil.ToUFSRealm(machine.GetLocation().GetZone().String())
}

// boardType returns the devboard type based on the fields being updated.
// Returns an empty string if no devboard specific fields are being updated.
// Returns an error if fields from conflicting types are being updated.
func (c *updateDevboardMachine) boardType() (string, error) {
	if c.ultradebug != "" {
		return "andreiboard", nil
	}
	// Currently there are no icetower specific fields, but make
	// it convenient to add them in the future.
	if false {
		return "icetower", nil
	}
	return "", nil
}

func (c *updateDevboardMachine) validateArgs() error {
	if c.name == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required.")
	}
	if c.zone != "" && !ufsUtil.IsUFSZone(ufsUtil.RemoveZonePrefix(c.zone)) {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%q is not a valid zone name, please check help info for '-zone'.", c.zone)
	}
	if c.state != "" && !ufsUtil.IsUFSState(ufsUtil.RemoveStatePrefix(c.state)) {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%q is not a valid state, please check help info for '-state'.", c.state)
	}
	if _, err := c.boardType(); err != nil {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nConflicting devboard fields provided: %s", err)
	}
	return nil
}
