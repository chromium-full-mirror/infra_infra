// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	schedulingapi "infra/libs/fleet/scheduling/api"
	"infra/libs/skylab/buildbucket"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

type repairDuts struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	onlyVerify    bool
	latestVersion bool
	deepRepair    bool
	bbBucket      string
	bbBuilder     string
}

// RepairDutsCmd contains repair-duts command specification
var RepairDutsCmd = &subcommands.Command{
	UsageLine: "repair-duts",
	ShortDesc: "Repair the DUT by name",
	LongDesc: `Repair the DUT by name.
	./shivas repair <dut_name1> ...
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
		return c
	},
}

const parisClientTag = "client:shivas"

// Run represent runner for reserve command
func (c *repairDuts) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *repairDuts) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	if len(args) == 0 {
		return errors.Reason("at least one hostname has to be provided").Err()
	}
	ctx := cli.GetContext(a, c, env)
	e := c.envFlags.Env()
	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions)
	if err != nil {
		return err
	}
	uc := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions,
	})
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "getting auth opts").Err()
	}
	ns, err := c.envFlags.Namespace(nil, "")
	if err != nil {
		return err
	}
	sessionTag := fmt.Sprintf("admin-session:%s", uuid.New().String())
	for _, host := range args {
		sc, err := utils.SchedukeClient(ctx, uc, authOpts, host)
		if err != nil {
			return errors.Annotate(err, "creating Scheduke client").Err()
		}
		taskURL, err := scheduleRepairBuilder(ctx, bc, sc, e, host, !c.onlyVerify, c.latestVersion, c.deepRepair, c.bbBuilder, c.bbBucket, ns, sessionTag)
		if err != nil {
			fmt.Fprintf(a.GetOut(), "%s: %s\n", host, err.Error())
		} else {
			fmt.Fprintf(a.GetOut(), "%s: %s\n", host, taskURL)
		}
	}
	utils.PrintTasksBatchLink(a.GetOut(), e.SwarmingService, sessionTag)
	return nil
}

// ScheduleRepairBuilder schedules a labpack Buildbucket builder/recipe with the necessary arguments to run repair.
func scheduleRepairBuilder(ctx context.Context, bc buildbucket.Client, sc schedulingapi.TaskSchedulingAPI, e site.Environment, host string, runRepair, latestVersion, deepRepair bool, builder string, bucket string, namespace string, adminSession string) (string, error) {
	v := buildbucket.CIPDProd
	if latestVersion {
		v = buildbucket.CIPDLatest
	}
	if !runRepair {
		builder = "verify"
	}
	task := buildbucket.Recovery
	if deepRepair {
		task = buildbucket.DeepRecovery
	}
	p := &buildbucket.Params{
		UnitName:       host,
		TaskName:       string(task),
		BuilderBucket:  bucket,
		BuilderName:    builder,
		EnableRecovery: runRepair,
		AdminService:   e.AdminService,
		// Note: UFS service is inventory service for fleet.
		InventoryService:   e.UnifiedFleetService,
		InventoryNamespace: namespace,
		UpdateInventory:    true,
		// Note: Scheduled tasks are not expected custom configuration.
		Configuration: "",
		ExtraTags: []string{
			adminSession,
			"task:recovery",
			parisClientTag,
			fmt.Sprintf("version:%s", v),
			"qs_account:unmanaged_p0",
		},
	}
	url, _, err := buildbucket.CreateTask(ctx, bc, sc, v, p, "shivas")
	return url, err
}
