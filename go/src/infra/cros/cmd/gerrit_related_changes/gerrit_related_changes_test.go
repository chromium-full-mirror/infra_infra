// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
)

// gerritChangesToStr is a helper function to compare actual and expected
// related change results.
func gerritChangesToStr(gChanges []gerrit.Change) []string {
	gChangesStr := make([]string, len(gChanges))
	for i, chg := range gChanges {
		gChangesStr[i] = fmt.Sprintf("ChangeNumber:%d, RevisionNumber:%d, Project:%s", chg.ChangeNumber, chg.RevisionNumber, chg.Project)
	}
	return gChangesStr
}

type relatedTestConfig struct {
	// E.g. `{"change": 1234567, "host": "chromium-review.googlesource.com"}`
	inputJSON string
	// The mocked output for GetRelatedChanges.
	relatedChangesMock map[string]map[int][]gerrit.Change
	// Expected related changes in output.
	expectedRelatedChanges []gerrit.Change
	// Expected related change count in output.
	expectedRelCount int
	// Expected bool for whether there are related changes in output.
	expectedHasRel bool
	// Expected status code from main.
	expectedRetVal int
}

func doTestRun(t *testing.T, tc *relatedTestConfig) {
	t.Helper()

	// Set up test input and output files.
	inputFile, err := os.CreateTemp("", "input_json")
	defer os.Remove(inputFile.Name())
	assert.NilError(t, err)

	outputFile, err := os.CreateTemp("", "output_json")
	defer os.Remove(outputFile.Name())
	assert.NilError(t, err)

	_, err = inputFile.WriteString(tc.inputJSON)
	assert.NilError(t, err)
	assert.NilError(t, inputFile.Close())

	// Mock command runner and gerrit client.
	commandRunners := []cmd.FakeCommandRunner{}

	r := relatedRun{
		cmdRunner: &cmd.FakeCommandRunnerMulti{
			CommandRunners: commandRunners,
		},
		gerritClient: &gerrit.MockClient{
			ExpectedRelatedChanges: tc.relatedChangesMock,
		},
		inputJSON:  inputFile.Name(),
		outputJSON: outputFile.Name(),
	}

	// Do the test run.
	ret := r.Run(nil, nil, nil)
	assert.IntsEqual(t, ret, tc.expectedRetVal)

	// For successful runs, check actual output.
	if tc.expectedRetVal == 0 {
		// Check expected output.
		data, err := os.ReadFile(outputFile.Name())
		assert.NilError(t, err)
		var output RelatedOutput
		assert.NilError(t, json.Unmarshal(data, &output))

		// Format changes for comparison.
		expectedChangesStr := gerritChangesToStr(tc.expectedRelatedChanges)
		actualChangesStr := gerritChangesToStr(output.Related)

		assert.StringArrsEqual(t, expectedChangesStr, actualChangesStr)
		assert.BoolsEqual(t, tc.expectedHasRel, output.HasRelated)
		assert.IntsEqual(t, tc.expectedRelCount, output.RelatedCount)
	}
}

func TestRun_related(t *testing.T) {
	t.Parallel()
	doTestRun(t, &relatedTestConfig{
		inputJSON: `{"change": 1234567,
		"host": "chromium-review.googlesource.com"}`,
		relatedChangesMock: map[string]map[int][]gerrit.Change{
			"chromium-review.googlesource.com": {
				1234567: {{ChangeNumber: 1234565, RevisionNumber: 1, Project: "chromiumos/platform2"}, {ChangeNumber: 1234567, RevisionNumber: 2, Project: "chromiumos/platform2"}},
			},
		},
		expectedRelatedChanges: []gerrit.Change{{ChangeNumber: 1234565, RevisionNumber: 1, Project: "chromiumos/platform2"}, {ChangeNumber: 1234567, RevisionNumber: 2, Project: "chromiumos/platform2"}},
		expectedRelCount:       2,
		expectedHasRel:         true,
		expectedRetVal:         0,
	})
}

func TestRun_norelated(t *testing.T) {
	t.Parallel()
	doTestRun(t, &relatedTestConfig{
		inputJSON: `{"change": 1234567,
		"host": "chromium-review.googlesource.com"}`,
		relatedChangesMock: map[string]map[int][]gerrit.Change{
			"chromium-review.googlesource.com": {
				1234567: {},
			},
		},
		expectedRelatedChanges: []gerrit.Change{},
		expectedRelCount:       0,
		expectedHasRel:         false,
		expectedRetVal:         0,
	})
}

func TestRun_failure(t *testing.T) {
	t.Parallel()
	doTestRun(t, &relatedTestConfig{
		inputJSON: `{"change": 1234567,
		"host": "badhost"}`,
		relatedChangesMock: map[string]map[int][]gerrit.Change{
			"chromium-review.googlesource.com": {
				1234567: {},
			},
		},
		expectedRetVal: 5,
	})
}
