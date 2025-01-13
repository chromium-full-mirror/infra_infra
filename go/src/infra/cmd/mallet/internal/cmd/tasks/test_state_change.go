// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"fmt"

	"github.com/maruel/subcommands"
	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/mallet/internal/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	"infra/cros/dutstate"
	"infra/libs/skylab/buildbucket"
	"infra/libs/skylab/common/heuristics"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// Run FW update to enable serial for the DUT.
var TestStateChange = &subcommands.Command{
	UsageLine: "test-state [-provision] [-reimage] [-usbkey] host...",
	ShortDesc: "update dut_state and set repair-requests for hosts",
	CommandRun: func() subcommands.CommandRun {
		c := &testStateChangeRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.newState, "state", "needs_repair", "Specify the state need to set for the DUT. Default needs-repair to address repair-requests")
		c.Flags.BoolVar(&c.needToClean, "clean", false, "Clean up repair-requests for DUT.")
		c.Flags.BoolVar(&c.needProvision, "provision", false, "Repair-request for provision request for DUT.")
		c.Flags.BoolVar(&c.needReimage, "reimage", false, "Repair-request for reimage request for DUT.")
		c.Flags.BoolVar(&c.needUpdateUSBkey, "usbkey", false, "Repair-request for re-downlaod image to USB drive request for DUT.")
		return c
	},
}

type testStateChangeRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	newState         string
	needToClean      bool
	needProvision    bool
	needReimage      bool
	needUpdateUSBkey bool
}

// Run executes a main logic of the tool.
func (c *testStateChangeRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *testStateChangeRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	if len(args) == 0 {
		return errors.Reason("internal run: unit is not specified").Err()
	}
	e := c.envFlags.Env()
	maskPaths := []string{"dut.state"}
	state := dutstate.ConvertToUFSState(dutstate.State(c.newState))
	if state == ufspb.State_STATE_UNSPECIFIED {
		return errors.Reason("internal run: state %q does not match any known state", c.newState).Err()
	}
	var repairRequests []ufslab.DutState_RepairRequest
	if c.needProvision {
		repairRequests = append(repairRequests, ufslab.DutState_REPAIR_REQUEST_PROVISION)
		maskPaths = append(maskPaths, "dut_state.repair_requests")
	}
	if c.needReimage {
		repairRequests = append(repairRequests, ufslab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY)
		maskPaths = append(maskPaths, "dut_state.repair_requests")
	}
	if c.needUpdateUSBkey {
		repairRequests = append(repairRequests, ufslab.DutState_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE)
		maskPaths = append(maskPaths, "dut_state.repair_requests")
	}
	if c.needToClean && len(repairRequests) > 0 {
		return errors.Reason("internal run: cannot set repair requests with clean up request").Err()
	} else if c.needToClean {
		// Create a new collection with a single item as an empty collection can be ignored by UFS.
		repairRequests = []ufslab.DutState_RepairRequest{ufslab.DutState_REPAIR_REQUEST_UNKNOWN}
		maskPaths = append(maskPaths, "dut_state.repair_requests")
	}
	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "internal run").Err()
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UFSService,
		Options: site.UFSPRPCOptions,
	})
	for i, arg := range args {
		args[i] = heuristics.NormalizeBotNameToDeviceName(arg)
	}
	var failDuts []string
	res := utils.ConcurrentGet(ctx, ic, args, utils.GetSingleMachineLSE)
	for _, r := range res {
		dut := r.(*ufspb.MachineLSE)
		dut.Name = ufsUtil.RemovePrefix(dut.Name)
		req := &ufsAPI.UpdateTestDataRequest{
			DeviceId:      dut.GetMachines()[0],
			Hostname:      dut.Name,
			ResourceState: state,
			UpdateMask:    &field_mask.FieldMask{Paths: maskPaths},
		}
		if len(repairRequests) > 0 {
			req.DeviceData = &ufsAPI.UpdateTestDataRequest_ChromeosData{
				ChromeosData: &ufsAPI.UpdateTestDataRequest_ChromeOs{
					DutState: &ufslab.DutState{
						RepairRequests: repairRequests,
					},
				},
			}
		}
		if _, err := ic.UpdateTestData(ctx, req); err != nil {
			failDuts = append(failDuts, dut.Name)
			fmt.Fprintf(a.GetErr(), "%s: fail with %s\n", dut.Name, err)
		} else {
			fmt.Fprintf(a.GetOut(), "%s: updated\n", dut.Name)
		}
	}
	if len(failDuts) > 0 {
		return errors.Reason("internal run: fail to do somethind").Err()
	}
	return nil
}
