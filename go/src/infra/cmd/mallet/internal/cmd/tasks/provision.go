// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/mallet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/libs/fleet/device"
	"go.chromium.org/infra/libs/fleet/scheduling/schedulers"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	"go.chromium.org/infra/libs/skylab/swarming"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// Recovery subcommand: Recovering the devices.
var CustomProvision = &subcommands.Command{
	UsageLine: "provision DUT1 DUT2 DUT3 ...",
	ShortDesc: "Quick provision ChromeOS device(s).",
	LongDesc:  "Quick provision ChromeOS device(s). Tool allows provide custom values for provisioning.",
	CommandRun: func() subcommands.CommandRun {
		c := &customProvisionRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.osName, "os", "", "ChromeOS version name like eve-release/R86-13380.0.0")
		c.Flags.StringVar(&c.osPath, "os-path", "", "GS path to where the payloads are located. Example: gs://chromeos-image-archive/eve-release/R86-13380.0.0")
		c.Flags.StringVar(&c.adminSession, "admin-session", "", "Admin session used to group created tasks. By default generated.")
		c.Flags.BoolVar(&c.noReboot, "no-reboot", false, "prevent reboot during the provision.")
		c.Flags.BoolVar(&c.latest, "latest", false, "Use latest version of CIPD when scheduling. By default no.")
		return c
	},
}

type customProvisionRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	osName       string
	osPath       string
	adminSession string
	noReboot     bool
	latest       bool
}

func (c *customProvisionRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *customProvisionRun) validateInput(args []string) error {
	if len(args) == 0 {
		return errors.Reason("Validate input: No target unit(s) specified").Err()
	}
	if c.osName != "" && c.osPath != "" {
		return errors.Reason("Validate input: Both os name and os path are specified, you must specify only one of them at a time.").Err()
	}
	return nil
}

func (c *customProvisionRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	hc, err := buildbucket.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "custom provision run").Err()
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions)
	if err != nil {
		return errors.Annotate(err, "custom provision run").Err()
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
	if err := c.validateInput(args); err != nil {
		return errors.Annotate(err, "custom provision run").Err()
	}
	// Admin session used to created common tag across created tasks.
	if c.adminSession == "" {
		c.adminSession = uuid.New().String()
	}
	sessionTag := fmt.Sprintf("admin-session:%s", c.adminSession)
	e := c.envFlags.Env()
	v := buildbucket.CIPDProd
	if c.latest {
		v = buildbucket.CIPDLatest
	}
	plan, err := c.createPlan()
	if err != nil {
		return errors.Annotate(err, "custom provision run").Err()
	}
	configuration := b64.StdEncoding.EncodeToString([]byte(plan))
	for _, unit := range args {
		unit = heuristics.NormalizeBotNameToDeviceName(unit)
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
				Configuration:    configuration,
				// We do not update as this is just manual action.
				UpdateInventory: false,
				ExtraTags: []string{
					sessionTag,
					"task:custom_provision",
					site.ClientTag,
					fmt.Sprintf("version:%s", v),
				},
			},
			"mallet",
		)
		if err != nil {
			return errors.Annotate(err, "create provision task").Err()
		}
		fmt.Fprintf(a.GetOut(), "Created provision task for %s: %s\n", unit, url)
	}
	fmt.Fprintf(a.GetOut(), "Created tasks: %s\n", swarming.TaskListURLForTags(e.SwarmingService, []string{sessionTag}))
	return nil
}

// Custom plan to execute provision
// TODO(otabek): Replace by build plan on fly.
const customProvisionPlanStart = `
{
	"plan_names": [
		"cros"
	],
	"plans": {
		"cros": {
			"critical_actions": [
				"cros_ssh",
				"Custom provision"
			],
			"actions": {
				"cros_ssh": {
					"dependencies": [
						"dut_has_name",
						"dut_has_board_name",
						"dut_has_model_name",
						"cros_ping"
					],
					"exec_name": "cros_ssh"
				},
				"Custom provision": {
					"docs": [
						"Provision device to the custom os version"
					],
					"exec_name": "cros_provision",
					"exec_extra_args": `
const customProvisionPlanTail = `,
					"exec_timeout": {
						"seconds": 3600
					}
				}
			}
		}
	}
}`

func (c *customProvisionRun) createPlan() (string, error) {
	customArg := []string{}
	if c.osPath != "" {
		customArg = append(customArg, fmt.Sprintf("os_image_path:%s", c.osPath))
	} else if c.osName != "" {
		customArg = append(customArg, fmt.Sprintf("os_name:%s", c.osName))
	}
	if c.noReboot {
		customArg = append(customArg, "no_reboot")
	}
	if len(customArg) > 0 {
		j, err := json.Marshal(customArg)
		if err != nil {
			return "", errors.Annotate(err, "create plan").Err()
		}
		return fmt.Sprintf("%s%s%s", customProvisionPlanStart, string(j), customProvisionPlanTail), nil
	} else {
		return fmt.Sprintf("%s%s%s", customProvisionPlanStart, "[]", customProvisionPlanTail), nil
	}
}
