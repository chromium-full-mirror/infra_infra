// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	crosfleetcommon "go.chromium.org/infra/cmd/crosfleet/internal/common"
	dutinfopb "go.chromium.org/infra/cmd/crosfleet/internal/proto"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmd/crosfleet/internal/ufs"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsutil "go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	infoCmdName = "info"
)

var info = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s HOSTNAME [HOSTNAME...]", infoCmdName),
	ShortDesc: "print DUT information",
	LongDesc: `Print DUT information from UFS.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &infoRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.printer.Register(&c.Flags)
		return c
	},
}

type infoRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  crosfleetcommon.EnvFlags
	printer   crosfleetcommon.CLIPrinter
}

func (c *infoRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		crosfleetcommon.PrintCmdError(a, err)
		return 1
	}
	return 0
}

func (c *infoRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) == 0 {
		return fmt.Errorf("missing DUT hostname arg")
	}
	ctx := cli.GetContext(a, c, env)
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return err
	}

	var infoList dutinfopb.DUTInfoList
	for _, deviceName := range args {
		info, err := common.UFSDeviceInfo(ctx, deviceName, authOpts)
		if err != nil {
			c.printer.WriteTextStdout("RPC error: %s", err.Error())
		}
		c.printer.WriteTextStdout("%s\n", dutInfoAsBashVariables(info))
		infoList.DUTs = append(infoList.DUTs, &dutinfopb.DUTInfo{
			Hostname: info.Name,
			LabSetup: info.LabSetup,
			Machine:  info.Machine,
		})
	}
	c.printer.WriteJSONStdout(&infoList)
	return nil
}

// getDutInfo returns information about the DUT with the given hostname, and a
// bool indicating whether all information fields were found in UFS.
func getDutInfo(ctx context.Context, ufsClient ufs.Client, hostname string) (*common.DeviceInfo, bool, error) {
	info := &common.DeviceInfo{
		Name: heuristics.NormalizeBotNameToDeviceName(hostname),
	}

	ctx = contextWithOSNamespace(ctx)
	var err error
	info.LabSetup, err = ufsClient.GetMachineLSE(ctx, &ufsapi.GetMachineLSERequest{
		Name: ufsutil.AddPrefix(ufsutil.MachineLSECollection, info.Name),
	})
	if err != nil {
		return info, false, err
	}
	if names := info.LabSetup.GetMachines(); len(names) > 0 && names[0] != "" {
		info.Machine, err = ufsClient.GetMachine(ctx, &ufsapi.GetMachineRequest{
			Name: ufsutil.AddPrefix(ufsutil.MachineCollection, names[0]),
		})
		if err != nil {
			return info, false, err
		}
	}
	allFieldsFound := info.Name != "" && info.LabSetup != nil && info.Machine != nil
	return info, allFieldsFound, nil
}

// dutInfoAsBashVariables returns a pretty-printed string containing info about
// the given DUT formatted as bash variables. Only the variables that are found
// in the DUT info proto message are printed.
func dutInfoAsBashVariables(info *common.DeviceInfo) string {
	var bashVars []string

	hostname := info.Name
	if hostname != "" {
		bashVars = append(bashVars,
			fmt.Sprintf("DUT_HOSTNAME=%s", hostname))
	}

	if info.Machine != nil {
		chromeOSMachine := info.Machine.GetChromeosMachine()
		if chromeOSMachine != nil {
			bashVars = append(bashVars,
				fmt.Sprintf("MODEL=%s\nBOARD=%s",
					chromeOSMachine.GetModel(),
					chromeOSMachine.GetBuildTarget()))
		}
	}

	if info.LabSetup != nil {
		servo := info.LabSetup.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
		if servo != nil {
			bashVars = append(bashVars,
				fmt.Sprintf("SERVO_HOSTNAME=%s\nSERVO_PORT=%d\nSERVO_SERIAL=%s",
					servo.GetServoHostname(),
					servo.GetServoPort(),
					servo.GetServoSerial()))
		}
	}

	return strings.Join(bashVars, "\n")
}
