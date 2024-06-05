// Copyright 2024 The Chromium Authors
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

	"infra/cmd/mallet/internal/site"
	"infra/cmdsupport/cmdlib"
	"infra/cros/recovery/config"
	"infra/libs/skylab/buildbucket"
	"infra/libs/skylab/common/heuristics"
	"infra/libs/skylab/swarming"
)

// SetFwTarget creates task to update fw-targets for the DUT.
var SetFwTarget = &subcommands.Command{
	UsageLine: "set-fw-target [options] dut-name",
	ShortDesc: "Update fw-targets for DUTs.",
	CommandRun: func() subcommands.CommandRun {
		c := &SetFwTargetRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.latest, "latest", true, "Use latest version of CIPD when scheduling. By default yes.")
		c.Flags.StringVar(&c.adminSession, "admin-session", "", "Admin session used to group created tasks. By default generated.")
		c.Flags.StringVar(&c.ecTarget, "ec-target", "", "Update target for EC.")
		c.Flags.StringVar(&c.apTarget, "ap-target", "", "Update target for AP.")
		return c
	},
}

type SetFwTargetRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	latest       bool
	adminSession string

	ecTarget, apTarget string
}

func (c *SetFwTargetRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *SetFwTargetRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "internal run").Err()
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions)
	if err != nil {
		return errors.Annotate(err, "internal run").Err()
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
		url, _, err := buildbucket.ScheduleTask(
			ctx,
			bc,
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
					"task:serial-console",
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

func (c *SetFwTargetRun) createPlan() []byte {
	rc := config.SetFwTargets(c.ecTarget, c.apTarget)
	b, err := json.Marshal(rc)
	if err != nil {
		log.Fatalf("Failed to create JSON config: %v", err)
	}
	return b
}
