// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	bbpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cros/cmd/try/try"
	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

const (
	validJSON = `{
		"buildbucket": {
			"bbagent_args": {
				"build": {
					"input": {
						"properties": {
							"$chromeos/my_module": {
								"my_prop": 100
							},
							"my_other_prop": 101
						}
					},
					"infra": {
						"buildbucket": {
							"experiment_reasons": {
								"chromeos.cros_artifacts.use_gcloud_storage": 1
							}
						}
					}
				}
			}
		}
	}`
)

func TestValidate(t *testing.T) {
	t.Parallel()
	r := &collectRun{}
	assert.ErrorContains(t, r.validate(), "--input_json")
	r = &collectRun{
		inputJSON: "foo",
	}
	assert.ErrorContains(t, r.validate(), "--output_json")
	r = &collectRun{
		inputJSON:  "foo",
		outputJSON: "bar",
	}
	assert.ErrorContains(t, r.validate(), "BBID")
	r = &collectRun{
		inputJSON:  "foo",
		outputJSON: "bar",
		bbids:      []string{"123"},
	}
	assert.NilError(t, r.validate())
}

type collectResult struct {
	status bbpb.Status
}

type FakeTryClient struct {
	t                   *testing.T
	originalToRetryBBID map[string]string
}

func (c *FakeTryClient) DoRetry(opts *try.RetryRunOpts) (string, error) {
	retryBBID, ok := c.originalToRetryBBID[opts.BBID]
	if !ok {
		return "", fmt.Errorf("unexpected retry for BBID %v", opts.BBID)
	}
	return retryBBID, nil
}

type collectTestConfig struct {
	configJSON          string
	bbids               []int64
	initialRetry        bool
	collectResults      []map[int64]collectResult
	originalToRetryBBID map[string]string
	expectedBBIDS       []int64
	expectedReturnCode  int
}

func doTestRun(t *testing.T, tc *collectTestConfig) {
	t.Helper()

	inputFile, err := os.CreateTemp("", "input_json")
	defer os.Remove(inputFile.Name())
	assert.NilError(t, err)

	outputFile, err := os.CreateTemp("", "output_json")
	defer os.Remove(outputFile.Name())
	assert.NilError(t, err)

	_, err = inputFile.WriteString(tc.configJSON)
	assert.NilError(t, err)
	assert.NilError(t, inputFile.Close())

	var initialBBIDs []string
	commandRunners := []cmd.FakeCommandRunner{
		bb.FakeWhichRunner("bb", 0),
		bb.FakeAuthInfoRunner("bb", 0),
		bb.FakeWhichRunner("led", 0),
		bb.FakeAuthInfoRunner("led", 0),
	}
	for _, collectResults := range tc.collectResults {
		var stdout string
		bbids := []string{}
		for bbid, collectResult := range collectResults {
			// CreateTime doesn't marshal correctly, construct JSON manually.
			buildJSON := fmt.Sprintf(`{"id":%d,"createTime":"%s", "status":%d}`,
				bbid,
				"2023-04-10T04:00:03.884668293Z",
				collectResult.status)
			bbids = append(bbids, fmt.Sprintf("%d", bbid))
			stdout += string(buildJSON) + "\n"
		}
		sort.Strings(bbids)
		if len(initialBBIDs) == 0 {
			initialBBIDs = bbids
		}
		args := []string{"bb", "get"}
		args = append(args, bbids...)
		args = append(args, "-p", "-json")
		commandRunners = append(commandRunners, cmd.FakeCommandRunner{
			ExpectedCmd: args,
			Stdout:      stdout,
		})
	}
	c := collectRun{
		tryClient: &FakeTryClient{
			t:                   t,
			originalToRetryBBID: tc.originalToRetryBBID,
		},
		cmdRunner: &cmd.FakeCommandRunnerMulti{
			CommandRunners: commandRunners,
		},
		inputJSON:              inputFile.Name(),
		outputJSON:             outputFile.Name(),
		pollingIntervalSeconds: 0,
		bbids:                  initialBBIDs,
		initialRetry:           tc.initialRetry,
	}
	ret := c.Run(nil, nil, nil)
	assert.IntsEqual(t, ret, tc.expectedReturnCode)

	data, err := os.ReadFile(outputFile.Name())
	assert.NilError(t, err)

	var output CollectOutput
	assert.NilError(t, json.Unmarshal(data, &output))
	strBBIDs := make([]string, len(tc.expectedBBIDS))
	for i, bbid := range tc.expectedBBIDS {
		strBBIDs[i] = fmt.Sprintf("%d", bbid)
	}
	assert.StringArrsEqual(t, output.BBIDs, strBBIDs)
}

func TestCollect_NoRetries(t *testing.T) {
	t.Parallel()
	doTestRun(t, &collectTestConfig{
		configJSON:    "{}",
		bbids:         []int64{12345, 12346, 12347},
		expectedBBIDS: []int64{12345, 12346, 12347},
		collectResults: []map[int64]collectResult{
			{
				12345: {
					bbpb.Status_SCHEDULED,
				},
				12346: {
					bbpb.Status_STARTED,
				},
				12347: {
					bbpb.Status_STARTED,
				},
			},
			{
				12345: {
					bbpb.Status_SUCCESS,
				},
				12346: {
					bbpb.Status_STARTED,
				},
				12347: {
					bbpb.Status_STARTED,
				},
			},
			{
				12346: {
					bbpb.Status_FAILURE,
				},
				12347: {
					bbpb.Status_INFRA_FAILURE,
				},
			},
		},
	})
}

var (
	basicRetryConfig = `{
		"rules": [
			{
				"max_retries": 3
			}
		]
	}`
)

func TestCollect_RetryFailure(t *testing.T) {
	t.Parallel()
	doTestRun(t, &collectTestConfig{
		configJSON: basicRetryConfig,
		bbids:      []int64{12345, 12346},
		// Retry failed so we expect the original set.
		expectedBBIDS:       []int64{12345, 12346},
		expectedReturnCode:  4,
		originalToRetryBBID: map[string]string{
			// Force an error by omitting "12346".
		},
		collectResults: []map[int64]collectResult{
			{
				12345: {
					bbpb.Status_SCHEDULED,
				},
				12346: {
					bbpb.Status_STARTED,
				},
			},
			{
				12345: {
					bbpb.Status_SUCCESS,
				},
				12346: {
					bbpb.Status_STARTED,
				},
			},
			{
				12346: {
					bbpb.Status_FAILURE,
				},
			},
		},
	})
}

func TestCollect_Retry(t *testing.T) {
	t.Parallel()
	doTestRun(t, &collectTestConfig{
		configJSON:    basicRetryConfig,
		bbids:         []int64{12345, 12346},
		expectedBBIDS: []int64{12345, 12347},
		originalToRetryBBID: map[string]string{
			"12346": "12347",
		},
		collectResults: []map[int64]collectResult{
			{
				12345: {
					bbpb.Status_SCHEDULED,
				},
				12346: {
					bbpb.Status_STARTED,
				},
			},
			{
				12345: {
					bbpb.Status_SUCCESS,
				},
				12346: {
					bbpb.Status_STARTED,
				},
			},
			{
				12346: {
					bbpb.Status_FAILURE,
				},
			},
			{
				12347: {
					bbpb.Status_SUCCESS,
				},
			},
		},
	})
}

// No retries but initialRetry is true so we should get one retry for free.
func TestCollect_NoRetries_InitialRetry(t *testing.T) {
	t.Parallel()
	doTestRun(t, &collectTestConfig{
		configJSON:    "{}",
		bbids:         []int64{12345, 12346},
		expectedBBIDS: []int64{12345, 12347},
		initialRetry:  true,
		originalToRetryBBID: map[string]string{
			"12346": "12347",
		},
		collectResults: []map[int64]collectResult{
			{
				12345: {
					bbpb.Status_SUCCESS,
				},
				12346: {
					bbpb.Status_FAILURE,
				},
			},
			{
				12347: {
					bbpb.Status_STARTED,
				},
			},
			{
				12347: {
					bbpb.Status_INFRA_FAILURE,
				},
			},
		},
	})
}
