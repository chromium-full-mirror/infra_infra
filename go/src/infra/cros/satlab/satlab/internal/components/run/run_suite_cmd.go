// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/errors"

	"infra/cmdsupport/cmdlib"
	common_run "infra/cros/satlab/common/run"
)

// RunCmd is the implementation of the "satlab run" command.
var RunCmd = &subcommands.Command{
	UsageLine: "run [options...]",
	ShortDesc: "execute a test or suite",
	CommandRun: func() subcommands.CommandRun {
		c := &run{}
		registerRunFlags(c)
		return c
	},
}

// run holds the arguments that are needed for the run command.
type run struct {
	runFlags
}

// Run attempts to run a test or suite and returns an exit status.
func (c *run) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	// Confirm required args are provided and no argument conflicts
	if err := c.validateArgs(); err != nil {
		fmt.Fprintln(a.GetErr(), err.Error())
		c.Flags.Usage()
		cmdlib.PrintError(a, err)
		return 1
	}
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// InnerRun is the implementation of the run command.
func (c *run) innerRun(a subcommands.Application, positionalArgs []string, env subcommands.Env) error {
	ctx := context.Background()
	var tests []string
	if c.test != "" {
		tests = append(tests, c.test)
	}
	r := &common_run.Run{
		Image:            c.image,
		Model:            c.model,
		Board:            c.board,
		Milestone:        c.milestone,
		Build:            c.build,
		Pool:             c.pool,
		Suite:            c.suite,
		Tests:            tests,
		Testplan:         c.testplan,
		TestplanLocal:    c.testplanLocal,
		Harness:          c.harness,
		TestArgs:         c.testArgs,
		SatlabId:         c.satlabId,
		Desktop:          c.desktop,
		CFT:              c.cft,
		TRV2:             c.trv2,
		DynamicTRV2:      c.dynamicTrv2,
		Local:            c.local,
		TimeoutMins:      c.timeoutMins,
		AddedDims:        c.addedDims,
		TagIncludes:      c.tagIncludes,
		TagExcludes:      c.tagExcludes,
		TestNameIncludes: c.testNameIncludes,
		TestNameExcludes: c.testNameExcludes,
	}
	buildLink, err := r.TriggerRun(ctx)
	fmt.Printf("\n-- BUILD LINK --\n%s\n\n", buildLink)
	return err
}

func (c *run) validateArgs() error {
	executionTarget := 0
	if c.testplan != "" {
		executionTarget++
	}
	if c.testplanLocal != "" {
		executionTarget++
	}
	if c.suite != "" || c.desktop {
		executionTarget++
	}
	if c.test != "" {
		executionTarget++
	}
	if executionTarget != 1 {
		return errors.Reason("Please specify only one of the following: -suite, -test, -testplan, -testplan_local").Err()
	}
	if c.cft && c.test != "" && c.harness == "" {
		return errors.Reason("-harness is required for cft test runs").Err()
	}
	if !c.cft && c.trv2 {
		return errors.Reason("-cft is required for test runner v2").Err()
	}
	if !c.cft && c.dynamicTrv2 {
		return errors.Reason("-cft is required for dynamic test runner v2").Err()
	}
	if !c.cft && c.desktop {
		return errors.Reason("-cft is required for desktop test").Err()
	}
	if c.suite == "" && !c.desktop && (len(c.tagIncludes) > 0 || len(c.tagExcludes) > 0 || len(c.testNameIncludes) > 0 || len(c.testNameExcludes) > 0) {
		return errors.Reason("-tags-includes/-tags-excludes/-test-name-includes/-test-name-excludes can run only with -suite").Err()
	}
	if c.board == "" {
		return errors.Reason("-board not specified").Err()
	}
	if c.image == "" && !c.desktop {
		if c.model == "" {
			return errors.Reason("-model must be specified if -image is not provided").Err()
		}
		if c.milestone == "" {
			return errors.Reason("-milestone must be specified if -image is not provided").Err()
		}
		if c.build == "" {
			return errors.Reason("-build must be specified if -image is not provided").Err()
		}
	}
	if c.image != "" && c.build != "" {
		return errors.Reason("-build can be specified only if -image is not provided").Err()
	}
	if c.pool == "" {
		return errors.Reason("-pool not specified").Err()
	}
	if _, ok := c.addedDims["drone"]; ok {
		return errors.Reason("-dims cannot include drone (control via -satlabId instead)").Err()
	}
	if c.maxTimeout {
		return errors.Reason("-max-timeout is deprecated").Err()
	}
	return nil
}
