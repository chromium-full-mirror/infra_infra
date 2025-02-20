// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/mallet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/recovery/config"
	"go.chromium.org/infra/libs/fleet/device"
	"go.chromium.org/infra/libs/fleet/scheduling/schedulers"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// Repair CBI: Restore backup CBI contents from UFS
// go/cbi-auto-recovery-dd
var RepairCBI = &subcommands.Command{
	UsageLine: "repair-cbi",
	ShortDesc: "Restore backup CBI contents from UFS",
	CommandRun: func() subcommands.CommandRun {
		c := &cbiRepairCommandRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		return c
	},
}

type cbiRepairCommandRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags
}

func (command *cbiRepairCommandRun) Run(app subcommands.Application, args []string, env subcommands.Env) int {
	if err := command.innerRun(app, args, env); err != nil {
		cmdlib.PrintError(app, err)
		return 1
	}
	return 0
}

func (command *cbiRepairCommandRun) innerRun(app subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) == 0 {
		return errors.Reason("repair CBI: no host name specified").Err()
	}
	ctx := cli.GetContext(app, command, env)
	httpClient, err := buildbucket.NewHTTPClient(ctx, &command.authFlags)
	if err != nil {
		return errors.Annotate(err, "repair CBI").Err()
	}
	buildBucketClient, err := buildbucket.NewClient(ctx, httpClient, site.DefaultPRPCOptions)
	if err != nil {
		return errors.Annotate(err, "repair CBI").Err()
	}
	uc := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       httpClient,
		Host:    command.envFlags.Env().UFSService,
		Options: site.UFSPRPCOptions,
	})
	authOpts, err := command.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "getting auth opts").Err()
	}
	plan, err := json.Marshal(config.RecoverCBIFromInventoryConfig())
	if err != nil {
		return errors.Reason("repair CBI: failed to create JSON config: %v", err).Err()
	}
	configuration := b64.StdEncoding.EncodeToString(plan)
	for _, hostName := range args {
		hostName = heuristics.NormalizeBotNameToDeviceName(hostName)
		commandEnv := command.envFlags.Env()
		pools, err := device.GetPools(ctx, uc, hostName)
		if err != nil {
			return errors.Annotate(err, "getting pools for device %s", hostName).Err()
		}
		if len(pools) == 0 {
			return fmt.Errorf("found no pool for device %s", hostName)
		}
		sc, err := schedulers.NewSchedukeClientForCLI(ctx, pools[0], authOpts)
		if err != nil {
			return errors.Annotate(err, "initializing Scheduke client").Err()
		}
		url, _, err := buildbucket.CreateTask(
			ctx,
			buildBucketClient,
			sc,
			buildbucket.CIPDLatest,
			&buildbucket.Params{
				UnitName:         hostName,
				TaskName:         string(buildbucket.Custom),
				AdminService:     commandEnv.AdminService,
				InventoryService: commandEnv.UFSService,
				Configuration:    configuration,
				UpdateInventory:  false,
				ExtraTags: []string{
					"task:repair_cbi",
					site.ClientTag,
					fmt.Sprintf("Buildbucket version: %s", buildbucket.CIPDLatest),
				},
			},
			"mallet",
		)
		if err != nil {
			fmt.Fprintf(app.GetErr(), "Failed to create task for %q: %s\n", hostName, err)
		} else {
			fmt.Fprintf(app.GetOut(), "Sucessfully created task for %q: %s\n", hostName, url)
		}
	}
	return nil
}
