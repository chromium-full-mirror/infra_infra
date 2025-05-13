// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

// TestDoesFWBranchHaveBuilder tests doesFWBranchHaveBuilder.
func TestDoesFWBranchHaveBuilder(t *testing.T) {
	t.Parallel()
	const (
		eveBranch    = "firmware-eve-9584.B"
		gruntBranch  = "firmware-grunt-11031.B"
		namiBranch   = "firmware-nami-10775.B"
		eveBuilder   = "chromeos/firmware/firmware-eve-9584.B-branch"
		gruntBuilder = "chromeos/firmware/firmware-grunt-11031.B-branch"
		namiBuilder  = "chromeos/firmware/firmware-nami-10775.B-branch"
	)
	cmdRunner := fakeBBBuildersRunner("chromeos/firmware", []string{eveBuilder, gruntBuilder})
	f := firmwareRun{
		tryRunBase: tryRunBase{
			cmdRunner: cmdRunner,
			bbClient:  bb.NewClient(cmdRunner, nil, nil),
		},
	}
	ctx := context.Background()
	for i, tc := range []struct {
		branch     string
		production bool
		expected   bool
	}{
		{eveBranch, true, true},
		{namiBranch, true, false},
	} {
		builderExists, err := f.doesFWBranchHaveBuilder(ctx, tc.branch, !tc.production)
		if err != nil {
			t.Errorf("#%d: Unexpected error calling doesFWBranchHaveBuilder: %+v", i, err)
		}
		if builderExists != tc.expected {
			t.Errorf("#%d: Unexpected response from doesFWBranchHaveBuilder: got %v; want %v", i, builderExists, tc.expected)
		}
	}
}

// TestGetFWBuilderFullName tests getFWBuilderFullName.
func TestGetFWBuilderFullName(t *testing.T) {
	t.Parallel()
	const (
		eveBranch         = "firmware-eve-9584.B"
		eveBuilder        = "chromeos/firmware/firmware-eve-9584.B-branch"
		eveStagingBuilder = "chromeos/staging/staging-firmware-eve-9584.B-branch"
	)
	assert.StringsEqual(t, getFWBuilderFullName(eveBranch, false), eveBuilder)
	assert.StringsEqual(t, getFWBuilderFullName(eveBranch, true), eveStagingBuilder)
}

func TestValidate_firmwareRun(t *testing.T) {
	t.Parallel()
	const (
		eveFWBuilder  = "chromeos/firmware/firmware-eve-9584.B-branch"
		eveFWBranch   = "firmware-eve-9584.B"
		gruntFWBranch = "firmware-grunt-11031.B"
		releaseBranch = "release-R106.15054.B"
	)
	ctx := context.Background()

	// Test the good workflow
	cmdRunner := fakeBBBuildersRunner("chromeos/firmware", []string{eveFWBuilder})
	f := firmwareRun{
		tryRunBase: tryRunBase{
			branch:     eveFWBranch,
			production: true,
			cmdRunner:  cmdRunner,
			bbClient:   bb.NewClient(cmdRunner, nil, nil),
		},
	}
	assert.NilError(t, f.validate(ctx))

	// No branch provided
	f.tryRunBase.branch = ""
	assert.NonNilError(t, f.validate(ctx))

	// Non-firmware branch
	f.tryRunBase.branch = releaseBranch
	assert.NonNilError(t, f.validate(ctx))

	// Firmware branch that doesn't have a builder
	f.tryRunBase.branch = gruntFWBranch
	assert.NonNilError(t, f.validate(ctx))

	// Patch set provided for production builder
	f.tryRunBase.branch = eveFWBranch
	f.tryRunBase.patches = []string{"crrev.com/c/1234567"}
	assert.NonNilError(t, f.validate(ctx))

	// Patch set provided for staging builder
	f.tryRunBase.production = false
	f.cmdRunner = fakeBBBuildersRunner("chromeos/staging", []string{"chromeos/staging/staging-firmware-eve-9584.B-branch"})
	f.bbClient = bb.NewClient(f.cmdRunner, nil, nil)
	assert.NilError(t, f.validate(ctx))
}

type firmwareTestConfig struct {
	// e.g. ["crrev.com/c/1234567"]
	patches []string
	// The output for GetRelatedChanges for this path.
	actualRelatedChanges map[string]map[int][]gerrit.Change
	// Expected patches after including all ancestors.
	expectedPatches []string
	// e.g. staging-release-R106.15054.B-orchestrator
	expectedBuilder string
	production      bool
	dryrun          bool
	publish         bool
	expectedPublish bool
	branch          string
}

func doFirmwareTest(t *testing.T, tc *firmwareTestConfig) {
	t.Helper()
	propsFile, err := os.CreateTemp("", "input_props")
	defer os.Remove(propsFile.Name())
	assert.NilError(t, err)

	var expectedBucket string
	expectedBuilder := tc.expectedBuilder
	if tc.production {
		expectedBucket = "chromeos/firmware"
	} else {
		expectedBucket = "chromeos/staging"
	}
	expectedPublish := tc.expectedPublish

	f := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			bb.FakeWhichRunner("bb", 0),
			bb.FakeAuthInfoRunner("bb", 0),
			bb.FakeWhichRunner("led", 0),
			bb.FakeAuthInfoRunner("led", 0),
			bb.FakeAuthInfoRunnerSuccessStdout("led", "sundar@google.com"),
			*fakeBBBuildersRunner(
				expectedBucket,
				[]string{"foo", fmt.Sprintf("%s/%s", expectedBucket, tc.expectedBuilder), "bar"},
			),
		},
	}
	f.CommandRunners = append(
		f.CommandRunners,
		*fakeLEDGetBuilderRunner(expectedBucket, expectedBuilder, true),
	)
	expectedAddCmd := []string{"bb", "add", fmt.Sprintf("%s/%s", expectedBucket, expectedBuilder)}
	expectedAddCmd = append(expectedAddCmd, "-t", "tryjob-launcher:sundar@google.com")
	if len(tc.expectedPatches) == 0 {
		tc.expectedPatches = tc.patches
	}
	for _, patch := range tc.expectedPatches {
		expectedAddCmd = append(expectedAddCmd, "-cl", patch)
	}
	expectedAddCmd = append(expectedAddCmd, "-p", fmt.Sprintf("@%s", propsFile.Name()))
	if !tc.dryrun {
		f.CommandRunners = append(f.CommandRunners, bb.FakeBBAddRunner(expectedAddCmd, "12345"))
	}

	r := firmwareRun{
		tryRunBase: tryRunBase{
			cmdRunner: f,
			gerritClient: &gerrit.MockClient{
				ExpectedRelatedChanges: tc.actualRelatedChanges,
			},
			branch:               tc.branch,
			production:           tc.production,
			patches:              tc.patches,
			publish:              tc.publish,
			skipProductionPrompt: true,
		},
		propsFile: propsFile,
	}
	ret := r.Run(nil, nil, nil)
	assert.IntsEqual(t, ret, Success)

	properties, err := bb.ReadStructFromFile(propsFile.Name())
	assert.NilError(t, err)

	skipPublish, exists := properties.GetFields()["$chromeos/cros_artifacts"].GetStructValue().GetFields()["skip_publish"]
	if !expectedPublish {
		assert.Assert(t, exists && skipPublish.GetBoolValue())
	} else {
		assert.Assert(t, !exists || !skipPublish.GetBoolValue())
	}
}

func TestFirmware_production(t *testing.T) {
	t.Parallel()
	doFirmwareTest(t, &firmwareTestConfig{
		branch:          "firmware-nissa-15217.B",
		expectedBuilder: "firmware-nissa-15217.B-branch",
		production:      true,
	})
}
func TestFirmware_staging(t *testing.T) {
	t.Parallel()
	doFirmwareTest(t, &firmwareTestConfig{
		patches:         []string{"crrev.com/c/1234567"},
		branch:          "firmware-nissa-15217.B",
		expectedBuilder: "staging-firmware-nissa-15217.B-branch",
		production:      false,
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {1234567: {}},
		},
	})
}

func TestFirmware_publish(t *testing.T) {
	t.Parallel()
	doFirmwareTest(t, &firmwareTestConfig{
		branch:          "firmware-nissa-15217.B",
		expectedBuilder: "staging-firmware-nissa-15217.B-branch",
		production:      false,
		publish:         true,
		expectedPublish: true,
	})
}

func TestFirmware_patches_withAncestors(t *testing.T) {
	t.Parallel()
	doFirmwareTest(t, &firmwareTestConfig{
		patches:         []string{"crrev.com/c/1234567"},
		branch:          "firmware-nissa-15217.B",
		expectedBuilder: "staging-firmware-nissa-15217.B-branch",
		production:      false,
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {
				1234567: {{ChangeNumber: 1234565}, {ChangeNumber: 1234567}, {ChangeNumber: 1234568}, {ChangeNumber: 1234560}},
			},
		},
		expectedPatches: []string{"crrev.com/c/1234560", "crrev.com/c/1234568", "crrev.com/c/1234567"},
	})
}
