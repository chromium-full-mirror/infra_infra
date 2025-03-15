// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	b64 "encoding/base64"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/mallet/internal/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/recovery/namespace"
	"go.chromium.org/infra/libs/fleet/device"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	"go.chromium.org/infra/libs/skylab/swarming"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// Recovery subcommand: recover the devices.
var Recovery = &subcommands.Command{
	UsageLine: "recovery",
	ShortDesc: "Recovery the DUT",
	LongDesc:  "Recovery the DUT.",
	CommandRun: func() subcommands.CommandRun {
		c := &recoveryRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.onlyVerify, "only-verify", false, "Block recovery actions and run only verifiers.")
		c.Flags.StringVar(&c.configFile, "config", "", "Path to the custom json config file.")
		c.Flags.BoolVar(&c.noStepper, "no-stepper", false, "Block steper from using. This will prevent by using steps and you can only see logs.")
		c.Flags.BoolVar(&c.useCsa, "use-csa", true, "Use CSA Service or not.")
		c.Flags.BoolVar(&c.disableCft, "disable-cft", false, "Disable CFT.")
		c.Flags.BoolVar(&c.deployTask, "deploy", false, "Run deploy task. By default run recovery task.")
		c.Flags.StringVar(&c.taskName, "task-name", "", `What type of task name to use.".`)
		c.Flags.BoolVar(&c.updateUFS, "update-ufs", false, "Update result to UFS. By default no.")
		c.Flags.BoolVar(&c.latest, "latest", false, "Use latest version of CIPD when scheduling. By default no.")
		c.Flags.StringVar(&c.adminSession, "admin-session", "", "Admin session used to group created tasks. By default generated.")
		c.Flags.StringVar(&c.bbBucket, "bucket", "", "Buildbucket bucket to use.")
		c.Flags.StringVar(&c.bbBuilder, "builder", "repair", "Buildbucket builder to use.")
		return c
	},
}

type recoveryRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	onlyVerify   bool
	noStepper    bool
	useCsa       bool
	configFile   string
	deployTask   bool
	taskName     string
	updateUFS    bool
	latest       bool
	adminSession string
	disableCft   bool
	bbBucket     string
	bbBuilder    string
}

func (c *recoveryRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *recoveryRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)

	ns := c.envFlags.Namespace()
	ctx = namespace.Set(ctx, ns)

	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "recovery run").Err()
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions)
	if err != nil {
		return err
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    c.envFlags.Env().UFSService,
		Options: site.UFSPRPCOptions,
	})
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "getting auth opts").Err()
	}
	if len(args) == 0 {
		return errors.Reason("create recovery task: unit is not specified").Err()
	}
	// Admin session used to created common tag across created tasks.
	if c.adminSession == "" {
		c.adminSession = uuid.New().String()
	}
	sessionTag := fmt.Sprintf("admin-session:%s", c.adminSession)
	e := c.envFlags.Env()
	for _, unitName := range args {
		unitName = heuristics.NormalizeBotNameToDeviceName(unitName)
		var configuration string
		if c.configFile != "" {
			b, err := os.ReadFile(c.configFile)
			if err != nil {
				return errors.Annotate(err, "create recovery task: open configuration file").Err()
			}
			configuration = b64.StdEncoding.EncodeToString(b)
		}
		taskName := c.getTaskName()
		if taskName == "" {
			return errors.Reason("Wrong task name %s for host %s", c.taskName, unitName).Err()
		}

		pools, err := device.GetPools(ctx, ic, unitName)
		if err != nil {
			return errors.Annotate(err, "getting pools for device %s", unitName).Err()
		}
		if len(pools) == 0 {
			return fmt.Errorf("found no pool for device %s", unitName)
		}

		hive := ufsUtil.GetHiveForDut(unitName, utils.GetHive(ctx, ic, unitName))
		realBuilderName := buildbucket.BuilderNamePerHive(c.bbBuilder, hive)
		adminParams, err := utils.PrepareAdminParams(ctx, unitName, realBuilderName, e.AdminService, ic, authOpts)
		if err != nil {
			return errors.Annotate(err, "creating Scheduling client").Err()
		}
		if !c.useCsa {
			adminParams.AdminService = ""
		}

		url, _, err := buildbucket.CreateTask(
			ctx,
			bc,
			adminParams.SchedukeClient,
			buildbucket.CipdVersion(c.latest),
			&buildbucket.Params{
				UnitName:           unitName,
				TaskName:           c.getTaskName(),
				BuilderName:        realBuilderName,
				BuilderBucket:      c.bbBucket,
				EnableRecovery:     !c.onlyVerify,
				AdminService:       adminParams.AdminService,
				InventoryService:   e.UFSService,
				InventoryNamespace: adminParams.ContextNamespace,
				UpdateInventory:    c.updateUFS,
				NoStepper:          c.noStepper,
				NoMetrics:          false,
				Configuration:      configuration,
				DisableCft:         c.disableCft,
				ExtraTags: []string{
					sessionTag,
					fmt.Sprintf("task:%s", taskName),
					site.ClientTag,
					fmt.Sprintf("version:%s", buildbucket.CipdVersion(c.latest)),
					"qs_account:unmanaged_p0",
				},
			},
			"mallet",
		)
		if err != nil {
			return errors.Annotate(err, "create recovery task").Err()
		}
		fmt.Fprintf(a.GetOut(), "Created recovery task for %s: %s\n", unitName, url)
	}
	fmt.Fprintf(a.GetOut(), "Created tasks: %s\n", swarming.TaskListURLForTags(e.SwarmingService, []string{sessionTag}))
	return nil
}

func (c *recoveryRun) getTaskName() string {
	task := string(buildbucket.Recovery)
	if c.deployTask {
		task = string(buildbucket.Deploy)
	} else if c.taskName != "" {
		tn, err := buildbucket.NormalizeTaskName(c.taskName)
		if err != nil {
			return ""
		}
		task = string(tn)
	}
	return task
}
