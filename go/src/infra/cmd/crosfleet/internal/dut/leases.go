// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"fmt"
	"strings"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/auth/client/authcli"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	crosfleetcommon "go.chromium.org/infra/cmd/crosfleet/internal/common"
	dutinfopb "go.chromium.org/infra/cmd/crosfleet/internal/proto"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmd/crosfleet/internal/ufs"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

const leasesCmd = "leases"

var leases = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...]", leasesCmd),
	ShortDesc: "print information on the current user's active leases",
	LongDesc: `Print information on the current user's active leases.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &leasesRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.printer.Register(&c.Flags)
		return c
	},
}

type leasesRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  crosfleetcommon.EnvFlags
	printer   crosfleetcommon.CLIPrinter
}

func (c *leasesRun) Run(a subcommands.Application, _ []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		crosfleetcommon.PrintCmdError(a, err)
		return 1
	}
	return 0
}

func (c *leasesRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	uc, err := ufs.NewUFSClient(ctx, c.envFlags.Env().UFSService, &c.authFlags)
	if err != nil {
		return err
	}
	currentUser, err := crosfleetcommon.GetUserEmail(ctx, &c.authFlags)
	if err != nil {
		return err
	}

	// Flow for non-Scheduke (legacy) leases. TODO(b/332370221): Delete this.
	leasesBBClient, err := buildbucket.NewClient(ctx, c.envFlags.Env().DUTLeaserBuilder, c.envFlags.Env().BuildbucketService, c.authFlags)
	if err != nil {
		return err
	}
	fieldMask := &field_mask.FieldMask{Paths: []string{
		"builds.*.created_by",
		"builds.*.id",
		"builds.*.create_time",
		"builds.*.start_time",
		"builds.*.status",
		"builds.*.input",
		"builds.*.infra",
		"builds.*.tags",
	}}
	legacyLeaseBuilds, err := leasesBBClient.GetAllBuildsByUser(ctx, currentUser, &buildbucketpb.SearchBuildsRequest{
		Predicate: &buildbucketpb.BuildPredicate{
			Status: buildbucketpb.Status_STARTED,
		},
		Fields: fieldMask,
	})
	if err != nil {
		return err
	}
	var leaseInfoList dutinfopb.LeaseInfoList
	for _, build := range legacyLeaseBuilds {
		li := &common.LeaseInfo{Build: build}
		dutHostname := buildbucket.FindDimValInFinalDims("dut_name", build)
		allDutInfoFound := true
		if dutHostname != "" {
			li.Device, allDutInfoFound, err = getDutInfo(ctx, uc, dutHostname)
		}
		// Swallow all errors from here on, since we have partial info to print.
		c.printer.WriteTextStdout("%s\n", leaseInfoAsBashVariables(li, leasesBBClient))
		if !allDutInfoFound {
			c.printer.WriteTextStdout("Couldn't fetch complete DUT info for %s, possibly due to transient UFS RPC errors;\nrun `crosfleet dut %s` to try again", dutHostname, leasesCmd)
		}
		if err != nil {
			c.printer.WriteTextStdout("RPC error: %s", err.Error())
		}
		leaseInfoList.Leases = append(leaseInfoList.Leases, &dutinfopb.LeaseInfo{
			Build: li.Build,
			DUT: &dutinfopb.DUTInfo{
				Hostname: li.Device.Name,
				LabSetup: li.Device.LabSetup,
				Machine:  li.Device.Machine,
			}})
	}

	// Flow for Scheduke leases.
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return err
	}
	schedukeLeases, allInfoFound, err := common.Leases(ctx, authOpts, c.envFlags.UseDev())
	if err != nil {
		return err
	}
	for _, li := range schedukeLeases {
		c.printer.WriteTextStdout("%s\n", leaseInfoAsBashVariables(li, leasesBBClient))
		leaseInfoList.Leases = append(leaseInfoList.Leases, &dutinfopb.LeaseInfo{
			Build: nil,
			DUT: &dutinfopb.DUTInfo{
				Hostname: li.Device.Name,
				LabSetup: li.Device.LabSetup,
				Machine:  li.Device.Machine,
			},
		})
	}
	if !allInfoFound {
		c.printer.WriteTextStdout("Couldn't fetch complete DUT info, possibly due to transient UFS RPC errors;\nrun `crosfleet dut %s` to try again", leasesCmd)
	}
	c.printer.WriteJSONStdout(&leaseInfoList)

	return nil
}

// leaseInfoAsBashVariables returns a pretty-printed string containing info
// about the given lease formatted as bash variables. Only the variables that
// are found in the lease info proto message are printed.
func leaseInfoAsBashVariables(info *common.LeaseInfo, leasesBBClient buildbucket.Client) string {
	var bashVars []string

	build := info.Build
	if build != nil {
		bashVars = append(bashVars,
			fmt.Sprintf("LEASE_TASK=%s\nSTATUS=%s\nMINS_REMAINING=%d",
				leasesBBClient.BuildURL(build.GetId()),
				build.GetStatus(),
				getRemainingMins(build)))
	}

	device := info.Device
	if device != nil {
		bashVars = append(bashVars, dutInfoAsBashVariables(device))
	}

	return strings.Join(bashVars, "\n")
}

// getRemainingMins gets the remaining minutes on a lease from a given
// dut_leaser Buildbucket build.
func getRemainingMins(build *buildbucketpb.Build) int64 {
	inputProps := build.GetInput().GetProperties().GetFields()
	leaseLengthMins := inputProps["lease_length_minutes"].GetNumberValue()
	status := build.GetStatus()
	switch status {
	case buildbucketpb.Status_SCHEDULED:
		// Lease hasn't started; full lease length remains.
		return int64(leaseLengthMins)
	case buildbucketpb.Status_STARTED:
		// Lease has started; subtract elapsed time from lease length.
		minsElapsed := time.Since(build.GetStartTime().AsTime()).Minutes()
		return int64(leaseLengthMins - minsElapsed)
	default:
		// Lease is finished; no time remains.
		return 0
	}
}
