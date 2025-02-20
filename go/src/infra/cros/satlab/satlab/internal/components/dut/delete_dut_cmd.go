// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"fmt"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/satlab/common/dut"
	"go.chromium.org/infra/cros/satlab/common/services/ufs"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DeleteDUTCmd is the implementation of the "satlab delete DUT" command.
var DeleteDUTCmd = &subcommands.Command{
	UsageLine: "dut [options ...]",
	ShortDesc: "Delete a Satlab DUT",
	CommandRun: func() subcommands.CommandRun {
		c := &deleteDUTCmd{}
		registerShivasFlags(c)
		return c
	},
}

// DeleteDUT holds the arguments that are needed for the delete DUT command.
type deleteDUTCmd struct {
	shivasDeleteDUT

	dut.DeleteDUT
}

// Run attempts to delete a DUT and returns an exit status.
func (c *deleteDUTCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// InnerRun is the implementation of the delete command.
func (c *deleteDUTCmd) innerRun(a subcommands.Application, positionalArgs []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, c.envFlags.GetNamespace())

	ufs, err := ufs.NewUFSClient(ctx, c.envFlags.GetUFSService(), &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "get dut").Err()
	}

	// assign the DUT names that we want to delete
	c.Names = positionalArgs

	// Validate the parameters before calling a `TriggerRun`
	if err = c.Validate(); err != nil {
		return err
	}

	res, err := c.TriggerRun(ctx, &executor.ExecCommander{}, ufs)

	printResult(res, c.Full)

	return err
}

// printResult print the delete DUTs result.
// it shows which Dut pass, and fail
func printResult(res *dut.DeleteDUTResult, full bool) {
	printMachineLSEs(res.MachineLSEs)
	fmt.Printf("\nSuccessfully deleted DUT(s):\n")
	fmt.Printf("%v\n", res.DutResults.Pass)
	fmt.Printf("\nFailed to delete DUT(s):\n")
	fmt.Printf("%v\n", res.DutResults.Fail)

	if full {
		fmt.Printf("\nSuccessfully deleted Assets(s):\n")
		fmt.Printf("%v\n", res.AssetResults.Pass)
		fmt.Printf("Failed to delete Assets(s):\n")
		fmt.Printf("%v\n\n", res.AssetResults.Fail)

		fmt.Printf("\nSuccessfully deleted Racks(s):\n")
		fmt.Printf("%v\n", res.RackResults.Pass)
		fmt.Printf("Failed to delete Racks(s):\n")
		fmt.Printf("%v\n\n", res.RackResults.Fail)
	}
}

func printMachineLSEs(machineLSEs []*ufsModels.MachineLSE) {
	for i, m := range machineLSEs {
		m.Name = ufsUtil.RemovePrefix(m.Name)
		PrintProtoJSON(m)
		if i < len(machineLSEs)-1 {
			fmt.Printf(",\n")
		}
	}
}

// PrintProtoJSON prints proto as a JSON object.
func PrintProtoJSON(pm proto.Message) {
	m := protojson.MarshalOptions{
		Indent:          "\t",
		EmitUnpopulated: true,
	}
	fmt.Print(m.Format(pm))
}
