// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devboard

import (
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// AddDevboardMachineCmd adds a devboard machine to the database.
var AddDevboardMachineCmd = &subcommands.Command{
	UsageLine:  "devboard-machine ...",
	ShortDesc:  "Add a devboard machine",
	LongDesc:   cmdhelp.AddDevboardMachineLongDesc,
	CommandRun: addDevboardMachineCommandRun,
}

var supportedBoardTypes = []string{
	"andreiboard",
	"icetower",
}

func addDevboardMachineCommandRun() subcommands.CommandRun {
	c := &addDevboardMachine{}
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.commonFlags.Register(&c.Flags)

	c.Flags.StringVar(&c.name, "name", "", "The name of the attached device machine to add.")
	c.Flags.StringVar(&c.zone, "zone", "", cmdhelp.ZoneHelpText)
	c.Flags.StringVar(&c.rack, "rack", "", "The rack to add the attached device machine to.")
	c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of tag(s). Can be specified multiple times.")

	c.Flags.StringVar(&c.boardType, "type", "", "The type of devboard. Supported values: "+strings.Join(supportedBoardTypes, ", "))
	c.Flags.StringVar(&c.ultradebug, "ultradebug", "", "UltraDebug serial.")
	return c
}

type addDevboardMachine struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	name string
	zone string
	rack string
	tags []string

	boardType  string
	ultradebug string
}

func (c *addDevboardMachine) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *addDevboardMachine) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
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
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	var req ufsAPI.MachineRegistrationRequest
	c.parseArgs(&req)

	if !ufsUtil.ValidateTags(req.Machine.Tags) {
		return fmt.Errorf(ufsAPI.InvalidTags)
	}

	res, err := ic.MachineRegistration(ctx, &req)
	if err != nil {
		return err
	}
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	fmt.Println("Successfully added the attached device machine: ", res.GetName())
	return nil
}

func (c *addDevboardMachine) parseArgs(req *ufsAPI.MachineRegistrationRequest) {
	ufsZone := ufsUtil.ToUFSZone(c.zone)
	devboard := &ufspb.Devboard{}
	req.Machine = &ufspb.Machine{
		Name: c.name,
		Location: &ufspb.Location{
			Zone: ufsZone,
			Rack: c.rack,
		},
		Realm: ufsUtil.ToUFSRealm(c.zone),
		Tags:  c.tags,
		Device: &ufspb.Machine_Devboard{
			Devboard: devboard,
		},
	}
	switch c.boardType {
	case "andreiboard":
		devboard.Board = &ufspb.Devboard_Andreiboard{
			Andreiboard: &ufspb.Andreiboard{
				UltradebugSerial: c.ultradebug,
			},
		}
	case "icetower":
		devboard.Board = &ufspb.Devboard_Icetower{
			Icetower: &ufspb.Icetower{},
		}
	default:
		panic(fmt.Sprintf("Unsupported board type %q", c.boardType))
	}
}

func (c *addDevboardMachine) validateArgs() error {
	if c.name == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required.")
	}
	if c.zone == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-zone' is required.")
	}
	if c.boardType == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-type' is required.")
	}
	if !isSupportedBoardType(c.boardType) {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid board type, please check help info for '-type'.", c.boardType)
	}
	if !ufsUtil.IsUFSZone(ufsUtil.RemoveZonePrefix(c.zone)) {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid zone name, please check help info for '-zone'.", c.zone)
	}
	return nil
}

func isSupportedBoardType(s string) bool {
	for _, x := range supportedBoardTypes {
		if s == x {
			return true
		}
	}
	return false
}
