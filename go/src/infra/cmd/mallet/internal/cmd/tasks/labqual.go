// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"errors"
	"fmt"
	"log"

	"github.com/maruel/subcommands"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/cmd/mallet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/libs/fleet/buildbucket"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

var Labqual = &subcommands.Command{
	UsageLine: "labqual [FLAGS...] HOSTNAME [HOSTNAME...]",
	ShortDesc: "Run a lab qualification on given host(s) using the board's stable build",
	CommandRun: func() subcommands.CommandRun {
		c := &LabqualRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.imagePath, "image", "", "Image to use for qualification")
		return c
	},
}

type LabqualRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	imagePath string
}

func (c *LabqualRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *LabqualRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {

	ctx := cli.GetContext(a, c, env)

	stdErrLog := log.New(a.GetErr(), "", 0)

	if len(args) == 0 {
		return cmdlib.NewUsageError(c.Flags, "Expected at least 1 hostname after flags")
	}

	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	md := metadata.Pairs(ufsUtil.Namespace, "os")
	ctx = metadata.NewOutgoingContext(ctx, md)

	invClient := fleet.NewInventoryPRPCClient(&prpc.Client{
		C:       hc,
		Host:    c.envFlags.Env().AdminService,
		Options: site.DefaultPRPCOptions,
	})

	ufsClient := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    c.envFlags.Env().UFSService,
		Options: site.UFSPRPCOptions,
	})

	bbClient, err := buildbucket.NewBuildBucketClient(ctx, hc)
	if err != nil {
		return fmt.Errorf("failed to create BuildBucket client: %w", err)
	}

	runErrs := []error{}

	for _, unit := range args {

		host := heuristics.NormalizeBotNameToDeviceName(unit)

		_, _ = fmt.Fprintf(a.GetOut(), "Requesting labqual run for host %q\n", host)

		stableReq := &fleet.GetRecoveryVersionRequest{
			DeviceType: "cros",
			DeviceName: host,
		}

		out, err := protojson.Marshal(stableReq)
		if err != nil {
			err = fmt.Errorf("failed to marshal stable version request for host %q: %w", host, err)
			runErrs = append(runErrs, err)
			continue
		}

		stdErrLog.Printf("Stable version request: %s\n", out)

		stableResp, err := invClient.GetRecoveryVersion(ctx, stableReq)
		if err != nil {
			err = fmt.Errorf("failed to get stable version for host %q: %w", host, err)
			runErrs = append(runErrs, err)
			continue
		}

		stdErrLog.Printf("Stable version response: %+v", stableResp)

		lseResponse, err := ufsClient.GetMachineLSE(ctx, &ufsAPI.GetMachineLSERequest{
			Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, host),
		})
		if err != nil {
			err = fmt.Errorf("failed to retrieve MachineLSE for host %q: %w", host, err)
			runErrs = append(runErrs, err)
			continue
		}

		if len(lseResponse.GetMachines()) < 1 {
			runErrs = append(runErrs, fmt.Errorf("machineLSE of host %q has no assets associated", host))
			continue
		}
		machine := lseResponse.GetMachines()[0]

		assetResponse, err := ufsClient.GetAsset(ctx, &ufsAPI.GetAssetRequest{
			Name: ufsUtil.AddPrefix(ufsUtil.AssetCollection, machine),
		})
		if err != nil {
			err = fmt.Errorf("failed to retrieve asset %q for host %q: %w", machine, host, err)
			runErrs = append(runErrs, err)
			continue
		}

		board := assetResponse.GetInfo().GetBuildTarget()
		dutId := assetResponse.GetInfo().GetAssetTag()

		var dims = make(map[string]string)

		dims["dut_id"] = dutId

		if c.imagePath == "" {
			stdErrLog.Print("No image provided, using lab stable configuration image")
			c.imagePath = board + "-release/" + stableResp.GetVersion().GetOsVersion()
		}

		pools := lseResponse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools()
		if len(pools) == 0 {
			runErrs = append(runErrs, fmt.Errorf("machineLSE of host %q has no pools associated", host))
			continue
		}

		cmd := buildbucket.Run{
			Board:       board,
			Pool:        pools[0],
			Image:       c.imagePath,
			Priority:    50,
			TimeoutMins: 2330,
			CFT:         true,
			IsProd:      true,
			Harness:     "tauto",
			Tests:       []string{"tast.generic"},
			AddedDims:   dims,
			// Expected string: tast_expr=\\(\\\"group:labqual_stable\\\"\\) tast.firmware.firmwarePath=firmware-dedede-13606.B-branch-firmware/R89-13606.597.0/dedede
			TestArgs: fmt.Sprintf("tast_expr=\\(\\\"group:labqual_stable\\\"\\) tast.firmware.firmwarePath=%s", stableResp.GetVersion().GetFirmwareRoImagePath()),
			BBClient: bbClient.BuildBucketClient,
		}
		url, err := cmd.TriggerRun(ctx)
		if err != nil {
			err = fmt.Errorf("failed to start labqual on host %q: %w", host, err)
			runErrs = append(runErrs, err)
			continue
		}
		_, _ = fmt.Fprintf(a.GetOut(), "Starting labqual on host %q: %s\n", host, url)
	}

	return errors.Join(runErrs...)
}
