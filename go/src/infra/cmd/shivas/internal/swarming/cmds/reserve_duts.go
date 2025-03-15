// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/user"

	"github.com/google/uuid"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cros/recovery/config"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

type reserveDuts struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	comment        string
	session        string
	expirationMins int
	// Configuration for reserve task.
	config string
}

// ReserveDutsCmd contains reserve-dut command specification
var ReserveDutsCmd = &subcommands.Command{
	UsageLine: "reserve-duts [-comment {comment}] [-session {admin-session}] [-expiration-mins 120] {HOST...}",
	ShortDesc: "Reserve the DUT by name",
	LongDesc: `Reserve the DUT by name.
	./shivas reserve <dut_name>
	Schedule a swarming Reserve task to the DUT to set the state to RESERVED to prevent scheduling tasks and tests to the DUT.
	Reserved DUT does not have expiration time and can be changed by scheduling any admin task on it.`,
	CommandRun: func() subcommands.CommandRun {
		c := &reserveDuts{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.IntVar(&c.expirationMins, "expiration-mins", 120, "The expiration minutes of the repair request.")
		c.Flags.StringVar(&c.comment, "comment", "", "The comment for reserved devices.")
		c.Flags.StringVar(&c.session, "session", "", "The admin session to group the tasks.")
		return c
	},
}

// Run represent runner for reserve command
func (c *reserveDuts) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *reserveDuts) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) == 0 {
		return errors.Reason("at least one hostname has to be provided").Err()
	}
	if c.comment == "" {
		return errors.Reason("please specify the reason in the comment").Err()
	}
	if err := c.initConfig(); err != nil {
		return err
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
	if c.session == "" {
		c.session = uuid.New().String()
	}
	c.session = fmt.Sprintf("admin-session:%s", c.session)
	for _, unitName := range args {
		hive := ufsUtil.GetHiveForDut(unitName, utils.GetHive(ctx, ic, unitName))
		realBuilderName := buildbucket.BuilderNamePerHive(utils.ReserveBuilderName, hive)
		adminParams, err := utils.PrepareAdminParams(ctx, unitName, realBuilderName, e.AdminService, ic, authOpts)
		if err != nil {
			fmt.Fprintf(a.GetErr(), "%s: failed to create Scheduke client %s\n", unitName, err)
			continue
		}

		tags := []string{
			c.session,
			"task:reserve",
			utils.ShivasClientTag,
			fmt.Sprintf("version:%s", buildbucket.CIPDProd),
			fmt.Sprintf("comment:%s", c.comment),
			"qs_account:unmanaged_p0",
		}
		if user, err := user.Current(); err == nil && user != nil && user.Username != "" {
			tags = append(tags, fmt.Sprintf("user:%s", user.Username))
		}

		url, _, err := buildbucket.CreateTask(
			ctx,
			bc,
			adminParams.SchedukeClient,
			buildbucket.CIPDProd,
			&buildbucket.Params{
				UnitName:           unitName,
				TaskName:           string(buildbucket.Custom),
				BuilderName:        realBuilderName,
				AdminService:       adminParams.AdminService,
				InventoryService:   e.UnifiedFleetService,
				InventoryNamespace: adminParams.ContextNamespace,
				NoStepper:          false,
				NoMetrics:          false,
				UpdateInventory:    true,
				Configuration:      c.config,
				ExtraTags:          tags,
			},
			"shivas",
		)

		if err != nil {
			fmt.Fprintf(a.GetErr(), "%s: fail with %s\n", unitName, err)
		} else {
			fmt.Fprintf(a.GetErr(), "%s: %s\n", unitName, url)
		}
	}
	utils.PrintTasksBatchLink(a.GetErr(), e.SwarmingService, c.session)
	return nil
}

// initConfig initializes config used for scheduling reserve tasks.
func (c *reserveDuts) initConfig() error {
	rc := config.ReserveDutConfig()
	jsonByte, err := json.Marshal(rc)
	if err != nil {
		return errors.Annotate(err, "initConfig json err:").Err()
	}
	c.config = base64.StdEncoding.EncodeToString(jsonByte)
	return nil
}
