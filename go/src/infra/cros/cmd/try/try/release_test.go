// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"infra/cros/internal/assert"
	"infra/cros/internal/cmd"
	"infra/cros/internal/gerrit"
	bb "infra/cros/lib/buildbucket"
)

// TestGetReleaseOrchestratorName tests getReleaseOrchestratorName.
func TestGetReleaseOrchestratorName(t *testing.T) {
	t.Parallel()
	for i, testCase := range []struct {
		production bool
		dev        bool
		branch     string
		expected   string
	}{
		{false, false, "main", "chromeos/try-preprod/staging-release-main-orchestrator"},
		{true, false, "main", "chromeos/release/release-main-orchestrator"},
		{false, false, "release-R106.15054.B", "chromeos/try-preprod/staging-release-R106.15054.B-orchestrator"},
		{true, false, "release-R106.15054.B", "chromeos/release/release-R106.15054.B-orchestrator"},
		{false, false, "release-R106.15054.B", "chromeos/try-preprod/staging-release-R106.15054.B-orchestrator"},
		{false, true, "main", "chromeos/try-dev/staging-release-main-orchestrator"},
		{false, true, "release-R106.15054.B", "chromeos/try-dev/staging-release-R106.15054.B-orchestrator"},
	} {
		r := releaseRun{
			tryRunBase: tryRunBase{
				branch:     testCase.branch,
				production: testCase.production,
			},
			dev: testCase.dev,
		}
		if actual := r.getReleaseOrchestratorName(); actual != testCase.expected {
			t.Errorf("#%d: Incorrect release orch name: got %s; want %s", i, actual, testCase.expected)
		}
	}
}

func TestGetReleaseBuilderNames(t *testing.T) {
	t.Parallel()
	for i, testCase := range []struct {
		staging      bool
		branch       string
		buildTargets []string
		expected     []string
	}{
		{false, "main", []string{"eve", "kevin-kernelnext"}, []string{"eve-release-main", "kevin-kernelnext-release-main"}},
		{true, "main", []string{"eve", "kevin-kernelnext"}, []string{"staging-eve-release-main", "staging-kevin-kernelnext-release-main"}},
		{false, "release-R106.15054.B", []string{"eve", "kevin-kernelnext"}, []string{"eve-release-R106.15054.B", "kevin-kernelnext-release-R106.15054.B"}},
		{true, "release-R106.15054.B", []string{"eve", "kevin-kernelnext"}, []string{"staging-eve-release-R106.15054.B", "staging-kevin-kernelnext-release-R106.15054.B"}},
	} {
		r := releaseRun{
			tryRunBase: tryRunBase{
				branch:       testCase.branch,
				production:   !testCase.staging,
				buildTargets: testCase.buildTargets,
			},
		}
		if actual := r.getReleaseBuilderNames(); !reflect.DeepEqual(actual, testCase.expected) {
			t.Errorf("#%d: Incorrect release builder names: got %s; want %s", i, actual, testCase.expected)
		}
	}
}

func TestValidate_releaseRun(t *testing.T) {
	t.Parallel()
	r := releaseRun{
		tryRunBase: tryRunBase{
			branch:     "release-R106.15054.B",
			production: true,
		},
		skipPaygen: true,
	}
	assert.ErrorContains(t, r.validate(), "not supported for production")
}

func TestValidate_stabilizeRun(t *testing.T) {
	t.Parallel()
	r := releaseRun{
		tryRunBase: tryRunBase{
			branch:     "stabilize-15185.B",
			production: false,
		},
	}
	assert.NilError(t, r.validate())

	r = releaseRun{
		tryRunBase: tryRunBase{
			branch:     "stabilize-15185.B",
			production: true,
		},
	}
	assert.NilError(t, r.validate())
}

type runTestConfig struct {
	// Whether to fail the child build check. Only used if buildTargets is set.
	failChildCheck bool
	// e.g. ["crrev.com/c/1234567"]
	patches []string
	// The output for GetRelatedChanges for this path.
	actualRelatedChanges map[string]map[int][]gerrit.Change
	// Expected patches after including all ancestors.
	expectedPatches []string
	// e.g. "eve"
	buildTargets []string
	// e.g. staging-release-R106.15054.B-orchestrator
	expectedOrch string
	// e.g. "staging-eve-release-R106.15054.B"
	expectedChildren        []string
	skipPaygen              bool
	production              bool
	dev                     bool
	dryrun                  bool
	branch                  string
	channelOverride         string
	expectedChannelOverride []string
}

func doTestRun(t *testing.T, tc *runTestConfig) {
	t.Helper()
	propsFile, err := os.CreateTemp("", "input_props")
	defer os.Remove(propsFile.Name())
	assert.NilError(t, err)

	var expectedBucket string
	expectedBuilder := tc.expectedOrch
	if tc.production {
		expectedBucket = "chromeos/release"
	} else if tc.dev {
		expectedBucket = "chromeos/try-dev"
	} else {
		expectedBucket = "chromeos/try-preprod"
	}

	f := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			bb.FakeWhichRunner("bb", 0),
			bb.FakeAuthInfoRunner("bb", 0),
			bb.FakeWhichRunner("led", 0),
			bb.FakeAuthInfoRunner("led", 0),
			bb.FakeAuthInfoRunnerSuccessStdout("led", "sundar@google.com"),
		},
	}
	for _, childBuilder := range tc.expectedChildren {
		f.CommandRunners = append(
			f.CommandRunners,
			*fakeLEDGetBuilderRunner(
				expectedBucket,
				childBuilder,
				!tc.failChildCheck,
			),
		)
	}
	f.CommandRunners = append(
		f.CommandRunners,
		*fakeLEDGetBuilderRunner(expectedBucket, expectedBuilder, true),
	)
	expectedAddCmd := []string{"bb", "add", fmt.Sprintf("%s/%s", expectedBucket, expectedBuilder)}
	expectedAddCmd = append(expectedAddCmd, "-t", "tryjob-launcher:sundar@google.com")
	if tc.expectedPatches == nil || len(tc.expectedPatches) == 0 {
		tc.expectedPatches = tc.patches
	}
	for _, patch := range tc.expectedPatches {
		expectedAddCmd = append(expectedAddCmd, "-cl", patch)
	}
	expectedAddCmd = append(expectedAddCmd, "-p", fmt.Sprintf("@%s", propsFile.Name()))
	if !tc.dryrun {
		f.CommandRunners = append(f.CommandRunners, bb.FakeBBAddRunner(expectedAddCmd, "12345"))
	}

	r := releaseRun{
		propsFile: propsFile,
		tryRunBase: tryRunBase{
			cmdRunner: f,
			gerritClient: &gerrit.MockClient{
				ExpectedRelatedChanges: tc.actualRelatedChanges,
			},
			dryrun:               tc.dryrun,
			branch:               tc.branch,
			production:           tc.production,
			patches:              tc.patches,
			buildTargets:         tc.buildTargets,
			skipProductionPrompt: true,
		},
		dev:             tc.dev,
		useProdTests:    true,
		channelOverride: tc.channelOverride,
	}
	ret := r.Run(nil, nil, nil)
	if tc.failChildCheck {
		assert.IntsEqual(t, ret, CmdError)
		return
	} else {
		assert.IntsEqual(t, ret, Success)
	}

	properties, err := bb.ReadStructFromFile(propsFile.Name())
	assert.NilError(t, err)

	if len(tc.buildTargets) > 0 {
		if len(tc.buildTargets) != len(tc.expectedChildren) {
			t.Fatalf("len(buildTargets) != len(expectedChildren), invalid test")
		}
		child_builds := properties.GetFields()["$chromeos/orch_menu"].GetStructValue().GetFields()["child_builds"].GetListValue().AsSlice()
		assert.StringArrsEqual(t, interfaceSliceToStr(child_builds), tc.expectedChildren)
	} else {
		_, exists := properties.GetFields()["$chromeos/orch_menu"].GetStructValue().GetFields()["child_builds"]
		assert.Assert(t, !exists)
	}

	skipPaygen, exists := properties.GetFields()["$chromeos/orch_menu"].GetStructValue().GetFields()["skip_paygen"]
	if !tc.skipPaygen {
		assert.Assert(t, !exists)
	} else {
		assert.Assert(t, exists && skipPaygen.GetBoolValue())
	}

	shouldOverrideChannels, exists := properties.GetFields()["$chromeos/cros_infra_config"].GetStructValue().GetFields()["should_override_release_channels"]

	overrideChannels := properties.GetFields()["$chromeos/cros_infra_config"].GetStructValue().GetFields()["override_release_channels"].GetListValue().AsSlice()

	if len(tc.channelOverride) > 0 {
		channelOverrideList := strings.Split(tc.channelOverride, ",")
		if len(channelOverrideList) != len(tc.expectedChannelOverride) {
			t.Fatalf("len(channelOverride) != len(expectedChannelOverride), invalid test")
		}
		assert.Assert(t, exists && shouldOverrideChannels.GetBoolValue())
		assert.StringArrsEqual(t, interfaceSliceToStr(overrideChannels), tc.expectedChannelOverride)
	} else {
		assert.Assert(t, !exists)
	}

	noPublicBuild, exists := properties.GetFields()["$chromeos/orch_menu"].GetStructValue().GetFields()["schedule_public_build"]
	assert.Assert(t, exists && !noPublicBuild.GetBoolValue())

	disable_build_plan_pruning, exists := properties.GetFields()["$chromeos/build_plan"].GetStructValue().GetFields()["disable_build_plan_pruning"]
	if len(tc.patches) > 0 {
		assert.Assert(t, disable_build_plan_pruning.GetBoolValue())
	} else {
		assert.Assert(t, !exists)
	}

	use_prod_tests := properties.GetFields()["$chromeos/cros_test_plan"].GetStructValue().GetFields()["use_prod_config"].GetBoolValue()
	assert.Assert(t, use_prod_tests)

	supportedBuild, exists := properties.GetFields()["$chromeos/cros_try"].GetStructValue().GetFields()["supported_build"]
	assert.Assert(t, exists && supportedBuild.GetBoolValue())

	buildFailuresFatal, exists := properties.GetFields()["build_failures_fatal"]
	assert.Assert(t, exists && buildFailuresFatal.GetBoolValue())
}

func TestRun_dryrun(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:       "release-R106.15054.B",
		dryrun:       true,
		expectedOrch: "staging-release-R106.15054.B-orchestrator",
	})
}

func TestRun_staging_noBuildTargets(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:       "release-R106.15054.B",
		skipPaygen:   false,
		expectedOrch: "staging-release-R106.15054.B-orchestrator",
	})
}

func TestRun_staging_buildTargets(t *testing.T) {
	doTestRun(t, &runTestConfig{
		branch:           "release-R106.15054.B",
		buildTargets:     []string{"eve", "kevin-kernelnext"},
		expectedOrch:     "staging-release-R106.15054.B-orchestrator",
		expectedChildren: []string{"staging-eve-release-R106.15054.B", "staging-kevin-kernelnext-release-R106.15054.B"},
	})
}

func TestRun_staging_buildTargets_fail(t *testing.T) {
	doTestRun(t, &runTestConfig{
		failChildCheck:   true,
		branch:           "release-R106.15054.B",
		buildTargets:     []string{"eve", "kevin-kernelnext"},
		expectedOrch:     "staging-release-R106.15054.B-orchestrator",
		expectedChildren: []string{"staging-eve-release-R106.15054.B", "staging-kevin-kernelnext-release-R106.15054.B"},
	})
}

func TestRun_dev(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:           "release-R106.15054.B",
		dev:              true,
		buildTargets:     []string{"eve", "kevin-kernelnext"},
		expectedOrch:     "staging-release-R106.15054.B-orchestrator",
		expectedChildren: []string{"staging-eve-release-R106.15054.B", "staging-kevin-kernelnext-release-R106.15054.B"},
	})
}

func TestRun_production(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:           "release-R106.15054.B",
		production:       true,
		buildTargets:     []string{"eve", "kevin-kernelnext"},
		expectedOrch:     "release-R106.15054.B-orchestrator",
		expectedChildren: []string{"eve-release-R106.15054.B", "kevin-kernelnext-release-R106.15054.B"},
	})
}

func TestRun_stabilize(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:           "stabilize-15185.B",
		production:       true,
		buildTargets:     []string{"eve", "kevin-kernelnext"},
		expectedOrch:     "release-stabilize-15185.B-orchestrator",
		expectedChildren: []string{"eve-release-stabilize-15185.B", "kevin-kernelnext-release-stabilize-15185.B"},
	})
}

func TestRun_patches(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:       "release-R106.15054.B",
		skipPaygen:   false,
		expectedOrch: "staging-release-R106.15054.B-orchestrator",
		patches:      []string{"crrev.com/c/1234567"},
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {1234567: {}},
		},
	})
}

func TestRun_patches_withAncestors(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:       "release-R106.15054.B",
		skipPaygen:   false,
		expectedOrch: "staging-release-R106.15054.B-orchestrator",
		patches:      []string{"crrev.com/c/1234567"},
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {
				1234567: {{ChangeNumber: 1234565}, {ChangeNumber: 1234567}, {ChangeNumber: 1234568}, {ChangeNumber: 1234560}},
			},
		},
		expectedPatches: []string{"crrev.com/c/1234560", "crrev.com/c/1234568", "crrev.com/c/1234567"},
	})
}

func TestRun_channelOverride(t *testing.T) {
	t.Parallel()
	doTestRun(t, &runTestConfig{
		branch:                  "release-R106.15054.B",
		production:              true,
		buildTargets:            []string{"eve", "kevin-kernelnext"},
		expectedOrch:            "release-R106.15054.B-orchestrator",
		expectedChildren:        []string{"eve-release-R106.15054.B", "kevin-kernelnext-release-R106.15054.B"},
		channelOverride:         "dev,beta,1",
		expectedChannelOverride: []string{"3", "2", "1"},
	})
}
