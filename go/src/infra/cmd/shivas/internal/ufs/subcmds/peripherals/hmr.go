// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	rpc "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/util"
)

var (
	AddPeripheralHMRCmd    = hmrCmd(actionAdd)
	DeletePeripheralHMRCmd = hmrCmd(actionDelete)
)

// hmrCmd creates command for adding, removing, or replacing HMR on a DUT.
func hmrCmd(mode action) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "peripheral-hmr -dut {DUT name} -touch-host-pi {touchhost hostname} -hmr-pi {hmrpi hostname} -hmr-model {hmrpi model}",
		ShortDesc: "Manage hmr system connections to a DUT",
		LongDesc:  cmdhelp.ManagePeripheralHMRLongDesc,
		CommandRun: func() subcommands.CommandRun {
			c := manageHmrCmd{mode: mode}
			c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
			c.envFlags.Register(&c.Flags)
			c.commonFlags.Register(&c.Flags)

			c.Flags.StringVar(&c.dutName, "dut", "", "DUT name to update")
			c.Flags.StringVar(&c.touchHostPi, "touch-host-pi", "", "hostname of touch-host-pi.")
			c.Flags.StringVar(&c.hmrPi, "hmr-pi", "", "hostname of hmr-pi.")
			c.Flags.StringVar(&c.hmrModel, "hmr-model", "", "model of hmr.")

			c.Flags.StringVar(&c.rpmHostname, "rpm", "", "hostname for rpm connected to hmr")
			c.Flags.StringVar(&c.rpmOutlet, "rpm-outlet", "", "outlet number of rpm connected to hmr")

			return &c
		},
	}
}

// manageHmrCmd supports adding, replacing, or deleting Hmr.
type manageHmrCmd struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dutName     string
	touchHostPi string
	hmrModel    string
	hmrPi       string

	rpmHostname string
	rpmOutlet   string

	mode action
}

// Run executed the hmr management subcommand. It cleans up passed flags and validates them.
func (c *manageHmrCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.run(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// run implements the core logic for Run.
func (c *manageHmrCmd) run(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.cleanAndValidateFlags(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, util.OSNamespace)

	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}

	client := rpc.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	lse, err := client.GetMachineLSE(ctx, &rpc.GetMachineLSERequest{
		Name: util.AddPrefix(util.MachineLSECollection, c.dutName),
	})
	if err != nil {
		return err
	}
	if err := utils.IsDUT(lse); err != nil {
		return errors.Annotate(err, "not a dut").Err()
	}

	var (
		peripherals = lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
		currentHmr  = peripherals.GetHumanMotionRobot()
	)

	newHmr, err := c.runHmrAction(currentHmr)
	if err != nil {
		return err
	}
	if c.commonFlags.Verbose() {
		fmt.Println("New HMR", newHmr)
	}

	peripherals.HumanMotionRobot = newHmr
	_, err = client.UpdateMachineLSE(ctx, &rpc.UpdateMachineLSERequest{MachineLSE: lse})
	return err
}

// runHmrAction returns a lab HMR based on the action specified in c.
func (c *manageHmrCmd) runHmrAction(current *lab.HumanMotionRobot) (*lab.HumanMotionRobot, error) {
	switch c.mode {
	case actionAdd:
		return c.createHmr()
	case actionDelete:
		return nil, nil
	default:
		return nil, errors.Reason("unknown action: %d", c.mode).Err()
	}
}

// createHmr returns a HMR based on what specified in c added.
func (c *manageHmrCmd) createHmr() (*lab.HumanMotionRobot, error) {
	hmr := &lab.HumanMotionRobot{
		Hostname:        c.hmrPi,
		HmrModel:        c.hmrModel,
		GatewayHostname: c.touchHostPi,
	}
	if c.rpmHostname != "" {
		hmr.Rpm = &lab.OSRPM{
			PowerunitName:   c.rpmHostname,
			PowerunitOutlet: c.rpmOutlet,
		}
	}
	return hmr, nil
}

const (
	errEmptyHmrModel            = "empty hmr model"
	errEmptyHmrPiHostname       = "empty hmr-pi hostname"
	errEmptyTouchHostPiHostname = "empty touch-host-pi hostname"
)

// cleanAndValidateFlags returns an error with the result of all validations. It strips whitespaces
// around hostnames and removes empty ones.
func (c *manageHmrCmd) cleanAndValidateFlags() error {
	var errStrs []string
	if len(c.dutName) == 0 {
		errStrs = append(errStrs, errDUTMissing)
	}

	if c.mode == actionDelete {
		return checkErrStr(c, errStrs)
	}

	checkHmrPiHostname(c, &errStrs)
	checkTouchHostPiHostname(c, &errStrs)
	checkHmrModel(c, &errStrs)
	checkRPM(c, &errStrs)

	return checkErrStr(c, errStrs)
}

func checkHmrPiHostname(c *manageHmrCmd, errStrs *[]string) {
	c.hmrPi = strings.TrimSpace(c.hmrPi)
	if c.hmrPi == "" {
		*errStrs = append(*errStrs, errEmptyHmrPiHostname)
	}
}

func checkTouchHostPiHostname(c *manageHmrCmd, errStrs *[]string) {
	c.touchHostPi = strings.TrimSpace(c.touchHostPi)
	if c.touchHostPi == "" {
		*errStrs = append(*errStrs, errEmptyTouchHostPiHostname)
	}
}

func checkHmrModel(c *manageHmrCmd, errStrs *[]string) {
	c.hmrModel = strings.TrimSpace(c.hmrModel)
	if c.hmrModel == "" {
		*errStrs = append(*errStrs, errEmptyHmrModel)
	}
}

func checkRPM(c *manageHmrCmd, errStrs *[]string) {
	if (c.rpmHostname != "" && c.rpmOutlet == "") || (c.rpmHostname == "" && c.rpmOutlet != "") {
		*errStrs = append(*errStrs, fmt.Sprintf("Need both rpm and its outlet. %s:%s is invalid", c.rpmHostname, c.rpmOutlet))
	}
}

func checkErrStr(c *manageHmrCmd, errStrs []string) error {
	if len(errStrs) == 0 {
		return nil
	}
	return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\n%s", strings.Join(errStrs, "\n")))
}
