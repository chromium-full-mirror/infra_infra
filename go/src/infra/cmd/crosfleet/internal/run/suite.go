// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/auth/client/authcli"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/cli"
	luciflag "go.chromium.org/luci/common/flag"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmd/crosfleet/internal/ufs"
	crosbb "go.chromium.org/infra/cros/lib/buildbucket"
)

// suiteCmdName is the name of the `crosfleet run suite` command.
const suiteCmdName = "suite"

var suite = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...] SUITE_NAME", suiteCmdName),
	ShortDesc: "runs a test suite",
	LongDesc: `Launches a suite task with the given suite name.

You must supply -board and -pool.

This command does not wait for the task to start running.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &suiteRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.printer.Register(&c.Flags)
		c.Flags.BoolVar(&c.allowDupes, "allow-duplicates", false,
			"If set, will schedule all builds, including those for which an equivalent build is already pending or running.")
		c.Flags.Var(luciflag.CommaList(&c.tagIncludes), "tag-includes", `Comma-separated list of tags for listing tests to be run(only tests matching ALL of the tags)
		Tags must match exactly (i.e. no regexp, wildcard, etc. allowed).`)
		c.Flags.Var(luciflag.CommaList(&c.tagExcludes), "tag-excludes", `Comma-separated list of tags that should exclude tests tagged with any of these tags during a CTP run
		Tags must match exactly (i.e. no regexp, wildcard, etc. allowed).`)
		c.Flags.Var(luciflag.CommaList(&c.testNameIncludes), "test-name-includes", `Comma-separated list of tests to be run(only tests matching ALL of the tags.
		Test names allow usage of wildcard.`)
		c.Flags.Var(luciflag.CommaList(&c.testNameExcludes), "test-name-excludes", `Comma-separated list of tests that should be excluded from execution. These tests will be skipped during the run.
		Test names allow usage of wildcard.`)

		c.testCommonFlags.register(&c.Flags, suiteCmdName)

		return c
	},
}

type suiteRun struct {
	subcommands.CommandRunBase
	testCommonFlags
	authFlags        authcli.Flags
	envFlags         common.EnvFlags
	printer          common.CLIPrinter
	allowDupes       bool
	tagIncludes      []string
	tagExcludes      []string
	testNameIncludes []string
	testNameExcludes []string
}

func (c *suiteRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	bbService := c.envFlags.Env().BuildbucketService

	ctpBuilder := c.getCTPBuilder(c.envFlags.Env())
	ctpBBClient, err := buildbucket.NewClient(ctx, ctpBuilder, bbService, c.authFlags)
	if err != nil {
		common.PrintCmdError(a, err)
		return 1
	}

	ufsClient, err := ufs.NewUFSClient(ctx, c.envFlags.Env().UFSService, &c.authFlags)
	if err != nil {
		common.PrintCmdError(a, err)
		return 2
	}

	if err := c.innerRun(a, args, ctx, ctpBBClient, ufsClient); err != nil {
		common.PrintCmdError(a, err)
		return 3
	}
	return 0
}

func (c *suiteRun) innerRun(a subcommands.Application, args []string, ctx context.Context, ctpBBClient buildbucket.Client, ufsClient ufs.Client) error {
	bbService := c.envFlags.Env().BuildbucketService
	if err := c.validateAndAutocompleteFlags(ctx, &c.Flags, args, suiteCmdName, bbService, c.authFlags, c.printer); err != nil {
		return err
	}

	fleetValidationResults, err := c.verifyFleetTestsPolicy(ctx, ufsClient, suiteCmdName, args, true)
	if err != nil {
		return err
	}
	if err = checkAndPrintFleetValidationErrors(*fleetValidationResults, c.printer, suiteCmdName); err != nil {
		return err
	}
	if fleetValidationResults.testValidationErrors != nil {
		c.models = fleetValidationResults.validModels
		args = fleetValidationResults.validTests
	}

	testLauncher := ctpRunLauncher{
		mainArgsTag: testOrSuiteNamesTag(args),
		printer:     c.printer,
		cmdName:     suiteCmdName,
		bbClient:    ctpBBClient,
		testPlan:    testPlanForSuites(args, c.tagIncludes, c.tagExcludes, c.testNameIncludes, c.testNameExcludes, c.enableAutotestSharding, c.testArgs, c.maxInShard),
		cliFlags:    &c.testCommonFlags,
	}

	if !c.allowDupes {
		hasModels, filteredModels, err := c.dedupeRequests(ctx, &testLauncher, ctpBBClient, args, c.models)
		if err != nil {
			return err
		}
		if !hasModels {
			return nil
		}
		c.models = filteredModels
	}
	return testLauncher.launchAndOutputTests(ctx)
}

// testPlanForSuites constructs a Test Platform test plan for the given tests.
func testPlanForSuites(suiteNames []string, tagIncludes []string, tagExcludes []string, testNameIncludes []string, testNameExcludes []string, enableAutotestSharding bool, testArgs string, maxInShard int64) *test_platform.Request_TestPlan {
	testPlan := test_platform.Request_TestPlan{MaxInShard: maxInShard}
	for _, suiteName := range suiteNames {
		suiteRequest := &test_platform.Request_Suite{Name: suiteName, TestArgs: testArgs}
		testPlan.Suite = append(testPlan.Suite, suiteRequest)
	}

	if len(tagIncludes) > 0 || len(tagExcludes) > 0 || len(testNameIncludes) > 0 || len(testNameExcludes) > 0 {
		testCaseTagCriteria := &api.TestSuite_TestCaseTagCriteria{
			Tags:             tagIncludes,
			TagExcludes:      tagExcludes,
			TestNames:        testNameIncludes,
			TestNameExcludes: testNameExcludes,
		}
		testPlan.TagCriteria = testCaseTagCriteria
	}
	testPlan.EnableAutotestSharding = enableAutotestSharding
	return &testPlan
}

// dedupeRequests filters out the models for which an unfinished CTP request already exists,
// returning:
// * whether or not there are any runs to schedule
// * the models to schedule for
// * optional error
func (c *suiteRun) dedupeRequests(ctx context.Context, runToLaunch *ctpRunLauncher, bbClient buildbucket.Client, args []string, models []string) (bool, []string, error) {
	mainArgsTag := testOrSuiteNamesTag(args)
	searchTags := c.testCommonFlags.buildTagsForCTPBuilds(suiteCmdName, mainArgsTag)
	searchTags["label-suite"] = mainArgsTag
	searchTags["label-image"] = c.testCommonFlags.image

	searchModels := models
	if len(searchModels) == 0 {
		searchModels = []string{""}
	}
	var filteredModels []string
	for _, model := range searchModels {
		if len(model) != 0 {
			searchTags["label-model"] = model
		}

		incompleteBuilds, err := bbClient.GetIncompleteBuildsWithTags(ctx, searchTags)
		if err != nil {
			return false, nil, err
		}

		// Searching just by tags casts too wide of a net -- filter by the actual
		// CTP request passed in input properties.

		// Get the test request we're using for the new build.
		ctpBuildToLaunch := runToLaunch.ctpBuilder(model)
		request, err := ctpBuildToLaunch.TestPlatformRequest(ctpBuildToLaunch.TestRunnerTags())
		if err != nil {
			return false, nil, err
		}

		// TODO(b/271462223): Dedupe irrespective of qs_account.
		var duplicateBuilds []*buildbucketpb.Build
		for _, build := range incompleteBuilds {
			if hasRequest, err := buildHasRequest(build, request); err != nil {
				return false, nil, err
			} else if hasRequest {
				duplicateBuilds = append(duplicateBuilds, build)
			}
		}

		if len(duplicateBuilds) != 0 {
			runningBuilds := make([]string, len(duplicateBuilds))
			for i, build := range duplicateBuilds {
				runningBuilds[i] = strconv.FormatInt(build.Id, 10)
			}
			modelText := ""
			if model != "" {
				modelText = fmt.Sprintf(" for model \"%s\"", model)
			}
			c.printer.WriteTextStdout("Found existing run(s) %s%s, won't run a new one.",
				strings.Join(runningBuilds, ","), modelText)
			continue
		}
		filteredModels = append(filteredModels, model)
	}
	if len(models) == 0 {
		return len(filteredModels) != 0, []string{}, nil
	}
	return len(filteredModels) != 0, filteredModels, nil
}

func interfaceToStrSlice(arr []interface{}) []string {
	strArr := make([]string, len(arr))
	for i, v := range arr {
		strArr[i] = fmt.Sprintf("%v", v)
	}
	return strArr
}

func sortTags(request *structpb.Struct) error {
	if v, ok := crosbb.GetProp(request.AsMap(), "params.decorations.tags"); ok {
		tags, ok := v.([]interface{})
		if !ok {
			return fmt.Errorf("Could not convert tags to []interface{}.")
		}
		strTags := interfaceToStrSlice(tags)
		sort.Strings(strTags)
		if err := crosbb.SetProperty(request, "params.decorations.tags", strTags); err != nil {
			return err
		}
	}
	return nil
}

// buildHasRequest checks whether the build has the given request.
func buildHasRequest(build *buildbucketpb.Build, request *test_platform.Request) (bool, error) {
	r, err := common.ProtoToStructVal(request.ProtoReflect().Interface())
	if err != nil {
		return false, err
	}
	requestStruct := r.GetStructValue()
	buildRequests := build.GetInput().GetProperties().GetFields()["requests"].GetStructValue().GetFields()

	for _, request := range buildRequests {
		buildRequestStruct := request.GetStructValue()
		// Need to sort tags.
		if err := sortTags(requestStruct); err != nil {
			return false, err
		}
		if err := sortTags(buildRequestStruct); err != nil {
			return false, err
		}

		if reflect.DeepEqual(requestStruct.AsMap(), buildRequestStruct.AsMap()) {
			return true, nil
		}
	}
	return false, nil
}
