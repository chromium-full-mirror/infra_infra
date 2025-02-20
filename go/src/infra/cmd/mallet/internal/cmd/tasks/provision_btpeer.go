// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tasks contains subcommands for mallet.
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

// ProvisionBtpeers provisions a DUTs btpeers.
var ProvisionBtpeers = &subcommands.Command{
	UsageLine: "provision-btpeer",
	ShortDesc: "Provision a DUTs btpeers with a specified image",
	CommandRun: func() subcommands.CommandRun {
		c := &provisionBtpeerCommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.imagePath, "image-path", "", "GCS path or URL to the image to use during the provisioning.")
		c.Flags.BoolVar(&c.printOnly, "print-config", false, "If we should only print the config .json to the stdout and exit.")
		return c
	},
}

type provisionBtpeerCommand struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags
	printOnly bool
	imagePath string
}

func (command *provisionBtpeerCommand) Run(app subcommands.Application, args []string, env subcommands.Env) int {
	if err := command.innerRun(app, args, env); err != nil {
		cmdlib.PrintError(app, err)
		return 1
	}
	return 0
}

func (command *provisionBtpeerCommand) innerRun(app subcommands.Application, args []string, env subcommands.Env) error {
	if err := command.verifyArgs(args); err != nil {
		return err
	}

	plan, err := json.MarshalIndent(config.ProvisionBtpeerConfig(command.imagePath), "", "\t")
	if err != nil {
		return errors.Reason("provision btpeer: failed to create JSON config: %v", err).Err()
	}
	configuration := b64.StdEncoding.EncodeToString(plan)

	if command.printOnly {
		fmt.Fprintf(app.GetOut(), "%s", plan)
		return nil
	}

	if len(args) == 0 {
		return errors.Reason("provision btpeer: no host name specified").Err()
	}
	ctx := cli.GetContext(app, command, env)
	httpClient, err := buildbucket.NewHTTPClient(ctx, &command.authFlags)
	if err != nil {
		return errors.Annotate(err, "provision btpeer").Err()
	}
	buildBucketClient, err := buildbucket.NewClient(ctx, httpClient, site.DefaultPRPCOptions)
	if err != nil {
		return errors.Annotate(err, "provision btpeer").Err()
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
					"task:provision_btpeer",
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

func (command *provisionBtpeerCommand) verifyArgs(args []string) error {
	if command.imagePath == "" {
		return errors.Reason("required argument image-path is missing").Err()
	}
	if command.printOnly {
		// hostname is not required since we are only printing the config to stdout.
		return nil
	}
	if len(args) == 0 {
		return errors.Reason("no hostname provided").Err()
	}
	return nil
}
