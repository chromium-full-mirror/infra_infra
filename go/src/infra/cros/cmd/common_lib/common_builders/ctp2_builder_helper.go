// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_builders

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
)

const (
	DefaultChromeosBuildGcsBucket = "chromeos-image-archive"
	ChromeosBuild                 = "chromeos_build"
	ChromeosBuildGcsBucket        = "chromeos_build_gcs_bucket"
	RoFirmwareBuild               = "ro_firmware_build"
	RwFirmwareBuild               = "rw_firmware_build"
	LacrosGcsPath                 = "lacros_gcs_path"
	AndroidImageVersion           = "android_image_version"
	GmsCorePackage                = "gms_core_package"
	Public                        = "PUBLIC"
	Private                       = "PRIVATE"
)

var (
	ExcludedChromeosBuildPrefixes  = []string{"staging", "dev"}
	ExcludedChromeosBuildPostfixes = []string{"main"}
	ExcludedVariantPostfixes       = []string{"sdknext"}
)

// GroupV2Requests filters CTP requests list by manifest. "PUBLIC" manifest will
// not be grouped. The manifest information is fetched from container image info.
// Eligible requests are grouped together based on suite and board.
// The function then returns the grouped eligible requests.
//
// NOTE: The manifest is being fetched from container image info because that's info
// is used while generating new containers or resuing cache containers. "PUBLIC" manifest
// boards do not use cached containers and therefore grouping shouldn't be done.
func GroupV2Requests(ctx context.Context, v2s []*V2WithKey, manifestFetcher ManifestFetcher) ([]*V2WithKey, map[string][]string) {

	// For 3D requests, group both public and private manifest build targets together.
	if len(v2s) > 0 && v2s[0].V2.GetSuiteRequest().GetDddSuite() {
		groupedEligibleRequests, reqChainMap := GroupEligibleV2Requests(ctx, v2s)
		return groupedEligibleRequests, reqChainMap
	}

	// If not a 3D suite, group private manifest build targets together.
	// Each public build target request will be grouped individually.
	eligible, public := FilterV2RequestsBasedOnManifest(ctx, v2s, manifestFetcher)
	groupedEligibleRequests, reqChainMap := GroupEligibleV2Requests(ctx, eligible)

	// merge public group as is with groupedEligibleRequests
	for _, eachPublic := range public {
		groupedEligibleRequests = append(groupedEligibleRequests, eachPublic)
		reqChainMap[eachPublic.Key] = []string{eachPublic.Key}
	}

	return groupedEligibleRequests, reqChainMap
}

// FilterV2RequestsBasedOnManifest divides the ctp requests into two groups based on the manifest from container
// image info. One set for "PUBLIC" manifest and another for others.
func FilterV2RequestsBasedOnManifest(ctx context.Context, v2s []*V2WithKey, manifestFetcher ManifestFetcher) ([]*V2WithKey, []*V2WithKey) {
	public := []*V2WithKey{}
	nonPublic := []*V2WithKey{}
	imageManifestMap := make(map[string]string)
	for _, v2Withkey := range v2s {
		v2 := v2Withkey.V2
		// Safe guard against CTPRequests missing targets to schedule on.
		if len(v2.GetScheduleTargets()) == 0 || len(v2.GetScheduleTargets()[0].GetTargets()) == 0 {
			logging.Infof(ctx, "request missing targets to schedule on, dropping: %v", v2)
			continue
		}
		// Translator only supplies singular length schedule targets,
		// and only care about checking the primary target's gcspath when grouping.
		gcsPath := v2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath()
		// Check if manifest for given gcsPath already exists
		manifest, ok := imageManifestMap[gcsPath]
		if !ok {
			// Fetch manifest info from containerImageInfo
			manifest, err := manifestFetcher(ctx, gcsPath)
			if err != nil {
				// Instead of dropping, add it to public
				logging.Infof(ctx, "failed to fetch manifest info for %s, add to public list: %v", gcsPath, v2)
				public = append(public, v2Withkey)
				continue
			}
			// Update the imageManifestMap with manifest info
			imageManifestMap[gcsPath] = manifest
			// If manifest is "PUBLIC" then add to public
			if manifest == Public {
				public = append(public, v2Withkey)
			} else {
				nonPublic = append(nonPublic, v2Withkey)
			}
		} else {
			// If manifest is "PUBLIC" then add to public
			if manifest == Public {
				public = append(public, v2Withkey)
			} else {
				nonPublic = append(nonPublic, v2Withkey)
			}
		}
	}
	return nonPublic, public
}

// GroupEligibleV2Requests reduces the list of v2 requests by grouping
// by build and suite request.
func GroupEligibleV2Requests(ctx context.Context, v2s []*V2WithKey) ([]*V2WithKey, map[string][]string) {
	reqChainMap := map[string][]string{}
	groups := map[string][]*V2WithKey{}
	for _, v2Withkey := range v2s {
		v2 := v2Withkey.V2
		reqKey := v2Withkey.Key
		logging.Infof(ctx, fmt.Sprintf("reqKey:%s", reqKey))
		// Translator only supplies singular length schedule targets,
		// and only care about checking the primary target's gcspath when grouping.
		gcsPath := v2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath()

		build := common.GetMajorBuildFromGCSPath(gcsPath)
		if build == "" || v2.SuiteRequest == nil {
			continue
		}
		if _, ok := groups[build]; !ok {
			groups[build] = []*V2WithKey{}
			logging.Infof(ctx, fmt.Sprintf("new group created for reqKey:%s", reqKey))
		}

		foundMatch := false
		for _, suiteGroup := range groups[build] {
			if canBeGrouped(suiteGroup.V2, v2) {
				combineRequests(suiteGroup.V2, v2)
				reqChainMap[suiteGroup.Key] = append(reqChainMap[suiteGroup.Key], reqKey)
				logging.Infof(ctx, fmt.Sprintf("Combining %s with %s", reqKey, suiteGroup.Key))
				foundMatch = true
				break
			}
		}
		if foundMatch {
			continue
		}
		groups[build] = append(groups[build], v2Withkey)
		// Add itself as well
		reqChainMap[reqKey] = []string{reqKey}
	}
	flatRequests := []*V2WithKey{}
	for _, buildRequests := range groups {
		flatRequests = append(flatRequests, buildRequests...)

	}

	return flatRequests, reqChainMap
}

// GetBuilderManifestFromContainer returns the manifest info fetched from container image info
func GetBuilderManifestFromContainer(ctx context.Context, gcsPath string) (string, error) {
	containerImageInfo, err := common.FetchImageData(ctx, "", gcsPath)
	if err != nil {
		return "", err
	}
	for _, imageInfo := range containerImageInfo {
		for _, tag := range imageInfo.GetTags() {
			if strings.Contains(tag, Public) {
				return Public, nil
			}
		}
		break
	}
	return Private, nil
}

// combineRequests does various combining techniques to
// reduce the requests into one.
func combineRequests(r1, r2 *testapi.CTPRequest) {
	// Combine list of schedule targets.
	r1.ScheduleTargets = append(r1.GetScheduleTargets(), r2.GetScheduleTargets()...)

	// Take the longest maximum duration for grouped suites.
	if r1.GetSuiteRequest().GetMaximumDuration() != nil || r2.GetSuiteRequest().GetMaximumDuration() != nil {
		timeoutSeconds := max(
			r1.GetSuiteRequest().GetMaximumDuration().GetSeconds(),
			r2.GetSuiteRequest().GetMaximumDuration().GetSeconds(),
		)
		r1.GetSuiteRequest().MaximumDuration = &durationpb.Duration{
			Seconds: timeoutSeconds,
		}
	}
}

// canBeGrouped checks whether two CTPRequests can be grouped
// together based on factors such as suite equality and matching
// filters.
func canBeGrouped(r1, r2 *testapi.CTPRequest) bool {
	// Compare suite request
	if !proto.Equal(
		removeNonGroupableSuiteFields(r1.GetSuiteRequest()),
		removeNonGroupableSuiteFields(r2.GetSuiteRequest())) {
		return false
	}

	// Compare filters provided
	if len(r1.GetKarbonFilters()) != len(r2.GetKarbonFilters()) {
		return false
	}

	for i := range r1.GetKarbonFilters() {
		if !proto.Equal(
			r1.GetKarbonFilters()[i],
			r2.GetKarbonFilters()[i]) {
			return false
		}
	}

	// Compare pool
	if r1.GetPool() != r2.GetPool() {
		return false
	}

	// Compare scheduler
	if !proto.Equal(r1.GetSchedulerInfo(), r2.GetSchedulerInfo()) {
		return false
	}

	// Compare 3D-ness
	// Don't combine 3D with non-3d requests
	r1Is3DSuite := r1.GetSuiteRequest().GetDddSuite()
	r2Is3DSuite := r2.GetSuiteRequest().GetDddSuite()
	if (r1Is3DSuite && !r2Is3DSuite) || (!r1Is3DSuite && r2Is3DSuite) {
		return false
	}

	return true
}

// removeNonGroupableSuiteFields creates a temp SuiteRequest that contains items that should match
// exactly between two grouped together SuiteRequests. The other fields should be combined/reduced
// in some way after being grouped.
func removeNonGroupableSuiteFields(suite *testapi.SuiteRequest) *testapi.SuiteRequest {
	groupable := proto.Clone(suite).(*testapi.SuiteRequest)

	// Remove non-groupable fields.
	groupable.MaximumDuration = nil

	return groupable
}

// buildCTPRequest converts a v1 ctp request into a v2 CTPRequest.
func buildCTPRequest(v1 *test_platform.Request, buildState *build.State) *testapi.CTPRequest {
	return &testapi.CTPRequest{
		SuiteRequest:    buildSuiteRequest(v1),
		ScheduleTargets: buildScheduleTargets(v1),
		SchedulerInfo:   buildSchedulerInfo(v1, buildState),
		Pool:            getSchedulingPool(v1),
		KarbonFilters:   v1.GetParams().GetUserDefinedFilters(),
		// Reuse translate flag from v1 to signal dynamic run in v2.
		RunDynamic: v1.GetParams().GetTranslateTrv2Request(),
		IsAlRun:    getIsALRun(v1),
	}
}

// buildSchedulerInfo produces the scheduling system to be used,
// as well as the qs account for qs scheduling.
func buildSchedulerInfo(v1 *test_platform.Request, buildState *build.State) *testapi.SchedulerInfo {
	dryRun := v1.GetParams().GetDryRunCtpv2()
	runWithQs := v1.GetParams().GetRunCtpv2WithQs()
	scheduler := testapi.SchedulerInfo_SCHEDUKE
	if dryRun {
		scheduler = testapi.SchedulerInfo_PRINT_REQUEST_ONLY
	} else if runWithQs || isVmlabPoolReq(v1) {
		scheduler = testapi.SchedulerInfo_QSCHEDULER
	}

	// Run via QS if external CTP bucket.
	builder := buildState.Build().GetBuilder()
	if builder != nil {
		if builder.Bucket != common.CTPBucket && builder.Bucket != common.CTPBucketShadow {
			scheduler = testapi.SchedulerInfo_QSCHEDULER
		}
	}

	return &testapi.SchedulerInfo{
		// TODO(cdelagarza): Update to upstream variable.
		Scheduler: scheduler,
		QsAccount: v1.GetParams().GetScheduling().GetQsAccount(),
	}
}

// isVmlabPoolReq checks if the request is for vmlab pool
func isVmlabPoolReq(v1 *test_platform.Request) bool {
	return v1.GetParams().GetScheduling().GetUnmanagedPool() == "vmlab"
}

// buildSuiteRequest converts a v1 ctp request into a SuiteRequest.
func buildSuiteRequest(v1 *test_platform.Request) *testapi.SuiteRequest {
	return &testapi.SuiteRequest{
		SuiteRequest: &testapi.SuiteRequest_TestSuite{
			TestSuite: buildTestSuite(v1),
		},
		MaximumDuration: v1.GetParams().GetTime().GetMaximumDuration(),
		TestArgs:        getTestArgs(v1),
		AnalyticsName:   getAnalyticsName(v1),
		MaxInShard:      v1.GetTestPlan().GetMaxInShard(),
		DddSuite:        IsDDDSuite(v1),
		RetryCount:      GetRetryCount(v1),
	}
}

// buildTestSuite parses through the available options for
// setting up a test suite. This code follows the ctpv1
// method `_ctr_test_suite` which converts a test plan
// into a TestSuite object.
func buildTestSuite(v1 *test_platform.Request) *testapi.TestSuite {
	testplan := v1.TestPlan
	testSuite := &testapi.TestSuite{
		TotalShards: testplan.TotalShards,
	}
	if len(testplan.GetSuite()) == 0 {
		testSuite.Name = "adhoc"
	} else {
		testSuite.Name = testplan.GetSuite()[0].GetName()
	}

	if testplan.Test != nil && len(testplan.Test) > 0 {
		testCaseIds := []*testapi.TestCase_Id{}
		for _, test := range testplan.Test {
			switch harness := test.GetHarness().(type) {
			case *test_platform.Request_Test_Autotest_:
				testCaseIds = append(testCaseIds, &testapi.TestCase_Id{
					Value: harness.Autotest.Name,
				})
			}
		}
		testSuite.Spec = &testapi.TestSuite_TestCaseIds{
			TestCaseIds: &testapi.TestCaseIdList{
				TestCaseIds: testCaseIds,
			},
		}
	} else if testplan.TagCriteria != nil &&
		(testplan.TagCriteria.TagExcludes != nil ||
			testplan.TagCriteria.Tags != nil ||
			testplan.TagCriteria.TestNameExcludes != nil ||
			testplan.TagCriteria.TestNames != nil) {
		testSuite.Spec = &testapi.TestSuite_TestCaseTagCriteria_{
			TestCaseTagCriteria: testplan.TagCriteria,
		}
	} else {
		tags := []string{}
		for _, suite := range testplan.GetSuite() {
			tags = append(tags, fmt.Sprintf("suite:%s", suite.GetName()))
		}
		testSuite.Spec = &testapi.TestSuite_TestCaseTagCriteria_{
			TestCaseTagCriteria: &testapi.TestSuite_TestCaseTagCriteria{
				Tags: tags,
			},
		}
	}

	return testSuite
}

// getTestArgs returns the available test args.
func getTestArgs(v1 *test_platform.Request) string {
	testplan := v1.TestPlan
	if testplan.Test != nil && len(testplan.Test) > 0 {
		return testplan.GetTest()[0].GetAutotest().GetTestArgs()
	} else if testplan.Suite != nil && len(testplan.Suite) > 0 {
		return testplan.GetSuite()[0].GetTestArgs()
	}

	return ""
}

// buildScheduleTargets converts the v1 primary and
// secondary devices into v2 target objects.
func buildScheduleTargets(v1 *test_platform.Request) []*testapi.ScheduleTargets {
	targets := []*testapi.Targets{
		buildTarget(
			v1.GetParams().GetSoftwareAttributes(),
			v1.GetParams().GetHardwareAttributes(),
			v1.GetParams().GetFreeformAttributes(),
			v1.GetParams().GetSoftwareDependencies()),
	}
	for _, secondary := range v1.GetParams().GetSecondaryDevices() {
		targets = append(targets,
			buildTarget(
				secondary.GetSoftwareAttributes(),
				secondary.GetHardwareAttributes(),
				&test_platform.Request_Params_FreeformAttributes{},
				secondary.GetSoftwareDependencies()))
	}
	return []*testapi.ScheduleTargets{
		{
			Targets: targets,
		},
	}
}

// buildTarget converts v1 software/hardware attributes
// and software dependencies into a v2 target object.
func buildTarget(
	softwareAttributes *test_platform.Request_Params_SoftwareAttributes,
	hardwareAttributes *test_platform.Request_Params_HardwareAttributes,
	freeformAttributes *test_platform.Request_Params_FreeformAttributes,
	softwareDeps []*test_platform.Request_Params_SoftwareDependency) *testapi.Targets {

	return &testapi.Targets{
		HwTarget: &testapi.HWTarget{
			Target: &testapi.HWTarget_LegacyHw{
				LegacyHw: &testapi.LegacyHW{
					Board:              softwareAttributes.GetBuildTarget().GetName(),
					Model:              hardwareAttributes.GetModel(),
					SwarmingDimensions: freeformAttributes.GetSwarmingDimensions(),
					Variant:            GetVariant(softwareDeps),
				},
			},
		},
		SwTarget: &testapi.SWTarget{
			SwTarget: &testapi.SWTarget_LegacySw{
				LegacySw: &testapi.LegacySW{
					Build:     GetBuildType(softwareDeps),
					GcsPath:   getImageGcsPath(softwareDeps),
					KeyValues: mapSoftwareDeps(softwareDeps),
				},
			},
		},
	}
}

// mapSoftwareDeps converts each software dependency
// into a corresponding key value pair.
func mapSoftwareDeps(softwareDeps []*test_platform.Request_Params_SoftwareDependency) []*testapi.KeyValue {
	m := map[string]string{}
	for _, softwareDep := range softwareDeps {
		switch dep := softwareDep.GetDep().(type) {
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuild:
			m[ChromeosBuild] = dep.ChromeosBuild
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuildGcsBucket:
			m[ChromeosBuildGcsBucket] = dep.ChromeosBuildGcsBucket
		case *test_platform.Request_Params_SoftwareDependency_RoFirmwareBuild:
			m[RoFirmwareBuild] = dep.RoFirmwareBuild
		case *test_platform.Request_Params_SoftwareDependency_RwFirmwareBuild:
			m[RwFirmwareBuild] = dep.RwFirmwareBuild
		case *test_platform.Request_Params_SoftwareDependency_LacrosGcsPath:
			m[LacrosGcsPath] = dep.LacrosGcsPath
		case *test_platform.Request_Params_SoftwareDependency_AndroidImageVersion:
			m[AndroidImageVersion] = dep.AndroidImageVersion
		case *test_platform.Request_Params_SoftwareDependency_GmsCorePackage:
			m[GmsCorePackage] = dep.GmsCorePackage
		}
	}

	keyValues := []*testapi.KeyValue{}
	for k, v := range m {
		keyValues = append(keyValues, &testapi.KeyValue{
			Key:   k,
			Value: v,
		})
	}
	return keyValues
}

// getSchedulingPool parses the v1 request tags for the label-pool.
func getSchedulingPool(v1 *test_platform.Request) string {
	return getTag(v1.GetParams().GetDecorations().GetTags(), common.LabelPool)
}

// getAnalyticsName parses the v1 request tags for the analytics_name.
func getAnalyticsName(v1 *test_platform.Request) string {
	return getTag(v1.GetParams().GetDecorations().GetTags(), common.AnalyticsName)
}

// IsDDDSuite will return if the suite is to run in ddd.
// TODO (b:327505895): For now, use the ddd prefix, but long term will move to a proper flag.
func IsDDDSuite(v1 *test_platform.Request) bool {
	return v1.GetParams().GetDddSuite() || strings.HasPrefix(getAnalyticsName(v1), "ddd")
}

// GetRetryCount returns the retry count from v1 request.
// By default it will be 0 which means no retry.
func GetRetryCount(v1 *test_platform.Request) int64 {
	if v1.GetParams().GetRetry().GetAllow() {
		retries := int64(v1.GetParams().GetRetry().GetMax())
		if v1.GetParams().GetTestExecutionBehavior() == test_platform.Request_Params_CRITICAL {
			// Limit to one retry per shard if critical. Remain zero if zero.
			return min(retries, 1)
		}
		return retries
	}

	return 0
}

// GetBuildType parses the software dependency's ChromeosBuild
// into the build type by taking the last part after removing postfixes.
func GetBuildType(softwareDeps []*test_platform.Request_Params_SoftwareDependency) string {
	for _, softwareDep := range softwareDeps {
		switch dep := softwareDep.GetDep().(type) {
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuild:
			chromeosBuildLeft := strings.Split(dep.ChromeosBuild, "/")[0]
			chromeosBuildParts := strings.Split(chromeosBuildLeft, "-")
			// Strip post-fixes.
			if slices.Contains(ExcludedChromeosBuildPostfixes, chromeosBuildParts[len(chromeosBuildParts)-1]) {
				chromeosBuildParts = chromeosBuildParts[:len(chromeosBuildParts)-1]
			}
			return chromeosBuildParts[len(chromeosBuildParts)-1]
		}
	}
	return ""
}

// GetVariant parses the software dependency's ChromeosBuild
// and removes the prefixes, postfixes, and base board.
func GetVariant(softwareDeps []*test_platform.Request_Params_SoftwareDependency) string {
	for _, softwareDep := range softwareDeps {
		switch dep := softwareDep.GetDep().(type) {
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuild:
			chromeosBuildLeft := strings.Split(dep.ChromeosBuild, "/")[0]
			chromeosBuildParts := strings.Split(chromeosBuildLeft, "-")
			// Strip post-fixes.
			if slices.Contains(ExcludedChromeosBuildPostfixes, chromeosBuildParts[len(chromeosBuildParts)-1]) {
				chromeosBuildParts = chromeosBuildParts[:len(chromeosBuildParts)-1]
			}
			// Strip pre-fixes.
			if slices.Contains(ExcludedChromeosBuildPrefixes, chromeosBuildParts[0]) {
				chromeosBuildParts = chromeosBuildParts[1:]
			}
			if len(chromeosBuildParts) <= 1 {
				return ""
			}
			// Remove base board and build type.
			variantParts := chromeosBuildParts[1 : len(chromeosBuildParts)-1]
			if len(variantParts) == 0 {
				return ""
			}
			// Strip excluded variant post-fixes.
			if slices.Contains(ExcludedVariantPostfixes, variantParts[len(variantParts)-1]) {
				variantParts = variantParts[:len(variantParts)-1]
			}
			return strings.Join(variantParts, "-")
		}
	}
	return ""
}

// getImageGcsPath parses through the software dependencies
// to build out the gcs image path for this request.
func getImageGcsPath(softwareDeps []*test_platform.Request_Params_SoftwareDependency) string {
	chromeosBuild := ""
	chromeosBuildGcsBucket := ""
	for _, softwareDep := range softwareDeps {
		switch dep := softwareDep.GetDep().(type) {
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuild:
			chromeosBuild = dep.ChromeosBuild
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuildGcsBucket:
			chromeosBuildGcsBucket = dep.ChromeosBuildGcsBucket
		case *test_platform.Request_Params_SoftwareDependency_LacrosGcsPath:
			return dep.LacrosGcsPath
		}
	}
	if chromeosBuild == "" {
		return ""
	}
	if chromeosBuildGcsBucket == "" {
		chromeosBuildGcsBucket = DefaultChromeosBuildGcsBucket
	}
	return fmt.Sprintf("gs://%s/%s", chromeosBuildGcsBucket, chromeosBuild)
}

// getTag parses a list of tags in the format of "k:v"
func getTag(tags []string, targetTag string) string {
	for _, tag := range tags {
		splitTag := strings.Split(tag, ":")
		if len(splitTag) == 2 && splitTag[0] == targetTag {
			return splitTag[1]
		}
	}
	return ""
}

func getIsALRun(v1 *test_platform.Request) bool {
	suites := v1.GetTestPlan().GetSuite()
	if len(suites) > 0 && strings.HasPrefix(strings.ToLower(suites[0].GetName()), "al.") {
		return true
	}
	return false
}
