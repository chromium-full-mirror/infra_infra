// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package try

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"infra/cros/internal/assert"
	"infra/cros/internal/cmd"
	"infra/cros/internal/gerrit"
	bb "infra/cros/lib/buildbucket"
)

// TestDoesFactoryBranchHaveBuilder tests doesFactoryBranchHaveBuilder.
func TestDoesFactoryBranchHaveBuilder(t *testing.T) {
	t.Parallel()
	const (
		nissaBranch  = "factory-nissa-15199.B"
		bryaBranch   = "factory-brya-14517.B"
		zorkBranch   = "factory-zork-13427.B"
		nissaBuilder = "chromeos/factory/factory-nissa-15199.B-orchestrator"
		bryaBuilder  = "chromeos/factory/factory-brya-14517.B-brya"
		zorkBuilder  = "chromeos/factory/factory-zork-13427.B-zork"
	)
	cmdRunner := fakeBBBuildersRunner("chromeos/factory", []string{nissaBuilder, bryaBuilder})
	f := factoryRun{
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
		{nissaBranch, true, true},
		{zorkBranch, true, false},
	} {
		builderExists, err := f.doesFactoryBranchHaveBuilder(ctx, tc.branch, !tc.production)
		if err != nil {
			t.Errorf("#%d: Unexpected error calling doesFactoryBranchHaveBuilder: %+v", i, err)
		}
		if builderExists != tc.expected {
			t.Errorf("#%d: Unexpected response from doesFactoryBranchHaveBuilder: got %v; want %v", i, builderExists, tc.expected)
		}
	}
}

// TestGetFactoryBuilderFullName tests getFactoryBuilderFullName.
func TestGetFactoryBuilderFullName(t *testing.T) {
	t.Parallel()
	const (
		nissaBranch         = "factory-nissa-15199.B"
		nissaBuilder        = "chromeos/factory/factory-nissa-15199.B-orchestrator"
		nissaStagingBuilder = "chromeos/staging/staging-factory-nissa-15199.B-orchestrator"
	)
	assert.StringsEqual(t, getFactoryBuilderFullName(nissaBranch, false), nissaBuilder)
	assert.StringsEqual(t, getFactoryBuilderFullName(nissaBranch, true), nissaStagingBuilder)
}

func TestValidate_factoryRun(t *testing.T) {
	t.Parallel()
	const (
		nissaFactoryBuilder = "chromeos/factory/factory-nissa-15199.B-orchestrator"
		nissaFactoryBranch  = "factory-nissa-15199.B"
		bryaFactoryBranch   = "factory-brya-14517.B"
		releaseBranch       = "release-R106.15054.B"
	)
	ctx := context.Background()

	// Test the good workflow
	cmdRunner := fakeBBBuildersRunner("chromeos/factory", []string{nissaFactoryBuilder})
	f := factoryRun{
		tryRunBase: tryRunBase{
			branch:     nissaFactoryBranch,
			production: true,
			cmdRunner:  cmdRunner,
			bbClient:   bb.NewClient(cmdRunner, nil, nil),
		},
	}
	assert.NilError(t, f.validate(ctx))

	// No branch provided
	f.tryRunBase.branch = ""
	assert.NonNilError(t, f.validate(ctx))

	// Non-factory branch
	f.tryRunBase.branch = releaseBranch
	assert.NonNilError(t, f.validate(ctx))

	// factory branch that doesn't have a builder
	f.tryRunBase.branch = bryaFactoryBranch
	assert.NonNilError(t, f.validate(ctx))

	// Patch set provided for production builder
	f.tryRunBase.branch = nissaFactoryBranch
	f.tryRunBase.patches = []string{"crrev.com/c/1234567"}
	assert.NonNilError(t, f.validate(ctx))

	// Patch set provided for staging builder
	f.tryRunBase.production = false
	f.cmdRunner = fakeBBBuildersRunner("chromeos/staging", []string{"chromeos/staging/staging-factory-nissa-15199.B-orchestrator"})
	f.bbClient = bb.NewClient(f.cmdRunner, nil, nil)
	assert.NilError(t, f.validate(ctx))
}

type factoryTestConfig struct {
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

func dofactoryTest(t *testing.T, tc *factoryTestConfig) {
	t.Helper()
	tempDir := t.TempDir()
	propsFilePath := filepath.Join(tempDir, "input_props")
	propsFile, err := os.Create(propsFilePath)
	assert.NilError(t, err)

	var expectedBucket string
	expectedBuilder := tc.expectedBuilder
	if tc.production {
		expectedBucket = "chromeos/factory"
	} else {
		expectedBucket = "chromeos/staging"
	}
	expectedPublish := tc.expectedPublish

	f := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			bb.FakeAuthInfoRunner("bb", 0),
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

	r := factoryRun{
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

func TestFactory_production(t *testing.T) {
	t.Parallel()
	dofactoryTest(t, &factoryTestConfig{
		branch:          "factory-nissa-15199.B",
		expectedBuilder: "factory-nissa-15199.B-orchestrator",
		production:      true,
	})
}
func TestFactory_staging(t *testing.T) {
	t.Parallel()
	dofactoryTest(t, &factoryTestConfig{
		patches:         []string{"crrev.com/c/1234567"},
		branch:          "factory-nissa-15199.B",
		expectedBuilder: "staging-factory-nissa-15199.B-orchestrator",
		production:      false,
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {1234567: {}},
		},
	})
}

func TestFactory_publish(t *testing.T) {
	t.Parallel()
	dofactoryTest(t, &factoryTestConfig{
		branch:          "factory-nissa-15199.B",
		expectedBuilder: "staging-factory-nissa-15199.B-orchestrator",
		production:      false,
		publish:         true,
		expectedPublish: true,
	})
}

func TestFactory_patches_withAncestors(t *testing.T) {
	t.Parallel()
	dofactoryTest(t, &factoryTestConfig{
		patches:         []string{"crrev.com/c/1234567"},
		branch:          "factory-nissa-15199.B",
		expectedBuilder: "staging-factory-nissa-15199.B-orchestrator",
		production:      false,
		actualRelatedChanges: map[string]map[int][]gerrit.Change{
			"https://chromium-review.googlesource.com": {
				1234567: {{ChangeNumber: 1234565}, {ChangeNumber: 1234567}, {ChangeNumber: 1234568}, {ChangeNumber: 1234560}},
			},
		},
		expectedPatches: []string{"crrev.com/c/1234560", "crrev.com/c/1234568", "crrev.com/c/1234567"},
	})
}
