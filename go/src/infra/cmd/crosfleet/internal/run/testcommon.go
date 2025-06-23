// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/ctp/builder"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/auth/client/authcli"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	luciflag "go.chromium.org/luci/common/flag"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/cmd/crosfleet/internal/flagx"
	crosfleetpb "go.chromium.org/infra/cmd/crosfleet/internal/proto"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmd/crosfleet/internal/ufs"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

const (
	// DefaultSwarmingPriority is the default priority for a Swarming task.
	DefaultSwarmingPriority = int64(140)
	// MinSwarmingPriority is the lowest-allowed priority for a Swarming task.
	MinSwarmingPriority = int64(50)
	// MaxSwarmingPriority is the highest-allowed priority for a Swarming task.
	MaxSwarmingPriority = int64(255)
	// defaultImageBucket is the GS bucket for the ChromeOS image archive.
	defaultImageBucket = "chromeos-image-archive"
	// containerMetadataURLSuffix is the URL suffix for the container metadata
	// URL in the ChromeOS image archive.
	containerMetadataURLSuffix = "metadata/containers.jsonpb"
	// How long build tags are allowed to be before we trigger a Swarming API
	// error due to how they store tags in a datastore. Most tags shouldn't be
	// anywhere close to this limit, but tags that could potentially be very
	// long we should crop them to this limit.
	maxSwarmingTagLength = 256
	// Maximum number of CTP builds that can be run from one "crosfleet run ..."
	// command.
	maxCTPRunsPerCmd = 12
	// Release P0 QS account.
	releaseP0QSaccount = "release_p0"
)

// ctExecuteSepNames are the names of the test-execution steps in
// cros_test_platform Buildbucket builds for CTPv1 and CTPv2.
var ctpExecuteStepNames = []string{
	"execute",
	"ctpv2 sub-build (async)|Suite Executions (async)",
}

// testCommonFlags contains parameters common to the "run
// test", "run suite", and "run testplan" subcommands.
type testCommonFlags struct {
	enableAutotestSharding bool
	board                  string
	secondaryBoards        []string
	models                 []string
	secondaryModels        []string
	pool                   string
	bucket                 string
	image                  string
	secondaryImages        []string
	release                string
	qsAccount              string
	releaseRetryUrgent     bool
	skipProvisioning       bool
	maxRetries             int
	repeats                int
	priority               int64
	timeoutMins            int
	addedDims              map[string]string
	provisionLabels        map[string]string
	addedTags              map[string]string
	keyvals                map[string]string
	exitEarly              bool
	lacrosPath             string
	secondaryLacrosPaths   []string
	cft                    bool
	testHarness            string
	publicBuilderBucket    string
	publicBuilder          string
	luciProject            string
	trv2                   bool
	dynamicTrv2            bool
	testArgs               string
	tagIncludes            []string
	tagExcludes            []string
	testNameIncludes       []string
	testNameExcludes       []string
	maxInShard             int64
	userDefinedFilters     []*api.CTPFilter
	desktop                bool
}

type fleetValidationResults struct {
	anyValidTests        bool
	validTests           []string
	validModels          []string
	testValidationErrors []string
}

// Registers run command-specific flags
func (c *testCommonFlags) register(f *flag.FlagSet, mainArgType string) {
	f.StringVar(&c.bucket, "bucket", defaultImageBucket, "Google Storage bucket where the specified image(s) are stored.")
	f.StringVar(&c.image, "image", "", `Optional fully specified image name to run test against, e.g. octopus-release/R89-13609.0.0.
If no value for image or release is passed, test will run against the latest green release build for the given board.`)
	f.Var(luciflag.CommaList(&c.secondaryImages), "secondary-images", "Comma-separated list of image name(empty string can be used to skip OS provision for a particular DUT, e.g. -secondary-images $path1,,$path2) for secondary DUTs to run tests against, it need to align with boards in secondary-boards args.")
	f.StringVar(&c.release, "release", "", `Optional ChromeOS release branch to run test against, e.g. R89-13609.0.0.
If no value for image or release is passed, test will run against the latest green release build for the given board.`)
	f.StringVar(&c.board, "board", "", "Board to run tests on.")
	f.Var(luciflag.CommaList(&c.secondaryBoards), "secondary-boards", "Comma-separated list of boards for secondary DUTs to run tests on, a.k.a multi-DUTs testing.")
	f.Var(luciflag.StringSlice(&c.models), "model", fmt.Sprintf(`Model to run tests on; may be specified multiple times.
A maximum of %d tests may be launched per "crosfleet run" command.`, maxCTPRunsPerCmd))
	f.Var(luciflag.CommaList(&c.models), "models", "Comma-separated list of models to run tests on in same format as -model.")
	f.Var(luciflag.CommaList(&c.secondaryModels), "secondary-models", "Comma-separated list of models for secondary DUTs to run tests on, if provided it need to align with boards in secondary-boards args.")
	f.IntVar(&c.repeats, "repeats", 1, fmt.Sprintf(`Number of repeat tests to launch (per model specified).
A maximum of %d tests may be launched per "crosfleet run" command.`, maxCTPRunsPerCmd))
	f.StringVar(&c.pool, "pool", "", "Device pool to run tests on.")
	f.StringVar(&c.qsAccount, "qs-account", "", `Optional Quota Scheduler account to use for this task. Overrides -priority flag.
If no account is set, tests are scheduled using -priority flag.`)
	f.BoolVar(&c.releaseRetryUrgent, "release-retry-urgent", false, `Use the release_p0 quota scheduler account. Only for use by release team.`)
	f.IntVar(&c.maxRetries, "max-retries", 0, "Maximum retries allowed. No retry if set to 0.")
	f.Int64Var(&c.priority, "priority", DefaultSwarmingPriority, `Swarming scheduling priority for tests, between 50 and 255 (lower values indicate higher priorities).
If a Quota Scheduler account is specified via -qs-account, this value is not used.`)
	f.Int64Var(&c.maxInShard, "max-in-shard", 0, `The total number of tests allowed to be used in a shard. Only used when run via CFT pr tag_criteria is also set.`)
	f.IntVar(&c.timeoutMins, "timeout-mins", 360, "Test run timeout.")
	f.Var(flagx.KeyVals(&c.addedDims), "dim", "Additional scheduling dimension in format key=val or key:val; may be specified multiple times.")
	f.Var(flagx.KeyVals(&c.addedDims), "dims", "Comma-separated additional scheduling addedDims in same format as -dim.")
	f.Var(flagx.KeyVals(&c.provisionLabels), "provision-label", "Additional provisionable label in format key=val or key:val; may be specified multiple times.")
	f.Var(flagx.KeyVals(&c.provisionLabels), "provision-labels", "Comma-separated additional provisionable labels in same format as -provision-label.")
	f.Var(flagx.KeyVals(&c.addedTags), "tag", "Additional Swarming metadata tag in format key=val or key:val; may be specified multiple times.")
	f.Var(flagx.KeyVals(&c.addedTags), "tags", "Comma-separated Swarming metadata tags in same format as -tag.")
	f.Var(flagx.KeyVals(&c.keyvals), "autotest-keyval", "Autotest keyval in format key=val or key:val; may be specified multiple times.")
	f.Var(flagx.KeyVals(&c.keyvals), "autotest-keyvals", "Comma-separated Autotest keyvals in same format as -keyval.")
	f.BoolVar(&c.exitEarly, "exit-early", false, "Exit command as soon as test is scheduled. crosfleet will not notify on test validation failure.")
	f.StringVar(&c.lacrosPath, "lacros-path", "", "Optional GCS path pointing to a lacros artifact.")
	f.Var(luciflag.CommaList(&c.secondaryLacrosPaths), "secondary-lacros-paths", "Comma-separated list of lacros paths(empty string can be used to skip lacros provision for a particular DUT, e.g. -secondary-lacros-paths $path1,,$path2) for secondary DUTs to run tests against, if provided it need to align with boards in secondary-boards args.")
	f.BoolVar(&c.cft, "cft", true, "Run via CFT.")
	f.BoolVar(&c.enableAutotestSharding, "enable-autotest-sharding", false, "Enable autotest sharding for autotest suite.")
	f.StringVar(&c.publicBuilder, "public-builder", "", "Public CTP Builder on which the tests are scheduled.")
	f.StringVar(&c.publicBuilderBucket, "public-builder-bucket", "", "Bucket for the Public CTP Builder on which the tests are scheduled.")
	f.StringVar(&c.luciProject, "luci-project", "", "LUCI project which the bucket and builder are associated with")
	f.BoolVar(&c.trv2, "trv2", false, "Run via Trv2.")
	f.BoolVar(&c.dynamicTrv2, "dynamic-trv2", false, "Run via Trv2.")
	f.BoolVar(&c.desktop, "desktop", false, "Run as 'desktop'.")
	f.StringVar(&c.testArgs, "test-args", "", "Test arguments string (meaning depends on test).")
	f.Func("user-defined-filter", "CTPv2 user defined filter as JSON. Can be repeated", func(s string) error {
		var filter api.CTPFilter
		if err := protojson.Unmarshal([]byte(s), &filter); err != nil {
			return err
		}
		c.userDefinedFilters = append(c.userDefinedFilters, &filter)
		return nil
	})

	if mainArgType == testCmdName {
		f.StringVar(&c.testHarness, "harness", "", "Test harness to run tests on (e.g. tast, tauto, etc.).")
	}
	f.BoolVar(&c.skipProvisioning, "skip-provisioning", false, "Skip provisioning DUT with image.")
}

// validateAndAutocompleteFlags returns any errors after validating the CLI
// flags, and autocompletes the -image flag unless it was specified by the user.
func (c *testCommonFlags) validateAndAutocompleteFlags(ctx context.Context, f *flag.FlagSet, args []string, mainArgType, bbService string, authFlags authcli.Flags, printer common.CLIPrinter) error {
	if err := c.validateArgs(f, args, mainArgType); err != nil {
		return err
	}
	if c.release != "" {
		// Users can specify the ChromeOS release branch via the -release flag,
		// rather than specifying a full image name. In this case, we infer the
		// full image name from the release branch.
		c.image = releaseImage(c.board, c.release)
	} else if c.image == "" {
		// If no release or image was specified, determine the latest green release
		// image for the given board.
		latestImage, err := latestImage(ctx, c.board, bbService, authFlags)
		if err != nil {
			printer.WriteTextStderr("Error found while determining latest image for board %s. Moving forward assuming this is an AL run.", c.board)
		}
		printer.WriteTextStderr("Using latest green build image %s for board %s", latestImage, c.board)
		c.image = latestImage
	}
	return nil
}

func (c *testCommonFlags) validateArgs(f *flag.FlagSet, args []string, mainArgType string) error {
	var errors []string
	if c.board == "" {
		errors = append(errors, "missing board flag")
	}
	if c.pool == "" {
		errors = append(errors, "missing pool flag")
	}

	// If running an individual test via CTP, we require the test harness to be
	// specified.
	if mainArgType == testCmdName && c.cft && c.testHarness == "" {
		errors = append(errors, "missing harness flag")
	}
	// harness should not be provided for non-cft.
	if mainArgType == testCmdName && !c.cft && c.testHarness != "" {
		errors = append(errors, "harness should only be provided for single cft test case")
	}
	// trv2 should be false for non-cft.
	if !c.cft && c.trv2 {
		errors = append(errors, "cannot run non-cft test case via trv2")
	}
	// trv2 should be false for non-cft.
	if !c.cft && c.dynamicTrv2 {
		errors = append(errors, "cannot run non-cft test case via dynamic trv2")
	}
	// userDefinedFilters is only for CTPv2, which needs both trv2 and dynamicTrv2
	if (!c.trv2 || !c.dynamicTrv2) && len(c.userDefinedFilters) > 0 {
		errors = append(errors, "user-defined-filters requires trv2 and dynamic-trv2")
	}
	// trv2 should be false for non-cft.
	if !c.cft && c.enableAutotestSharding {
		errors = append(errors, "cannot run non-cft with autotest sharding")
	}
	// cft should be true for tag_criteria based test plan.
	if !c.cft && (len(c.tagIncludes) > 0 || len(c.tagExcludes) > 0 || len(c.testNameIncludes) > 0 || len(c.testNameExcludes) > 0) {
		errors = append(errors, "cannot run non-cft with tag_criteria args")
	}
	if c.image != "" && c.release != "" {
		errors = append(errors, "cannot specify both image and release branch")
	}
	if c.priority < MinSwarmingPriority || c.priority > MaxSwarmingPriority {
		errors = append(errors, fmt.Sprintf("priority flag should be in [%d, %d]", MinSwarmingPriority, MaxSwarmingPriority))
	}
	// If no models are specified, we still schedule one test with model label
	// left blank.
	numUniqueDUTs := int(math.Max(1, float64(len(c.models))))
	if numUniqueDUTs*c.repeats > maxCTPRunsPerCmd {
		errors = append(errors, fmt.Sprintf("total number of CTP runs launched (# models specified * repeats) cannot exceed %d", maxCTPRunsPerCmd))
	}
	if len(args) == 0 {
		errors = append(errors, fmt.Sprintf("missing %v arg", mainArgType))
	}
	// For multi-DUTs result reporting purpose we need board info, so even if
	// explicit secondary models request, we need to ensure board info is also
	// provided and the count matches.
	if len(c.secondaryModels) > 0 && len(c.secondaryBoards) != len(c.secondaryModels) {
		errors = append(errors, fmt.Sprintf("number of requested secondary-boards: %d does not match with number of requested secondary-models: %d", len(c.secondaryBoards), len(c.secondaryModels)))
	}
	// If OS provision is required for secondary DUTs, then we require an image name for
	// each secondary DUT.
	if len(c.secondaryImages) > 0 && len(c.secondaryBoards) != len(c.secondaryImages) {
		errors = append(errors, fmt.Sprintf("number of requested secondary-boards: %d does not match with number of requested secondary-images: %d", len(c.secondaryBoards), len(c.secondaryImages)))
	}
	// If lacros provision is required for secondary DUTs, then we require a path for
	// each secondary DUT.
	if len(c.secondaryLacrosPaths) > 0 && len(c.secondaryLacrosPaths) != len(c.secondaryBoards) {
		errors = append(errors, fmt.Sprintf("number of requested secondary-boards: %d does not match with number of requested secondary-lacros-paths: %d", len(c.secondaryBoards), len(c.secondaryLacrosPaths)))
	}

	// Public Bucket and Public Builder should both provided
	if (c.publicBuilder == "" && c.publicBuilderBucket != "") || (c.publicBuilder != "" && c.publicBuilderBucket == "") {
		errors = append(errors, "both PublicBuilderBucket and PublicBuilder should be specified")
	}

	if c.luciProject != "" && (c.publicBuilderBucket == "" || c.publicBuilder == "") {
		errors = append(errors, "if luciProject is specified, PublicBuilderBucket and PublicBuilder should be specified")
	}

	if len(errors) > 0 {
		return cmdlib.NewUsageError(*f, strings.Join(errors, "\n"))
	}
	return nil
}

// releaseImage constructs a build image name from the release builder for the
// given board and ChromeOS release branch.
func releaseImage(board, release string) string {
	return fmt.Sprintf("%s-release/%s", board, release)
}

// latestImage gets the build image from the latest green release build for
// the given board.
func latestImage(ctx context.Context, board, bbService string, authFlags authcli.Flags) (string, error) {
	releaseBuilder := &buildbucketpb.BuilderID{
		Project: "chromeos",
		Bucket:  "release",
		Builder: fmt.Sprintf("%s-release-main", board),
	}
	releaseBBClient, err := buildbucket.NewClient(ctx, releaseBuilder, bbService, authFlags)
	if err != nil {
		return "", err
	}
	latestGreenRelease, err := releaseBBClient.GetLatestGreenBuild(ctx)
	if err != nil {
		return "", err
	}
	outputProperties := latestGreenRelease.Output.Properties.GetFields()
	artifacts := outputProperties["artifacts"].GetStructValue().GetFields()
	image := artifacts["gs_path"].GetStringValue()
	if image == "" {
		buildURL := releaseBBClient.BuildURL(latestGreenRelease.Id)
		return "", fmt.Errorf("most recent release for board %s has no corresponding build image; visit release build at %s for more details", board, buildURL)
	}
	return image, nil
}

// buildTagsForCrosfleet combines test metadata tags with user-added tags
func (c *testCommonFlags) commonTagsForAllBuilds(crosfleetTool string, mainArg string) map[string]string {
	tags := map[string]string{}

	// Add user-added tags.
	for key, val := range c.addedTags {
		tags[key] = val
	}

	// Add crosfleet-tool tag.
	if crosfleetTool == "" {
		panic(fmt.Errorf("must provide %s tag", common.CrosfleetToolTag))
	}
	tags[common.CrosfleetToolTag] = crosfleetTool
	if mainArg != "" {
		// Intended for `run test` and `run suite` commands. This label takes
		// the form "label-suite:SUITE_NAME" for a `run suite` command.
		tags[fmt.Sprintf("label-%s", crosfleetTool)] = mainArg
	}

	return tags
}

func (c *testCommonFlags) buildTagsForCTPBuilds(crosfleetTool string, mainArg string) map[string]string {
	tags := c.commonTagsForAllBuilds(crosfleetTool, mainArg)
	tags[buildbucket.UserAgentTagKey] = buildbucket.CrosfleetUserAgent

	return tags
}

// Gets the CTPBuilder based on the env and the specified custom public ctp builder parameters.
func (c *testCommonFlags) getCTPBuilder(env site.Environment) *buildbucketpb.BuilderID {
	builder := proto.Clone(env.DefaultCTPBuilder).(*buildbucketpb.BuilderID)
	if c.publicBuilderBucket != "" {
		builder.Bucket = c.publicBuilderBucket
	}
	if c.publicBuilder != "" {
		builder.Builder = c.publicBuilder
	}
	if c.luciProject != "" {
		builder.Project = c.luciProject
	}
	return builder
}

// testRunLauncher contains the necessary information to launch and validate a
// CTP test plan.
type ctpRunLauncher struct {
	// Tag denoting the tests or suites specified to run; left blank for custom
	// test plans.
	mainArgsTag string
	printer     common.CLIPrinter
	cmdName     string
	bbClient    buildbucket.Client
	testPlan    *test_platform.Request_TestPlan
	cliFlags    *testCommonFlags
}

// launchAndOutputTests invokes the inner launchTestsAsync() function
// and handles the CLI output of the buildLaunchList JSON object, which should
// happen even in case of command failure.
func (l *ctpRunLauncher) launchAndOutputTests(ctx context.Context) error {
	buildLaunchList, err := l.launchTestsAsync(ctx)
	l.printer.WriteJSONStdout(buildLaunchList)
	return err
}

// launchTestsAsync requests a run of the given CTP run launcher's
// test plan, and returns the ID of the launched cros_test_platform Buildbucket
// build. Unless the exitEarly arg is passed as true, the function waits to
// return until the build passes request-validation and setup steps.
func (l *ctpRunLauncher) launchTestsAsync(ctx context.Context) (*crosfleetpb.BuildLaunchList, error) {
	buildLaunchList, scheduledAnyBuilds, schedulingErrors := l.scheduleCTPBuildsAsync(ctx)
	if len(schedulingErrors) > 0 {
		fullErrorMsg := fmt.Sprintf("Encountered the following errors requesting %s run(s):\n%s\n",
			l.cmdName, strings.Join(schedulingErrors, "\n"))
		if scheduledAnyBuilds {
			// Don't fail the command if we were able to request some builds.
			l.printer.WriteTextStderr(fullErrorMsg)
		} else {
			return buildLaunchList, errors.New(fullErrorMsg)
		}
	}
	if l.cliFlags.exitEarly {
		return buildLaunchList, nil
	}
	l.printer.WriteTextStderr(`Waiting to confirm %s run request validation...
(To skip this step, pass the -exit-early flag on future %s run commands)
`, l.cmdName, l.cmdName)
	confirmedAnyBuilds, confirmationErrors := l.confirmCTPBuildsAsync(ctx, buildLaunchList)
	if len(confirmationErrors) > 0 {
		fullErrorMsg := fmt.Sprintf("Encountered the following errors confirming %s run(s):\n%s\n",
			l.cmdName, strings.Join(confirmationErrors, "\n"))
		if confirmedAnyBuilds {
			// Don't fail the command if we were able to confirm some of the
			// requested builds as having started.
			l.printer.WriteTextStderr(fullErrorMsg)
		} else {
			return buildLaunchList, errors.New(fullErrorMsg)
		}
	}
	return buildLaunchList, nil
}

// scheduleCTPBuild uses the given Buildbucket client to request a
// cros_test_platform Buildbucket build for the CTP run launcher's test plan,
// build tags, and command line flags, and returns the ID of the pending build.
func (l *ctpRunLauncher) scheduleCTPBuild(ctx context.Context, model string) (*buildbucketpb.Build, error) {
	ctp := l.ctpBuilder(model)
	return ctp.ScheduleCTPBuild(ctx)
}

func (l *ctpRunLauncher) ctpBuilder(model string) *builder.CTPBuilder {
	ctpTags := l.cliFlags.buildTagsForCTPBuilds(l.cmdName, l.mainArgsTag)
	testRunnerTags := l.cliFlags.commonTagsForAllBuilds(l.cmdName, l.mainArgsTag)
	props := map[string]any{}
	buildbucket.AddServiceVersion(props)

	if l.cliFlags.releaseRetryUrgent {
		l.cliFlags.qsAccount = releaseP0QSaccount
	}

	filters := l.cliFlags.userDefinedFilters
	if l.testPlan.Suite != nil {
		for _, suite := range l.testPlan.GetSuite() {
			if l.cliFlags.desktop {
				if !strings.HasPrefix(suite.GetName(), "AL.") {
					suite.Name = "AL." + suite.Name
				}
			}
			if strings.HasPrefix(suite.GetName(), "AL.") {
				alProvisionFilter := &api.CTPFilter{
					ContainerInfo: &api.ContainerInfo{
						Container: &buildapi.ContainerImageInfo{
							Name: "al-provision-filter",
						},
					},
				}
				foilFilter := &api.CTPFilter{
					ContainerInfo: &api.ContainerInfo{
						Container: &buildapi.ContainerImageInfo{
							Name: "foil-filter",
						},
					},
				}
				alTestFinder := &api.CTPFilter{
					ContainerInfo: &api.ContainerInfo{
						Container: &buildapi.ContainerImageInfo{
							Name: "test-finder",
						},
					},
				}
				antsPublishFilter := &api.CTPFilter{
					ContainerInfo: &api.ContainerInfo{
						Container: &buildapi.ContainerImageInfo{
							Name: "ants-publish-filter",
						},
					},
				}
				filters = append(filters, alProvisionFilter, foilFilter, alTestFinder, antsPublishFilter)
				break
			}
		}
	}

	return &builder.CTPBuilder{
		BBClient:            l.bbClient.GetBuildsClient(),
		Board:               l.cliFlags.board,
		BuilderID:           l.bbClient.GetBuilderID(),
		CFT:                 l.cliFlags.cft,
		CTPBuildTags:        ctpTags,
		Dimensions:          l.cliFlags.addedDims,
		Image:               l.cliFlags.image,
		ImageBucket:         l.cliFlags.bucket,
		Keyvals:             l.cliFlags.keyvals,
		MaxRetries:          l.cliFlags.maxRetries,
		Model:               model,
		Pool:                l.cliFlags.pool,
		Priority:            l.cliFlags.priority,
		Properties:          props,
		ProvisionLabels:     l.cliFlags.provisionLabels,
		QSAccount:           l.cliFlags.qsAccount,
		SecondaryBoards:     l.cliFlags.secondaryBoards,
		SecondaryImages:     l.cliFlags.secondaryImages,
		SecondaryModels:     l.cliFlags.secondaryModels,
		SkipProvisioning:    l.cliFlags.skipProvisioning,
		TestPlan:            l.testPlan,
		TestRunnerBuildTags: testRunnerTags,
		TimeoutMins:         l.cliFlags.timeoutMins,
		TRV2:                l.cliFlags.trv2 || l.cliFlags.dynamicTrv2,
		DynamicTRV2:         l.cliFlags.dynamicTrv2,
		UserDefinedFilters:  filters,
	}
}

// scheduleCTPBuildsAsync schedules all builds asynchronously and returns a
// build launch list, a bool indicating whether any builds were successfully
// scheduled, and a slice of scheduling error strings. Mutex locks are used to
// avoid race conditions from concurrent writes to the return variables in the
// async loop.
func (l *ctpRunLauncher) scheduleCTPBuildsAsync(ctx context.Context) (buildLaunchList *crosfleetpb.BuildLaunchList, scheduledAnyBuilds bool, schedulingErrors []string) {
	buildLaunchList = &crosfleetpb.BuildLaunchList{}
	waitGroup := sync.WaitGroup{}
	mutex := sync.Mutex{}
	allModels := l.cliFlags.models
	if len(allModels) == 0 {
		// If no models are specified, just launch one run with a blank model.
		allModels = []string{""}
	}
	for _, model := range allModels {
		for range l.cliFlags.repeats {
			waitGroup.Add(1)
			model := model
			go func() {
				build, err := l.scheduleCTPBuild(ctx, model)
				mutex.Lock()
				errString := ""
				if err != nil {
					errString = fmt.Sprintf("Error requesting %s run for model %s: %s", l.cmdName, model, err.Error())
					schedulingErrors = append(schedulingErrors, errString)
				} else {
					scheduledAnyBuilds = true
					l.printer.WriteTextStderr("Requesting %s run at %s", l.cmdName, l.bbClient.BuildURL(build.Id))
				}
				buildLaunchList.Launches = append(buildLaunchList.Launches, &crosfleetpb.BuildLaunch{
					Build:      build,
					BuildError: errString,
				})
				mutex.Unlock()
				waitGroup.Done()
			}()
		}
	}
	waitGroup.Wait()
	return
}

// confirmCTPBuildsAsync waits for all builds to start asynchronously, and
// updates the details for each build it confirms has started in the given build
// launch list. The function returns a bool indicating whether any builds were
// confirmed started, and a slice of confirmation error strings. Mutex locks are
// used to avoid race conditions from concurrent writes to the return variables
// in the async loop.
func (l *ctpRunLauncher) confirmCTPBuildsAsync(ctx context.Context, buildLaunchList *crosfleetpb.BuildLaunchList) (confirmedAnyBuilds bool, confirmationErrors []string) {
	waitGroup := sync.WaitGroup{}
	mutex := sync.Mutex{}
	for _, buildLaunch := range buildLaunchList.Launches {
		// Only wait for builds that were already scheduled without issues.
		if buildLaunch.Build == nil || buildLaunch.Build.GetId() == 0 || buildLaunch.BuildError != "" {
			continue
		}
		waitGroup.Add(1)
		go func() {
			updatedBuild, err := l.bbClient.WaitForBuildStepStart(ctx, buildLaunch.Build.Id, ctpExecuteStepNames...)
			mutex.Lock()
			if updatedBuild != nil {
				buildLaunch.Build = updatedBuild
				if updatedBuild.Status == buildbucketpb.Status_STARTED {
					confirmedAnyBuilds = true
					l.printer.WriteTextStdout("Successfully started %s run %d", l.cmdName, updatedBuild.Id)
				}
			}
			if err != nil {
				errString := fmt.Sprintf("Error waiting for build %d to start: %s", buildLaunch.Build.Id, err.Error())
				buildLaunch.BuildError = errString
				confirmationErrors = append(confirmationErrors, errString)
			}
			mutex.Unlock()
			waitGroup.Done()
		}()
	}
	waitGroup.Wait()
	return
}

// testOrSuiteNamesTag formats a label for the given test/suite names, to be
// added to the build tags of a cros_test_platform build launched for the given
// tests/suites.
func testOrSuiteNamesTag(names []string) string {
	if len(names) == 0 {
		panic("no test/suite names given")
	}
	var label string
	if len(names) > 1 {
		label = fmt.Sprintf("%v", names)
	} else {
		label = names[0]
	}
	if len(label) > maxSwarmingTagLength {
		return label[:maxSwarmingTagLength]
	}
	return label
}

// verifyFleetTestsPolicy validate tests based on fleet-side permission check.
//
// This method calls UFS CheckFleetTestsPolicy RPC for each testName, board, image and model combination.
// The test run stops if an invalid board or image is specified.
// After this validation only valid models and tests will be used in the run command.
func (c *testCommonFlags) verifyFleetTestsPolicy(ctx context.Context, ufsClient ufs.Client, cmdName string,
	testNames []string, allowPublicUserAccount bool) (*fleetValidationResults, error) {
	validTestNamesMap := map[string]bool{}
	validModelsMap := map[string]bool{}
	results := &fleetValidationResults{}

	// Calling the UFS CheckFleetTestsPolicy with empty test params.
	// For a user account which runs private tests there is no validation on the UFS side so the CheckFleetTestsPolicy will return a valid test response for empty test params.
	// If UFS returns an OK status for this RPC then it means that the service account is not something that is used to run public tests so we can skip further validation.
	// This check is to avoid unnecessary RPC calls to UFS for tests run by service accounts meant for private tests.
	isPublicTestResponse, err := ufsClient.CheckFleetTestsPolicy(ctx, &ufsapi.CheckFleetTestsPolicyRequest{})
	if err != nil && !allowPublicUserAccount {
		return nil, fmt.Errorf("Public user service accounts are not allowed to run %s run(s)",
			cmdName)
	}
	if isPublicTestResponse != nil && isPublicTestResponse.TestStatus.Code == ufsapi.TestStatus_OK {
		results.anyValidTests = true
		results.validModels = c.models
		results.validTests = testNames
		return results, nil
	}

	if len(c.models) == 0 {
		// Model is optional when the board has all public models
		c.models = []string{""}
	}
	for _, model := range c.models {
		for _, testName := range testNames {
			resp, err := ufsClient.CheckFleetTestsPolicy(ctx, &ufsapi.CheckFleetTestsPolicyRequest{
				TestName:  testName,
				Board:     c.board,
				Model:     model,
				Image:     c.image,
				QsAccount: c.qsAccount,
			})
			if err != nil {
				results.testValidationErrors = append(results.testValidationErrors, err.Error())
				continue
			}
			if resp.TestStatus.Code == ufsapi.TestStatus_OK {
				results.anyValidTests = true
				validModelsMap[model] = true
				validTestNamesMap[testName] = true
				continue
			}
			if resp.TestStatus.Code == ufsapi.TestStatus_NOT_A_PUBLIC_BOARD || resp.TestStatus.Code == ufsapi.TestStatus_NOT_A_PUBLIC_IMAGE ||
				resp.TestStatus.Code == ufsapi.TestStatus_INVALID_QS_ACCOUNT {
				// No tests can be run with Invalid Board, Image or QsAccount so returning early to avoid unnecessary calls to UFS
				return nil, fmt.Errorf(resp.TestStatus.Message)
			}
			results.testValidationErrors = append(results.testValidationErrors, resp.TestStatus.Message)
		}
	}

	for test := range validTestNamesMap {
		results.validTests = append(results.validTests, test)
	}
	for model := range validModelsMap {
		results.validModels = append(results.validModels, model)
	}

	return results, nil
}

func checkAndPrintFleetValidationErrors(results fleetValidationResults, printer common.CLIPrinter, cmdName string) error {
	if len(results.testValidationErrors) > 0 {
		fullErrorMsg := fmt.Sprintf("Encountered the following errors requesting %s run(s):\n%s\n",
			cmdName, strings.Join(results.testValidationErrors, "\n"))
		if results.anyValidTests {
			// Don't fail the command if we were able to request some runs.
			printer.WriteTextStderr(fullErrorMsg)
		} else {
			return errors.New(fullErrorMsg)
		}
	}
	return nil
}
