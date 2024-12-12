// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"fmt"
	"os"
	"testing"

	"infra/cros/internal/assert"
	"infra/cros/internal/cmd"
	"infra/cros/internal/gerrit"
	bb "infra/cros/lib/buildbucket"
)

// TestChromiumOSSDKGetBuilderFullName tests chromiumosSDKRun.getBuilderFullName.
func TestChromiumOSSDKGetBuilderFullName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		production          bool
		expectedBuilderName string
	}{
		{true, "chromeos/infra/build-chromiumos-sdk"},
		{false, "chromeos/staging/staging-build-chromiumos-sdk"},
	} {
		run := chromiumOSSDKRun{
			tryRunBase: tryRunBase{
				production: tc.production,
			},
		}
		actualBuilderName := run.getBuilderFullName()
		assert.StringsEqual(t, actualBuilderName, tc.expectedBuilderName)
	}
}

// chromiumOSSDKRunTestConfig contains info for an end-to-end test of chromiumOSSDKRun.Run().
type chromiumOSSDKRunTestConfig struct {
	production            bool
	expectedBucket        string
	expectedBuilder       string
	branch                string
	launchPUpr            bool
	cqPolicy              string
	expectedCQPolicyValue int
	expectedReviewerEmail string
	patches               []string
	// The output for GetRelatedChanges for this path.
	actualRelatedChanges map[string]map[int][]gerrit.Change
	// Expected patches after including all ancestors.
	expectedPatches []string
}

func doChromiumOSSDKRun(t *testing.T, tc chromiumOSSDKRunTestConfig) {
	t.Helper()

	// Set up properties tempfile.
	propsFile, err := os.CreateTemp("", "input_props")
	defer os.Remove(propsFile.Name())
	assert.NilError(t, err)

	// Set up fake commands.
	expectedAddCmd := []string{
		"bb",
		"add",
		fmt.Sprintf("%s/%s", tc.expectedBucket, tc.expectedBuilder),
		"-t",
		"tryjob-launcher:sundar@google.com",
		"-p",
		"@" + propsFile.Name(),
	}
	if tc.expectedPatches == nil || len(tc.expectedPatches) == 0 {
		tc.expectedPatches = tc.patches
	}
	for _, patch := range tc.expectedPatches {
		expectedAddCmd = append(expectedAddCmd, "-cl", patch)
	}

	cmdRunner := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			bb.FakeWhichRunner("bb", 0),
			bb.FakeAuthInfoRunner("bb", 0),
			bb.FakeWhichRunner("led", 0),
			bb.FakeAuthInfoRunner("led", 0),
			bb.FakeAuthInfoRunnerSuccessStdout("led", "sundar@google.com"),
			bb.FakeAuthInfoRunnerSuccessStdout("led", "sundar@google.com"),
			*fakeLEDGetBuilderRunner(tc.expectedBucket, tc.expectedBuilder, true),
			bb.FakeBBAddRunner(
				expectedAddCmd,
				"12345",
			),
		},
	}

	// Set up fake chromiumOSSDKRun.
	run := chromiumOSSDKRun{
		tryRunBase: tryRunBase{
			cmdRunner: cmdRunner,
			gerritClient: &gerrit.MockClient{
				ExpectedRelatedChanges: tc.actualRelatedChanges,
			},
			production:           tc.production,
			skipProductionPrompt: true,
			branch:               tc.branch,
			patches:              tc.patches,
		},
		launchPUpr: tc.launchPUpr,
		cqPolicy:   tc.cqPolicy,
		propsFile:  propsFile,
	}

	// Try running!
	ret := run.Run(nil, nil, nil)
	assert.IntsEqual(t, ret, Success)

	// Inspect properties.
	propsStruct, err := bb.ReadStructFromFile(propsFile.Name())
	assert.NilError(t, err)
	properties := propsStruct.GetFields()

	branch, exists := properties["manifest_branch"]
	if tc.branch == "" {
		assert.Assert(t, !exists)
	} else {
		assert.Assert(t, exists)
		assert.Assert(t, tc.branch == branch.GetStringValue())
	}

	launchPUpr, exists := properties["launch_pupr"]
	assert.Assert(t, exists)
	assert.Assert(t, tc.launchPUpr == launchPUpr.GetBoolValue())

	branchPolicy, exists := properties["pupr_branch_policy"]
	if tc.cqPolicy == "" {
		assert.Assert(t, !exists)
	} else {
		t.Log("Checking branch policy, which should exist.")
		assert.Assert(t, exists)
		bpStruct := branchPolicy.GetStructValue()
		bpFields := bpStruct.GetFields()

		t.Logf("Checking existing_cls_policy, which should be %d.", tc.expectedCQPolicyValue)
		existingCLsPolicy, exists := bpFields["existing_cls_policy"]
		assert.Assert(t, exists)
		assert.Assert(t, tc.expectedCQPolicyValue == int(existingCLsPolicy.GetNumberValue()))

		t.Logf("Checking no_existing_cls_policy, which should be %d.", tc.expectedCQPolicyValue)
		noExistingCLsPolicy, exists := bpFields["no_existing_cls_policy"]
		assert.Assert(t, exists)
		assert.Assert(t, tc.expectedCQPolicyValue == int(noExistingCLsPolicy.GetNumberValue()))

		reviewers, exists := bpFields["reviewers"]
		assert.Assert(t, exists)
		reviewersSlice := reviewers.GetListValue().AsSlice()
		assert.Assert(t, len(reviewersSlice) == 1)
		reviewerMap, ok := reviewersSlice[0].(map[string]interface{})
		assert.Assert(t, ok)
		assert.Assert(t, tc.expectedReviewerEmail == reviewerMap["email"])
	}
}

// TestChromiumOSSDKRun_Production is an end-to-end test of chromiumOSSDKRun.Run() for a production build.
func TestChromiumOSSDKRun_Production(t *testing.T) {
	t.Parallel()
	tc := chromiumOSSDKRunTestConfig{
		production:      true,
		expectedBucket:  "chromeos/infra",
		expectedBuilder: "build-chromiumos-sdk",
	}
	doChromiumOSSDKRun(t, tc)
}

// TestChromiumOSSDKRun_Staging is an end-to-end test of chromiumOSSDKRun.Run() for a staging build.
func TestChromiumOSSDKRun_Staging(t *testing.T) {
	t.Parallel()
	tc := chromiumOSSDKRunTestConfig{
		production:      false,
		expectedBucket:  "chromeos/staging",
		expectedBuilder: "staging-build-chromiumos-sdk",
		patches:         []string{"crrev.com/c/1234567"},
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {
				1234567: {},
			},
		},
		expectedPatches: []string{"crrev.com/c/1234567"},
	}
	doChromiumOSSDKRun(t, tc)
}

// TestChromiumOSSDKRun_Staging_includeAncestors is an end-to-end test of chromiumOSSDKRun.Run() for a staging build.
// In this test, the passed patch has ancestors that should be included.
func TestChromiumOSSDKRun_Staging_includeAncestors(t *testing.T) {
	t.Parallel()
	tc := chromiumOSSDKRunTestConfig{
		production:      false,
		expectedBucket:  "chromeos/staging",
		expectedBuilder: "staging-build-chromiumos-sdk",
		patches:         []string{"crrev.com/c/1234567"},
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {
				1234567: {{ChangeNumber: 1234565}, {ChangeNumber: 1234567}, {ChangeNumber: 1234568}, {ChangeNumber: 1234560}},
			},
		},
		expectedPatches: []string{"crrev.com/c/1234560", "crrev.com/c/1234568", "crrev.com/c/1234567"},
	}
	doChromiumOSSDKRun(t, tc)
}

func TestChromiumOSSDKRun_Branch(t *testing.T) {
	t.Parallel()
	tc := chromiumOSSDKRunTestConfig{
		production:      true,
		expectedBucket:  "chromeos/infra",
		expectedBuilder: "build-chromiumos-sdk",
		branch:          "stabilize-10000.B",
	}
	doChromiumOSSDKRun(t, tc)
}

func TestChromiumOSSDKRun_LaunchPUpr(t *testing.T) {
	t.Parallel()
	tc := chromiumOSSDKRunTestConfig{
		production:      false,
		expectedBucket:  "chromeos/staging",
		expectedBuilder: "staging-build-chromiumos-sdk",
		launchPUpr:      true,
	}
	doChromiumOSSDKRun(t, tc)
}

func TestChromiumOSSDKRun_CQPolicy(t *testing.T) {
	t.Parallel()
	tc := chromiumOSSDKRunTestConfig{
		production:            false,
		expectedBucket:        "chromeos/staging",
		expectedBuilder:       "staging-build-chromiumos-sdk",
		cqPolicy:              "dry-run",
		expectedCQPolicyValue: 2, // Based on generator.proto
		expectedReviewerEmail: "sundar@google.com",
	}
	doChromiumOSSDKRun(t, tc)
}
