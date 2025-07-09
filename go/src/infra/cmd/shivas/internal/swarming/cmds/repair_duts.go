// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/libs/fleet/admintask"
	"go.chromium.org/infra/libs/fleet/device"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

type repairDuts struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	onlyVerify    bool // runs only verify actions instead of full repair
	latestVersion bool // uses the latest version of CIPD when scheduling. By default use prod.
	deepRepair    bool
	bbBucket      string
	bbBuilder     string
	useAdminLib   bool // use the new admin task library implementation (beta)
}

// RepairDutsCmd contains repair-duts command specification
var RepairDutsCmd = &subcommands.Command{
	UsageLine: "repair-duts",
	ShortDesc: "Repair the DUT by name",
	LongDesc: `Repair the DUT by name.
	./shivas repair-duts <dut_name1> ...
	Schedule a swarming Repair task to the DUT to try to recover/verify it.`,
	CommandRun: func() subcommands.CommandRun {
		c := &repairDuts{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.onlyVerify, "verify", false, "Run only verify actions.")
		c.Flags.BoolVar(&c.latestVersion, "latest", false, "Use latest version of CIPD when scheduling. By default use prod.")
		c.Flags.BoolVar(&c.deepRepair, "deep", false, "Use deep-repair task when scheduling a task.")
		//TODO(macdisi): add validation for bucket and builder parameters, allowlist or otherwise
		c.Flags.StringVar(&c.bbBucket, "bucket", "labpack_runner", "Buildbucket bucket to use.")
		c.Flags.StringVar(&c.bbBuilder, "builder", "repair", "Buildbucket builder to use.")
		//TODO: b/394429368 - remove this flag once the new admin task library is stable
		c.Flags.BoolVar(&c.useAdminLib, "admin-lib", false, "Use the new admin task library implementation (beta).")
		return c
	},
}

// Run represent runner for reserve command
func (c *repairDuts) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun is the core logic for the repair-duts subcommand
// creates clients, prepares environment, schedules tasks, prints links to swarming UI
func (c *repairDuts) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	if len(args) == 0 {
		return errors.Reason("at least one hostname has to be provided").Err()
	}
	ctx := cli.GetContext(a, c, env)
	ns, err := getNamespace(&c.envFlags)
	if err != nil {
		return err
	}
	ctx = utils.SetupContext(ctx, ns)
	e := c.envFlags.Env()
	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions(c.envFlags))
	if err != nil {
		return err
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "getting auth opts").Err()
	}
	sessionTag := fmt.Sprintf("admin-session:%s", uuid.New().String())

	var adminClient *admintask.Client
	if c.useAdminLib {
		adminClient = &admintask.Client{
			UFSClient:               ic,
			BBClient:                bc,
			AdminServiceAddress:     e.AdminService,
			InventoryServiceAddress: e.UnifiedFleetService,
			InventoryNamespace:      ns,
			Version:                 buildbucket.CipdVersion(c.latestVersion),
		}
	}

	tags := []string{
		sessionTag,
		"task:recovery",
		utils.ShivasClientTag,
		"qs_account:unmanaged_p0",
		fmt.Sprintf("version:%s", buildbucket.CipdVersion(c.latestVersion)),
	}

	for _, dutName := range args {

		var url string
		var taskErr error

		if adminClient != nil {
			sc, err := utils.SchedukeClient(ctx, ic, authOpts, dutName)
			if err != nil {
				fmt.Fprintf(a.GetErr(), "%s: failed to create Scheduke client %s\n", dutName, err)
				continue
			}
			adminClient.SchedukeClient = sc

			repairTaskRequest := &admintask.RepairTaskRequest{
				Task: admintask.Task{
					UpdateInventory: true,
					ExtraTags:       tags,
					BuilderName:     c.bbBuilder,
					BuilderBucket:   c.bbBucket,
				},
				UnitName:   dutName,
				DeepRepair: c.deepRepair,
				OnlyVerify: c.onlyVerify,
			}

			result, err := adminClient.ScheduleRepairTask(
				ctx,
				repairTaskRequest,
			)
			taskErr = err

			if result != nil {
				url = result.TaskURL
			}
		} else {
			hive := ufsUtil.GetHiveForDut(dutName, utils.GetHive(ctx, ic, dutName))
			builderName, taskName := c.getBuilderAndTaskName()
			realBuilderName := buildbucket.BuilderNamePerHive(builderName, hive)

			adminParams, err := utils.PrepareAdminParams(ctx, dutName, realBuilderName, e.AdminService, ic, authOpts)
			if err != nil {
				fmt.Fprintf(a.GetErr(), "%s: failed to prepare admin params %s\n", dutName, err)
				continue
			}

			di, err := device.GetDeviceInfo(ctx, ic, dutName)
			if err != nil {
				fmt.Fprintf(a.GetErr(), "%s: failed to get device info %s\n", dutName, err)
				continue
			}

			url, _, taskErr = buildbucket.CreateTask(
				ctx,
				bc,
				adminParams.SchedukeClient,
				buildbucket.CipdVersion(c.latestVersion),
				&buildbucket.Params{
					UnitName:       dutName,
					UnitID:         di.ID,
					TaskName:       taskName,
					BuilderName:    realBuilderName,
					BuilderBucket:  c.bbBucket,
					EnableRecovery: !c.onlyVerify,
					AdminService:   adminParams.AdminService,
					// NOTE: We use the UFS service, not the Inventory service here.
					InventoryService:   e.UnifiedFleetService,
					InventoryNamespace: adminParams.ContextNamespace,
					UpdateInventory:    true,
					ExtraTags:          tags,
				},
				"shivas",
			)
		}

		if taskErr != nil {
			fmt.Fprintf(a.GetOut(), "%s: %s\n", dutName, taskErr.Error())
		} else {
			fmt.Fprintf(a.GetOut(), "%s: %s\n", dutName, url)
		}
	}
	utils.PrintTasksBatchLink(a.GetOut(), e.SwarmingService, sessionTag)
	return nil
}

// getNamespace returns the namespace used to call UFS with appropriate
// validation and default behavior. It is primarily separated from the main
// function for testing purposes
func getNamespace(c *site.EnvFlags) (string, error) {
	if c == nil {
		return ufsUtil.OSNamespace, nil
	}
	return c.Namespace(site.OSLikeNamespaces, ufsUtil.OSNamespace)
}

func (c *repairDuts) getBuilderAndTaskName() (string, string) {
	builderName := c.bbBuilder
	if c.onlyVerify {
		builderName = "verify"
	}
	taskName := string(buildbucket.Recovery)
	if c.deepRepair {
		taskName = string(buildbucket.DeepRecovery)
	}
	return builderName, taskName
}
