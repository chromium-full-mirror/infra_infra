// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"context"
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
	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
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
	uc := ufsAPI.NewFleetPRPCClient(&prpc.Client{
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
	for _, host := range args {
		sc, err := utils.SchedukeClient(ctx, uc, authOpts, host)
		if err != nil {
			fmt.Fprintf(a.GetErr(), "%s: failed to create Scheduke client %s\n", host, err)
			continue
		}
		if url, _, err := c.scheduleReserveBuilder(ctx, bc, sc, e, host, ns); err != nil {
			fmt.Fprintf(a.GetErr(), "%s: fail with %s\n", host, err)
		} else {
			fmt.Fprintf(a.GetErr(), "%s: %s\n", host, url)
		}
	}
	utils.PrintTasksBatchLink(a.GetErr(), e.SwarmingService, c.session)
	return nil
}

// scheduleReserveBuilder schedules a labpack Buildbucket builder/recipe with the necessary arguments to run reserve.
func (c *reserveDuts) scheduleReserveBuilder(ctx context.Context, bc buildbucket.Client, sc schedulingapi.TaskSchedulingAPI, e site.Environment, host, namespace string) (string, int64, error) {
	// TODO(b/229896419): refactor to hide labpack.Params struct.
	v := buildbucket.CIPDProd
	tags := []string{
		c.session,
		"task:reserve",
		parisClientTag,
		fmt.Sprintf("version:%s", v),
		fmt.Sprintf("comment:%s", c.comment),
		"qs_account:unmanaged_p0",
	}
	if user, err := user.Current(); err == nil && user != nil && user.Username != "" {
		tags = append(tags, fmt.Sprintf("user:%s", user.Username))
	}
	p := &buildbucket.Params{
		UnitName:     host,
		TaskName:     string(buildbucket.Custom),
		BuilderName:  "reserve",
		AdminService: e.AdminService,
		// NOTE: We use the UFS service, not the Inventory service here.
		InventoryService:   e.UnifiedFleetService,
		InventoryNamespace: namespace,
		NoStepper:          false,
		NoMetrics:          false,
		UpdateInventory:    true,
		Configuration:      c.config,
		ExtraTags:          tags,
	}
	url, taskID, err := buildbucket.CreateTask(ctx, bc, sc, v, p, "shivas")
	return url, taskID, errors.Annotate(err, "scheduleReserveBuilder").Err()
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
