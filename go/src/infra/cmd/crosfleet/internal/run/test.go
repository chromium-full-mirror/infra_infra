// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmd/crosfleet/internal/ufs"
)

// testCmdName is the name of the `crosfleet run test` command.
const testCmdName = "test"

var test = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...] TEST_NAME [TEST_NAME...]", testCmdName),
	ShortDesc: "runs an individual test",
	LongDesc: `Launches an individual test task with the given test name.

You must supply -board, -pool, and -harness.

This command does not wait for the task to start running.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &testRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.printer.Register(&c.Flags)
		c.testCommonFlags.register(&c.Flags, testCmdName)
		return c
	},
}

type testRun struct {
	subcommands.CommandRunBase
	testCommonFlags
	authFlags authcli.Flags
	envFlags  common.EnvFlags
	printer   common.CLIPrinter
}

func (c *testRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		common.PrintCmdError(a, err)
		return 1
	}
	return 0
}

func (c *testRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	bbService := c.envFlags.Env().BuildbucketService
	ctx := cli.GetContext(a, c, env)
	if err := c.validateAndAutocompleteFlags(ctx, &c.Flags, args, testCmdName, bbService, c.authFlags, c.printer); err != nil {
		return err
	}

	ctpBuilder := c.getCTPBuilder(c.envFlags.Env())
	ctpBBClient, err := buildbucket.NewClient(ctx, ctpBuilder, bbService, c.authFlags)
	if err != nil {
		return err
	}

	ufsClient, err := ufs.NewUFSClient(ctx, c.envFlags.Env().UFSService, &c.authFlags)
	if err != nil {
		return err
	}

	fleetValidationResults, err := c.verifyFleetTestsPolicy(ctx, ufsClient, testCmdName, args, true)
	if err != nil {
		return err
	}
	if err = checkAndPrintFleetValidationErrors(*fleetValidationResults, c.printer, testCmdName); err != nil {
		return err
	}
	if fleetValidationResults.testValidationErrors != nil {
		c.models = fleetValidationResults.validModels
		args = fleetValidationResults.validTests
	}

	testLauncher := ctpRunLauncher{
		mainArgsTag: testOrSuiteNamesTag(args),
		printer:     c.printer,
		cmdName:     testCmdName,
		bbClient:    ctpBBClient,
		testPlan:    testPlanForTests(c.testArgs, c.testCommonFlags.testHarness, args, c.maxInShard),
		cliFlags:    &c.testCommonFlags,
	}
	return testLauncher.launchAndOutputTests(ctx)
}

// testPlanForTests constructs a Test Platform test plan for the given tests.
func testPlanForTests(testArgs string, testHarness string, testNames []string, maxInShard int64) *test_platform.Request_TestPlan {
	// Due to crbug/984103, the first autotest arg gets dropped somewhere between here and
	// when autotest reads the args. Add a dummy arg to prevent this bug for now.
	// TODO(crbug/984103): Remove the dummy arg once the underlying bug is fixed.
	if testArgs != "" {
		testArgs = "dummy=crbug/984103 " + testArgs
	}
	testPlan := &test_platform.Request_TestPlan{MaxInShard: maxInShard}
	for _, testName := range testNames {
		if testHarness != "" {
			testName = testHarness + "." + testName
		}
		testRequest := &test_platform.Request_Test{
			Harness: &test_platform.Request_Test_Autotest_{
				Autotest: &test_platform.Request_Test_Autotest{
					Name:     testName,
					TestArgs: testArgs,
				},
			},
		}
		testPlan.Test = append(testPlan.Test, testRequest)
	}
	return testPlan
}
