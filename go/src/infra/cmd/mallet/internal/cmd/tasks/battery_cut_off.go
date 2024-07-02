// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/mallet/internal/site"
	"infra/cmdsupport/cmdlib"
	"infra/cros/recovery/config"
	"infra/libs/fleet/device"
	"infra/libs/fleet/scheduling/schedulers"
	"infra/libs/skylab/buildbucket"
	"infra/libs/skylab/common/heuristics"
	"infra/libs/skylab/swarming"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// Recovery devices requires battery cut-off.
var BatteryCutOff = &subcommands.Command{
	UsageLine: "battery-cut-off",
	ShortDesc: "battery-cut-off describe in b/268089647#comment38",
	CommandRun: func() subcommands.CommandRun {
		c := &BatteryCutOffRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.latest, "latest", true, "Use latest version of CIPD when scheduling. By default yes.")
		c.Flags.StringVar(&c.adminSession, "admin-session", "", "Admin session used to group created tasks. By default generated.")
		return c
	},
}

type BatteryCutOffRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	latest       bool
	adminSession string
}

func (c *BatteryCutOffRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *BatteryCutOffRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "internal run").Err()
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions)
	if err != nil {
		return errors.Annotate(err, "internal run").Err()
	}
	uc := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    c.envFlags.Env().UFSService,
		Options: site.UFSPRPCOptions,
	})
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "getting auth opts").Err()
	}
	if len(args) == 0 {
		return errors.Reason("internal run: unit is not specified").Err()
	}
	v := buildbucket.CIPDProd
	if c.latest {
		v = buildbucket.CIPDLatest
	}
	// Admin session used to created common tag across created tasks.
	if c.adminSession == "" {
		c.adminSession = uuid.New().String()
	}
	sessionTag := fmt.Sprintf("admin-session:%s", c.adminSession)
	for _, unit := range args {
		unit = heuristics.NormalizeBotNameToDeviceName(unit)
		e := c.envFlags.Env()
		configuration := b64.StdEncoding.EncodeToString(c.createPlan())
		pools, err := device.GetPools(ctx, uc, unit)
		if err != nil {
			return errors.Annotate(err, "getting pools for device %s", unit).Err()
		}
		if len(pools) == 0 {
			return fmt.Errorf("found no pool for device %s", unit)
		}
		sc, err := schedulers.NewSchedukeClientForCLI(ctx, pools[0], authOpts)
		if err != nil {
			return errors.Annotate(err, "initializing Scheduke client").Err()
		}
		url, _, err := buildbucket.CreateTask(
			ctx,
			bc,
			sc,
			v,
			&buildbucket.Params{
				UnitName:         unit,
				TaskName:         string(buildbucket.Custom),
				AdminService:     e.AdminService,
				InventoryService: e.UFSService,
				NoMetrics:        false,
				NoStepper:        false,
				EnableRecovery:   true,
				Configuration:    configuration,
				UpdateInventory:  true,
				ExtraTags: []string{
					"task:batery-cut-off",
					site.ClientTag,
					fmt.Sprintf("version:%s", v),
					sessionTag,
				},
			},
			"mallet",
		)
		if err != nil {
			fmt.Fprintf(a.GetErr(), "Created task for %q fail: %s\n", unit, err)
		} else {
			fmt.Fprintf(a.GetOut(), "Created task for %q: %s\n", unit, url)
		}
	}
	// For run with more than one DUTs we provide a grouped tasks link for user to track all of them.
	if len(args) > 1 {
		fmt.Fprintf(a.GetOut(), "Created tasks: %s\n", swarming.TaskListURLForTags(c.envFlags.Env().SwarmingService, []string{sessionTag}))
	}
	return nil
}

func (c *BatteryCutOffRun) createPlan() []byte {
	rc := config.FixBatteryCutOffConfig()
	b, err := json.Marshal(rc)
	if err != nil {
		log.Fatalf("Failed to create JSON config: %v", err)
	}
	return b
}
