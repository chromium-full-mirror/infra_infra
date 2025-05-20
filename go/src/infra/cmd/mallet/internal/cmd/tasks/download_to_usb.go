// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"

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

// Recovery subcommand: Recovering the devices.
var DownloadToUsbDrive = &subcommands.Command{
	UsageLine: "usb-download",
	ShortDesc: "Download image to servo USB-drive.",
	LongDesc:  "Download image to servo USB-drive.",
	CommandRun: func() subcommands.CommandRun {
		c := &downloadToUsbDriveRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.imageName, "image", "", "ChromeOS version name like eve-release/R86-13380.0.0")
		c.Flags.StringVar(&c.gsImagePath, "gs-path", "", "GS path to where the payloads are located. Example: gs://chromeos-image-archive/eve-release/R86-13380.0.0")
		return c
	},
}

type downloadToUsbDriveRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	imageName   string
	gsImagePath string
}

func (c *downloadToUsbDriveRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *downloadToUsbDriveRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
	if len(args) == 0 {
		return errors.Reason("create recovery task: unit is not specified").Err()
	}
	v := buildbucket.CIPDLatest
	for _, unit := range args {
		unit = heuristics.NormalizeBotNameToDeviceName(unit)
		e := c.envFlags.Env()
		configuration := b64.StdEncoding.EncodeToString(c.createPlan())
		deviceInfo, err := device.GetDeviceInfo(ctx, uc, unit)
		if err != nil {
			return errors.Annotate(err, "getting device info for hostname %s", unit).Err()
		}
		pools := deviceInfo.Pools
		if len(pools) == 0 {
			return fmt.Errorf("found no pool for device %s", unit)
		}
		sc, err := schedulers.NewSchedukeClientForCLI(ctx, pools[0], authOpts)
		if err != nil {
			return errors.Annotate(err, "initializing Scheduke client").Err()
		}
		// TODO(b:347729967): plumb sc through here.
		url, _, err := buildbucket.CreateTask(
			ctx,
			bc,
			sc,
			v,
			&buildbucket.Params{
				UnitName:         unit,
				UnitID:           deviceInfo.ID,
				TaskName:         string(buildbucket.Custom),
				AdminService:     e.AdminService,
				InventoryService: e.UFSService,
				NoMetrics:        true,
				Configuration:    configuration,
				// We do not update as this is just manual action.
				UpdateInventory: false,
				ExtraTags: []string{
					"task:download_to_usb",
					site.ClientTag,
					fmt.Sprintf("version:%s", v),
				},
			},
			"mallet",
		)
		if err != nil {
			fmt.Fprintf(a.GetErr(), "Created recovery task for %q fail: %s\n", unit, err)
		} else {
			fmt.Fprintf(a.GetOut(), "Created recovery task for %q: %s\n", unit, url)
		}
	}
	return nil
}

func (c *downloadToUsbDriveRun) createPlan() []byte {
	rc := config.DownloadImageToServoUSBDrive(c.gsImagePath, c.imageName)
	b, err := json.Marshal(rc)
	if err != nil {
		log.Fatalf("Failed to create JSON config: %v", err)
	}
	return b
}
