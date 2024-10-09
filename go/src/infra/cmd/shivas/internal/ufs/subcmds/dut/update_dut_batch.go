// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"
	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/internal/ufs/subcmds/host"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

const defaultDutCount = -1

const (
	// Valid values for 'update-field' flag
	logicalZoneField = "logicalzone"
)

// UpdateDUTBatchCmd update a batch of duts based on the filters
var UpdateDUTBatchCmd = &subcommands.Command{
	UsageLine: "dut-batch [options]",
	ShortDesc: "Update a batch of DUTs",
	LongDesc:  cmdhelp.UpdateDUTBatchLongDesc,
	CommandRun: func() subcommands.CommandRun {
		c := &updateDUTBatch{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.updateField, "update-field", "", "The name of the field to update. Valid values: "+validUpdateFieldsString()+".")
		c.Flags.StringVar(&c.updateValue, "update-value", "", "The value to set for the update field. Run `shivas update dut -help` for info on the field.")

		c.Flags.Var(flag.StringSlice(&c.models), "model", "Name(s) of a model to include. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.boards), "board", "Name(s) of a board to include. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.pools), "pool", "Name(s) of a board to include. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.zones), "zone", "Name(s) of a zone to include. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.logicalZones), "logical-zone", "Name(s) of a board to include. Can be specified multiple times.")
		c.Flags.Var(flag.StringSlice(&c.states), "state", "Name(s) of a board to include. Can be specified multiple times.")

		c.Flags.IntVar(&c.percentage, "percentage", defaultDutCount, "N%% of DUTs to update, chosen randomly after filters. Cannot be combined with -count. Valid values: [1,100].")
		c.Flags.IntVar(&c.count, "count", defaultDutCount, "Number of DUTs to update, chosen randomly after filters. Cannot be combined with -percentage. Must be a positive integer.")
		return c
	},
}

type updateDUTBatch struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	updateField string
	updateValue string
	percentage  int
	count       int

	models       []string
	boards       []string
	pools        []string
	zones        []string
	logicalZones []string
	states       []string
}

// List of valid fields for bulk update
func validUpdateFieldsString() string {
	validFieldsList := []string{logicalZoneField}
	return fmt.Sprintf("[%s]", strings.Join(validFieldsList, ", "))
}

func (c *updateDUTBatch) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateDUTBatch) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}

	ctx := cli.GetContext(a, c, env)
	ns, err := c.getNamespace()
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

	var filteredDuts []proto.Message
	filteredDuts, err = utils.BatchList(ctx, ic, host.ListHosts, c.formatFilters(), 0, true, false, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Found %v DUTs matching the filters\n", len(filteredDuts))
	if c.commonFlags.Verbose() {
		fmt.Printf("-----\n")
		utils.PrintDutsShort(filteredDuts, true)
		fmt.Printf("-----\n")
	}

	requests, err := c.generateUpdateRequests(filteredDuts)
	if err != nil {
		return err
	}

	// Create a summary results table with 3 columns.
	resTable := utils.NewSummaryResultsTable([]string{"DUT", ufsOp})

	for _, req := range requests {
		// Attempt to update UFS.
		err = c.updateDUTToUFS(ctx, ic, req)
		// Record the result of the action.
		resTable.RecordResult(ufsOp, req.MachineLSE.GetName(), err)
	}

	fmt.Printf("\nSummary of results:\n\n")
	resTable.PrintResultsTable(os.Stdout, false)

	return nil
}

func (c *updateDUTBatch) formatFilters() []string {
	filters := make([]string, 0)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.ModelFilterName, c.models)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.BoardFilterName, c.boards)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.PoolsFilterName, c.pools)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.ZoneFilterName, c.zones)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.LogicalZoneFilterName, c.logicalZones)...)
	filters = utils.JoinFilters(filters, utils.PrefixFilters(ufsUtil.StateFilterName, c.states)...)
	return filters
}

// getNamespace returns the namespace used to call UFS with appropriate
// validation and default behavior. It is primarily separated from the main
// function for testing purposes
func (c *updateDUTBatch) getNamespace() (string, error) {
	return c.envFlags.Namespace(site.OSLikeNamespaces, ufsUtil.OSNamespace)
}

// validateArgs validates the set of inputs to updateDUTBatch.
func (c updateDUTBatch) validateArgs() error {
	if err := c.validateFieldUpdate(); err != nil {
		return err
	}

	if c.percentage != defaultDutCount {
		if c.count != defaultDutCount {
			return cmdlib.NewQuietUsageError(c.Flags, "Only one of percentage and count should be specified")
		}
		if c.percentage < 1 || c.percentage > 100 {
			return cmdlib.NewQuietUsageError(c.Flags, "Percentage should be between 1 and 100 inclusive")
		}
	} else if c.count != defaultDutCount {
		if c.count < defaultDutCount {
			return cmdlib.NewQuietUsageError(c.Flags, "Count must be a positive integer")
		}
	} else {
		return cmdlib.NewQuietUsageError(c.Flags, "If you want to modify ALL DUTs that match the filters, specify -percentage 100")
	}
	return nil
}

// validateFieldUpdate validates that the bulk field update has valid values
func (c updateDUTBatch) validateFieldUpdate() error {
	switch c.updateField {
	case "":
		return cmdlib.NewQuietUsageError(c.Flags, "Need field to update")
	case logicalZoneField:
		if c.updateValue == "" || !ufsUtil.IsLogicalZone(c.updateValue) {
			return cmdlib.NewQuietUsageError(c.Flags, "Not a valid logicalzone value to update.")
		}
		return nil
	default:
		return cmdlib.NewQuietUsageError(c.Flags, "Not a valid field")
	}
}

// generateUpdateRequests takes a list of duts and cmd line parameters
// and generates a list of update requests
func (c *updateDUTBatch) generateUpdateRequests(duts []proto.Message) ([]*ufsAPI.UpdateMachineLSERequest, error) {
	numToUpdate := len(duts)
	if c.percentage != defaultDutCount {
		numToUpdate = (len(duts) * c.percentage) / 100
	} else if c.count != defaultDutCount {
		if len(duts) < c.count {
			return nil, cmdlib.NewQuietUsageError(c.Flags, "Number of filtered DUTs %v is less than specified count %v", len(duts), c.count)
		}
		numToUpdate = c.count
	}

	rand.Shuffle(len(duts), func(i, j int) {
		duts[i], duts[j] = duts[j], duts[i]
	})

	fmt.Printf("Updating %v DUTs\n", numToUpdate)
	if c.commonFlags.Verbose() {
		fmt.Printf("DUTs to update:\n%v\n\n", duts[:numToUpdate])
	}

	requests := []*ufsAPI.UpdateMachineLSERequest{}
	for _, r := range duts[:numToUpdate] {
		dut := r.(*ufspb.MachineLSE)
		dut.Name = ufsUtil.RemovePrefix(dut.Name)
		lse, mask, err := c.initializeLSEAndMask(dut.Name)
		if err != nil {
			return nil, err
		}
		requests = append(requests, &ufsAPI.UpdateMachineLSERequest{
			MachineLSE: lse,
			UpdateMask: mask,
		})
	}
	return requests, nil
}

// initializeLSEAndMask creates the MachineLSE and FieldMask for the dut update request
func (c *updateDUTBatch) initializeLSEAndMask(hostname string) (*ufspb.MachineLSE, *field_mask.FieldMask, error) {
	// Generate lse and mask
	lse := &ufspb.MachineLSE{
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Dut{
							Dut: &chromeosLab.DeviceUnderTest{
								Peripherals: &chromeosLab.Peripherals{
									Chameleon:     &chromeosLab.Chameleon{},
									Servo:         &chromeosLab.Servo{},
									Rpm:           &chromeosLab.OSRPM{},
									Audio:         &chromeosLab.Audio{},
									Wifi:          &chromeosLab.Wifi{},
									Touch:         &chromeosLab.Touch{},
									CameraboxInfo: &chromeosLab.Camerabox{},
								},
							},
						},
					},
				},
			},
		},
	}
	mask := &field_mask.FieldMask{}
	lse.Name = hostname
	lse.Hostname = hostname
	lse.GetChromeosMachineLse().GetDeviceLse().GetDut().Hostname = hostname

	switch c.updateField {
	case logicalZoneField:
		mask.Paths = append(mask.Paths, logicalZonePath)
		lse.LogicalZone = ufsUtil.ToLogicalZone(c.updateValue)
	default:
		return nil, nil, errors.New("Internal Error. Could not find field to update")
	}

	return lse, mask, nil
}

// updateDUTToUFS verifies the request and calls UpdateMachineLSE API with the given request.
func (c *updateDUTBatch) updateDUTToUFS(ctx context.Context, ic ufsAPI.FleetClient, req *ufsAPI.UpdateMachineLSERequest) error {
	// Validate the update request.
	if err := validateUpdateDUTRequest(ctx, ic, req); err != nil {
		return err
	}
	req.MachineLSE.Name = ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, req.MachineLSE.Name)
	res, err := ic.UpdateMachineLSE(ctx, req)
	if err != nil {
		return err
	}
	// Remove prefix from the request. It's used for comparison later.
	req.MachineLSE.Name = ufsUtil.RemovePrefix(req.MachineLSE.Name)
	res.Name = ufsUtil.RemovePrefix(res.Name)
	fmt.Printf("Successfully updated DUT to UFS: %s \n", res.GetName())
	return nil
}
