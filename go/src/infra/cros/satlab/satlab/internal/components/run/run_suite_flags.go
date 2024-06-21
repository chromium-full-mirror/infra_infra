// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"github.com/maruel/subcommands"

	"infra/cros/satlab/satlab/internal/flagx"
)

// runFlags holds the flags necessary for test execution
type runFlags struct {
	subcommands.CommandRunBase

	image         string
	model         string
	board         string
	milestone     string
	build         string
	pool          string
	suite         string
	test          string
	testplan      string
	testplanLocal string
	harness       string
	testArgs      string
	satlabId      string
	cft           bool
	trv2          bool
	local         bool
	maxTimeout    bool
	timeoutMins   int
	addedDims     map[string]string
}

// registerRunFlags registers the test execution flags.
func registerRunFlags(c *run) {
	c.Flags.StringVar(&c.image, "image", "", "image to run test against")
	c.Flags.StringVar(&c.suite, "suite", "", "test suite to execute")
	c.Flags.StringVar(&c.test, "test", "", "individual test to execute")
	c.Flags.StringVar(&c.testplan, "testplan", "", "path to testplan file")
	// changing the variable to match with the standard convention
	// but not changing flag name as it is now used by users
	c.Flags.StringVar(&c.testplanLocal, "testplan_local", "", "path to local testplan file")
	c.Flags.StringVar(&c.model, "model", "", "model specifies what model a test should run on")
	c.Flags.StringVar(&c.board, "board", "", "board is the board to run against")
	c.Flags.StringVar(&c.milestone, "milestone", "", "milestone of the ChromeOS image")
	c.Flags.StringVar(&c.build, "build", "", "build version of the ChromeOS image")
	c.Flags.StringVar(&c.pool, "pool", "", "pool specifies what `label-pool` dimension we should run a test on")
	c.Flags.StringVar(&c.harness, "harness", "", "test harness to use for test execution")
	c.Flags.StringVar(&c.testArgs, "testArgs", "", "test args to use for test execution")
	c.Flags.StringVar(&c.satlabId, "satlabId", "", "id of satlab box to execute tests on (e.g. 'satlab-XXXXXXXXX')")
	c.Flags.BoolVar(&c.cft, "cft", true, "whether to use CFT execution framework")
	c.Flags.BoolVar(&c.trv2, "trv2", false, "whether to use Test Runner v2")
	c.Flags.BoolVar(&c.local, "local", false, "whether to execute tests on local satlab")
	c.Flags.IntVar(&c.timeoutMins, "timeout-mins", 0, "how many minutes to time build out after")
	c.Flags.BoolVar(&c.maxTimeout, "max-timeout", false, "DEPRECATED: Use `-timeout-mins` instead")
	c.Flags.Var(flagx.MapToFlagValue(&c.addedDims), "dims", "Additional scheduling dimension in format key=val or key:val; may be specified multiple times.")
}
