// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	crosfleetcommon "go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/cmd/crosfleet/internal/flagx"
	dutinfopb "go.chromium.org/infra/cmd/crosfleet/internal/proto"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmd/crosfleet/internal/ufs"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
)

const (
	// maxLeaseLengthMinutes is 24 hours in minutes.
	maxLeaseLengthMinutes = 24 * 60
	// Buildbucket priority for dut_leaser builds.
	dutLeaserBuildPriority = 15
	// leaseCmdName is the name of the `crosfleet dut lease` command.
	leaseCmdName             = "lease"
	maxLeaseReasonCharacters = 30
	poolConfigsDirURL        = "https://chrome-internal.googlesource.com/chromeos/infra/config/+/refs/heads/main/testingconfig/"
	poolLabelName            = "label-pool"
	dutIDLabelName           = "dut_id"
	boardLabelName           = "label-board"
	modelLabelName           = "label-model"
	deviceNameLabelName      = "dut_name"
)

var lease = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...]", leaseCmdName),
	ShortDesc: "lease DUT for debugging",
	LongDesc: `Lease DUT for debugging.

DUTs can be leased by Swarming dimensions or by individual DUT hostname.
Leasing by dimensions is fastest, since the first available DUT matching the
requested dimensions is reserved. 'label-board' and 'label-model' dimensions can
be specified via the -board and -model flags, respectively; other Swarming
dimensions can be specified via the freeform -dim/-dims flags.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &leaseRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.printer.Register(&c.Flags)
		c.leaseFlags.register(&c.Flags)
		return c
	},
}

type leaseRun struct {
	subcommands.CommandRunBase
	leaseFlags
	authFlags authcli.Flags
	envFlags  crosfleetcommon.EnvFlags
	printer   crosfleetcommon.CLIPrinter
}

func (c *leaseRun) Run(a subcommands.Application, _ []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		crosfleetcommon.PrintCmdError(a, err)
		return 1
	}
	return 0
}

func (c *leaseRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	if err := c.leaseFlags.validate(&c.Flags); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	swarmingBotsClient, err := newSwarmingBotsClient(ctx, c.envFlags.Env().SwarmingService, &c.authFlags)
	if err != nil {
		return err
	}
	// Set correct pool if hostname-based lease, as our default pool value may be
	// incorrect.
	if c.host != "" {
		c.printer.WriteTextStderr(`#################################################################
# NOTE: DUTs that appear free in Swarming are not always free.  #
# To obtain a DUT as quickly as possible in the future, request #
# your lease with the -pool and -dims flags only, which gives   #
# crosfleet a better chance at finding the first free DUT that  #
# is compatible with your use-case.				#
# Lease by host will internally convert to lease by dut-id      #
#################################################################`)
		c.host = heuristics.NormalizeBotNameToDeviceName(c.host)
		pool, dutID, err := hostnameToPool(ctx, swarmingBotsClient, c.host)
		if err != nil {
			return err
		}
		c.pool = pool
		c.dutID = dutID
	}

	botDims, buildTags, err := botDimsAndBuildTags(c.leaseFlags)
	if err != nil {
		return err
	}
	c.printer.WriteTextStderr("Verifying the provided DUT dimensions...")
	duts, err := countBotsWithDims(ctx, swarmingBotsClient, botDims)
	if err != nil {
		return err
	}
	if duts.Count == 0 {
		return fmt.Errorf("no matching DUTs found; please double-check the provided DUT dimensions")
	}
	c.printer.WriteTextStderr("Found %d DUT(s) (%d busy) matching the provided DUT dimensions", duts.Count, duts.Busy)
	buildProps := map[string]any{
		"lease_length_minutes": c.durationMins,
	}
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return err
	}
	useScheduke, err := common.ShouldUseScheduke(ctx, c.pool, authOpts)
	if err != nil {
		return err
	}
	leaseInfo := &common.LeaseInfo{}
	var host string
	var allInfoFound bool

	if useScheduke {
		// Scheduke flow.
		c.printer.WriteTextStderr("Requesting %d minute lease from Scheduke", c.durationMins)
		c.printer.WriteTextStderr("Waiting to confirm DUT %s request validation and print leased DUT details...", leaseCmdName)
		listDims := map[string][]string{}
		for key, val := range botDims {
			listDims[key] = []string{val}
		}
		leaseIDChan := make(chan int64)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			leaseInfo, allInfoFound, err = common.Lease(ctx, authOpts, listDims, c.durationMins, leaseIDChan)
		}()
		// Get the lease ID before the lease is obtained, in case the user wants to
		// cancel the lease.
		leaseID := <-leaseIDChan
		c.printer.WriteTextStderr("Internal Scheduke lease ID (can be used for cancellation): %d", leaseID)
		wg.Wait()
		if err != nil {
			return err
		}
		host = leaseInfo.Device.Name
	} else {
		uc, err := ufs.NewUFSClient(ctx, c.envFlags.Env().UFSService, &c.authFlags)
		if err != nil {
			return err
		}
		// Non-Scheduke (legacy) flow. TODO(b/332370221): Delete this.
		leasesBBClient, err := buildbucket.NewClient(ctx, c.envFlags.Env().DUTLeaserBuilder, c.envFlags.Env().BuildbucketService, c.authFlags)
		if err != nil {
			return err
		}
		leaseInfo.Build, err = leasesBBClient.ScheduleBuild(ctx, buildProps, botDims, buildTags, dutLeaserBuildPriority)
		if err != nil {
			return err
		}
		c.printer.WriteTextStderr("Requesting %d minute lease at %s", c.durationMins, leasesBBClient.BuildURL(leaseInfo.Build.Id))

		c.printer.WriteTextStderr("Waiting to confirm DUT %s request validation and print leased DUT details...", leaseCmdName)
		leaseInfo.Build, err = leasesBBClient.WaitForBuildStepStart(ctx, leaseInfo.Build.Id, c.leaseStartStepName())
		if err != nil {
			return err
		}
		host = buildbucket.FindDimValInFinalDims("dut_name", leaseInfo.Build)

		// Swallow any UFS errors since the lease has been secured at this point.
		leaseInfo.Device, allInfoFound, err = getDutInfo(ctx, uc, host)
		if err != nil {
			c.printer.WriteTextStderr("RPC error: %s", err.Error())
		}
	}

	// Slightly underestimate the printed end time to account for polling delay
	// while waiting for Scheduke to return the lease status.
	endTime := time.Now().Add(time.Duration(c.durationMins-1) * time.Minute).Format(time.RFC822)
	c.printer.WriteTextStdout("Leased %s until %s\n", host, endTime)
	c.printer.WriteTextStderr("%s\n", dutInfoAsBashVariables(leaseInfo.Device))
	if !allInfoFound {
		c.printer.WriteTextStderr("Couldn't fetch complete DUT info for %s, possibly due to transient UFS RPC errors;\nrun `crosfleet dut %s %s` to try again", host, infoCmdName, host)
	}
	c.printer.WriteTextStdout("Visit http://go/chromeos-lab-duts-ssh for up-to-date docs on SSHing to a leased DUT")
	c.printer.WriteJSONStdout(&dutinfopb.LeaseInfo{
		Build: leaseInfo.Build,
		DUT: &dutinfopb.DUTInfo{
			Hostname: leaseInfo.Device.Name,
			LabSetup: leaseInfo.Device.LabSetup,
			Machine:  leaseInfo.Device.Machine,
		},
	})
	return nil
}

// botDimsAndBuildTags constructs bot dimensions and Buildbucket build tags for
// a dut_leaser build from the given lease flags and optional bot ID.
func botDimsAndBuildTags(leaseFlags leaseFlags) (dims, tags map[string]string, err error) {
	dims = map[string]string{}
	tags = map[string]string{}
	if leaseFlags.host != "" {
		// Hostname-based lease.
		tags["lease-by"] = dutIDLabelName
		tags[deviceNameLabelName] = leaseFlags.host
		dims[dutIDLabelName] = leaseFlags.dutID
	} else {
		// Swarming dimension-based lease.
		dims["dut_state"] = "ready"
		if board := leaseFlags.board; board != "" {
			tags[boardLabelName] = board
			dims[boardLabelName] = board
		}
		if model := leaseFlags.model; model != "" {
			tags[modelLabelName] = model
			dims[modelLabelName] = model
		}
	}

	// Add pool regardless of whether lease is by hostname or dims.
	if pool := leaseFlags.pool; pool != "" {
		tags[poolLabelName] = pool
		dims[poolLabelName] = pool
	}
	// Add free-form dims last to allow them to override any defaults.
	for key, val := range leaseFlags.freeformDims {
		dims[key] = val
		tags[key] = val
	}

	// Add these metadata tags last to avoid being overwritten by freeform dims.
	tags[crosfleetcommon.CrosfleetToolTag] = leaseCmdName
	tags["lease-reason"] = leaseFlags.reason
	tags["qs_account"] = "leases"
	return
}

// leaseFlags contains parameters for the "dut lease" subcommand.
type leaseFlags struct {
	durationMins int64
	reason       string
	host         string
	model        string
	board        string
	pool         string
	freeformDims map[string]string
	dutID        string
}

// Registers lease-specific flags.
func (c *leaseFlags) register(f *flag.FlagSet) {
	f.Int64Var(&c.durationMins, "minutes", 60, "Duration of lease in minutes.")
	f.StringVar(&c.reason, "reason", "", fmt.Sprintf("Optional reason for leasing (limit %d characters).", maxLeaseReasonCharacters))
	f.StringVar(&c.board, "board", "", fmt.Sprintf("'%s' Swarming dimension to lease DUT by.", boardLabelName))
	f.StringVar(&c.model, "model", "", fmt.Sprintf("'%s' Swarming dimension to lease DUT by.", modelLabelName))
	f.StringVar(&c.pool, "pool", common.DefaultQuotaPool, fmt.Sprintf("'%s' Swarming dimension to lease DUT by.", poolLabelName))
	f.StringVar(&c.host, "host", "", `Hostname of an individual DUT to lease. If leasing by hostname instead of other Swarming dimensions,
and the host DUT is running another task, the lease won't start until that task completes.
Mutually exclusive with -board/-model/-dim(s).`)
	f.Var(flagx.KeyVals(&c.freeformDims), "dim", "Freeform Swarming dimension to lease DUT by, in format key=val or key:val; may be specified multiple times.")
	f.Var(flagx.KeyVals(&c.freeformDims), "dims", "Comma-separated Swarming dimensions, in same format as -dim.")
}

func (c *leaseFlags) validate(f *flag.FlagSet) error {
	var errors []string
	if !c.hasEitherHostnameOrSwarmingDims() {
		errors = append(errors, "must specify DUT dimensions (-board/-model/-dim(s)) or DUT hostname (-host), but not both")
	}
	if c.durationMins <= 0 {
		errors = append(errors, "duration should be greater than 0")
	}
	if c.durationMins > maxLeaseLengthMinutes {
		errors = append(errors, fmt.Sprintf("duration cannot exceed %d minutes (%d hours)", maxLeaseLengthMinutes, maxLeaseLengthMinutes/60))
	}
	if len(c.reason) > maxLeaseReasonCharacters {
		errors = append(errors, fmt.Sprintf("reason cannot exceed %d characters", maxLeaseReasonCharacters))
	}

	if len(errors) > 0 {
		return cmdlib.NewUsageError(*f, strings.Join(errors, "\n"))
	}
	return nil
}

// hasOnePrimaryDim verifies that the lease flags contain either a DUT hostname
// or swarming dimensions (via -board/-model/-dim(s)), but not both.
func (c *leaseFlags) hasEitherHostnameOrSwarmingDims() bool {
	hasHostname := c.host != ""
	hasSwarmingDims := c.board != "" || c.model != "" || len(c.freeformDims) > 0 || (c.pool != common.DefaultQuotaPool && c.pool != "")
	return hasHostname != hasSwarmingDims
}

func (c *leaseRun) leaseStartStepName() string {
	hours := c.durationMins / 60
	mins := c.durationMins % 60
	return fmt.Sprintf("lease DUT for %d hr %d min", hours, mins)
}
