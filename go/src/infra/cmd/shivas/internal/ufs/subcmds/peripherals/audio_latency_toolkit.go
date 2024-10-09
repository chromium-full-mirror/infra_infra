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

const (
	DefaultAudioLatencyToolkitCommand = "peripheral-audio-latency-toolkit"
	AliasedAudioLatencyToolkitCommand = "peripheral-alt"
)

var (
	AddPeripheralAudioLatencyToolkitCmd    = audioLatencyToolkitCmd(actionAdd, DefaultAudioLatencyToolkitCommand)
	AddPeripheralALTCmd                    = audioLatencyToolkitCmd(actionAdd, AliasedAudioLatencyToolkitCommand)
	DeletePeripheralAudioLatencyToolkitCmd = audioLatencyToolkitCmd(actionDelete, DefaultAudioLatencyToolkitCommand)
	DeletePeripheralALTCmd                 = audioLatencyToolkitCmd(actionDelete, AliasedAudioLatencyToolkitCommand)
)

// audioLatencyToolkitCmd creates command for adding, removing, or replacing Audio Latency Toolkit on a DUT.
func audioLatencyToolkitCmd(mode action, command string) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: fmt.Sprintf("%s -dut {DUT name}", command),
		ShortDesc: "Manage Audio Latency Toolkit connected to a DUT",
		LongDesc:  cmdhelp.ManagePeripheralAudioLatencyToolkitLongDesc,
		CommandRun: func() subcommands.CommandRun {
			c := manageAudioLatencyToolkitCmd{mode: mode}
			c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
			c.envFlags.Register(&c.Flags)
			c.commonFlags.Register(&c.Flags)

			c.Flags.StringVar(&c.dutName, "dut", "", "DUT name to update")
			c.version = "4.1"

			return &c
		},
	}
}

// manageAudioLatencyToolkitCmd supports adding, replacing, or deleting Audio Latency Toolkit.
type manageAudioLatencyToolkitCmd struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dutName string
	version string

	mode action
}

// Run executed the Audio Latency Toolkit management subcommand.
func (c *manageAudioLatencyToolkitCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.run(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// run implements the core logic for Run. It cleans up passed flags and validates them and updates the MachineLSE
func (c *manageAudioLatencyToolkitCmd) run(a subcommands.Application, args []string, env subcommands.Env) error {
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

	newAudioLatencyToolkit, err := c.runAudioLatencyToolkitAction()
	if err != nil {
		return err
	}
	if c.commonFlags.Verbose() {
		fmt.Println("New Audio Latency Toolkit", newAudioLatencyToolkit)
	}

	peripherals := lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
	peripherals.AudioLatencyToolkit = newAudioLatencyToolkit

	_, err = client.UpdateMachineLSE(ctx, &rpc.UpdateMachineLSERequest{MachineLSE: lse})
	return err
}

// runAudioLatencyToolkitAction returns a lab Audio Latency Toolkit based on the action specified in c.
func (c *manageAudioLatencyToolkitCmd) runAudioLatencyToolkitAction() (*lab.AudioLatencyToolkit, error) {
	switch c.mode {
	case actionAdd:
		return c.createAudioLatencyToolkit()
	case actionDelete:
		return nil, nil
	default:
		return nil, errors.Reason("unknown action: %d", c.mode).Err()
	}
}

// createAudioLatencyToolkit returns a Audio Latency Toolkit based on what specified in c added.
func (c *manageAudioLatencyToolkitCmd) createAudioLatencyToolkit() (*lab.AudioLatencyToolkit, error) {
	audioLatencyToolkit := &lab.AudioLatencyToolkit{
		Version: c.version,
	}
	return audioLatencyToolkit, nil
}

// cleanAndValidateFlags returns an error with the result of all validations. It strips whitespaces
// around hostnames and removes empty ones.
func (c *manageAudioLatencyToolkitCmd) cleanAndValidateFlags() error {
	var errStrs []string

	c.dutName = strings.TrimSpace(c.dutName)
	if len(c.dutName) == 0 {
		errStrs = append(errStrs, errDUTMissing)
	}

	return c.checkErrStr(errStrs)
}

func (c *manageAudioLatencyToolkitCmd) checkErrStr(errStrs []string) error {
	if len(errStrs) == 0 {
		return nil
	}
	return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\n%s", strings.Join(errStrs, "\n")))
}
