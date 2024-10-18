// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func TestSkylabTestRunnerConversions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tc := []TestRunnerTestCase{
		{
			Name:      "test1",
			Verdict:   "VERDICT_PASS",
			StartTime: parseTime("2021-07-26T18:53:33.983328614Z"),
			EndTime:   parseTime("2021-07-26T18:53:37.983328614Z"),
		},
		// No EndTime
		{
			Name:      "test2",
			Verdict:   "VERDICT_NO_VERDICT",
			StartTime: parseTime("2021-07-26T18:53:33.983328614Z"),
		},
		// No StartTime and EndTime
		{
			Name:                 "test3",
			Verdict:              "VERDICT_FAIL",
			HumanReadableSummary: "test failure",
		},
		{
			Name:                 "test4",
			Verdict:              "VERDICT_ERROR",
			HumanReadableSummary: "test error",
			StartTime:            parseTime("2021-07-26T18:53:33.983328614Z"),
			EndTime:              parseTime("2021-07-26T18:53:37.983328614Z"),
		},
		{
			Name:                 "test5",
			Verdict:              "VERDICT_ABORT",
			HumanReadableSummary: "test abort",
			StartTime:            parseTime("2021-07-26T18:53:33.983328614Z"),
			EndTime:              parseTime("2021-07-26T18:53:37.983328614Z"),
		},
	}

	results := TestRunnerResult{Autotest: TestRunnerAutotest{
		TestCases: tc,
	}}

	ftt.Run(`From JSON works`, t, func(t *ftt.Test) {
		str := `{
			"autotest_result": {
				"test_cases": [
				{
					"verdict": "VERDICT_PASS",
					"name": "test1",
					"start_time": "2021-07-26T18:53:33.983328614Z",
					"end_time": "2021-07-26T18:53:37.983328614Z"
				},
				{
					"verdict": "VERDICT_NO_VERDICT",
					"name": "test2",
					"start_time": "2021-07-26T18:53:33.983328614Z"
				},
				{
					"verdict": "VERDICT_FAIL",
					"name": "test3",
					"human_readable_summary": "test failure"
				},
				{
					"verdict": "VERDICT_ERROR",
					"name": "test4",
					"human_readable_summary": "test error",
					"start_time": "2021-07-26T18:53:33.983328614Z",
					"end_time": "2021-07-26T18:53:37.983328614Z"
				},
				{
					"verdict": "VERDICT_ABORT",
					"name": "test5",
					"human_readable_summary": "test abort",
					"start_time": "2021-07-26T18:53:33.983328614Z",
					"end_time": "2021-07-26T18:53:37.983328614Z"
				}
				]
			}
		}`

		results := &TestRunnerResult{}
		err := results.ConvertFromJSON(strings.NewReader(str))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, results.Autotest.TestCases, should.Match(tc))
	})

	ftt.Run(`ToProtos`, t, func(t *ftt.Test) {
		artBaseDir := filepath.Join("test_data", "cros_test_result", "artifacts")

		t.Run("test passes", func(t *ftt.Test) {

			testResults, err := results.ToProtos(ctx, "", artBaseDir)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:    "test1",
					Expected:  true,
					Status:    pb.TestStatus_PASS,
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 4},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "1"),
					},
					TestMetadata: &pb.TestMetadata{},
				},
				{
					TestId:    "test2",
					Expected:  true,
					Status:    pb.TestStatus_SKIP,
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "2"),
					},
					TestMetadata: &pb.TestMetadata{},
				},
				{
					TestId:      "test3",
					Expected:    false,
					Status:      pb.TestStatus_FAIL,
					SummaryHtml: "<pre>test failure</pre>",
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "test failure",
						Errors: []*pb.FailureReason_Error{
							{Message: "test failure"},
						},
					},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "3"),
					},
					TestMetadata: &pb.TestMetadata{},
				},
				{
					TestId:      "test4",
					Expected:    false,
					Status:      pb.TestStatus_CRASH,
					SummaryHtml: "<pre>test error</pre>",
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "test error",
						Errors: []*pb.FailureReason_Error{
							{Message: "test error"},
						},
					},
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 4},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "4"),
					},
					TestMetadata: &pb.TestMetadata{},
				},
				{
					TestId:      "test5",
					Expected:    false,
					Status:      pb.TestStatus_ABORT,
					SummaryHtml: "<pre>test abort</pre>",
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "test abort",
						Errors: []*pb.FailureReason_Error{
							{Message: "test abort"},
						},
					},
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 4},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "5"),
					},
					TestMetadata: &pb.TestMetadata{},
				},
			}
			assert.Loosely(t, testResults, should.HaveLength(5))
			assert.Loosely(t, testResults, should.Match(expected))
		})

		t.Run("test passes: CFT test run with CFT metadata", func(t *ftt.Test) {
			testCases := make([]TestRunnerTestCase, len(tc))
			copy(testCases, tc)

			// The test_metadata.json file is only available for CFT runs. In
			// these runs the test name is prefixed with 'tauto.'.
			for i := range testCases {
				testCases[i].Name = "tauto." + testCases[i].Name
			}
			results := TestRunnerResult{Autotest: TestRunnerAutotest{
				TestCases: testCases,
			}}
			testResults, err := results.ToProtos(ctx, "./test_data/skylab_test_runner/test_metadata.json", "")
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:    "tauto.test1",
					Expected:  true,
					Status:    pb.TestStatus_PASS,
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 4},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "1"),
						pbutil.StringPair("owners", "owner1@test.com"),
						pbutil.StringPair("requirements", "requirement 1"),
						pbutil.StringPair("bug_component", "b:1234"),
						pbutil.StringPair("criteria", "criteria 1"),
						pbutil.StringPair("hw_agnostic", "true"),
						pbutil.StringPair("life_cycle_stage", "LIFE_CYCLE_OWNER_MONITORED"),
						pbutil.StringPair("tags", "group:mainline,wificell-qa"),
						pbutil.StringPair("test_harness", "Tauto"),
					},
					TestMetadata: &pb.TestMetadata{
						BugComponent: &pb.BugComponent{
							System: &pb.BugComponent_IssueTracker{
								IssueTracker: &pb.IssueTrackerComponent{
									ComponentId: 1234,
								},
							},
						},
						PropertiesSchema: metadataSchema,
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							executionOrderTag:  {Kind: &structpb.Value_StringValue{StringValue: "1"}},
							"owners":           {Kind: &structpb.Value_StringValue{StringValue: "owner1@test.com"}},
							"requirements":     {Kind: &structpb.Value_StringValue{StringValue: "requirement 1"}},
							"bug_component":    {Kind: &structpb.Value_StringValue{StringValue: "b:1234"}},
							"criteria":         {Kind: &structpb.Value_StringValue{StringValue: "criteria 1"}},
							"hw_agnostic":      {Kind: &structpb.Value_StringValue{StringValue: "true"}},
							"life_cycle_stage": {Kind: &structpb.Value_StringValue{StringValue: "LIFE_CYCLE_OWNER_MONITORED"}},
							"tags":             {Kind: &structpb.Value_StringValue{StringValue: "group:mainline,wificell-qa"}},
							"test_harness":     {Kind: &structpb.Value_StringValue{StringValue: "Tauto"}},
						}},
					},
				},
				{
					TestId:    "tauto.test2",
					Expected:  true,
					Status:    pb.TestStatus_SKIP,
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "2"),
						pbutil.StringPair("owners", "owner1@test.com,owner2@test.com"),
						pbutil.StringPair("test_harness", "Tauto"),
					},
					TestMetadata: &pb.TestMetadata{
						PropertiesSchema: metadataSchema,
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							executionOrderTag: {Kind: &structpb.Value_StringValue{StringValue: "2"}},
							"owners":          {Kind: &structpb.Value_StringValue{StringValue: "owner1@test.com,owner2@test.com"}},
							"test_harness":    {Kind: &structpb.Value_StringValue{StringValue: "Tauto"}},
						}},
					},
				},
				{
					TestId:      "tauto.test3",
					Expected:    false,
					Status:      pb.TestStatus_FAIL,
					SummaryHtml: "<pre>test failure</pre>",
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "test failure",
						Errors: []*pb.FailureReason_Error{
							{Message: "test failure"},
						},
					},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "3"),
						pbutil.StringPair("requirements", "requirement a,requirement b"),
						pbutil.StringPair("test_harness", "Tauto"),
					},
					TestMetadata: &pb.TestMetadata{
						PropertiesSchema: metadataSchema,
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							executionOrderTag: {Kind: &structpb.Value_StringValue{StringValue: "3"}},
							"requirements":    {Kind: &structpb.Value_StringValue{StringValue: "requirement a,requirement b"}},
							"test_harness":    {Kind: &structpb.Value_StringValue{StringValue: "Tauto"}},
						}}},
				},
				{
					TestId:      "tauto.test4",
					Expected:    false,
					Status:      pb.TestStatus_CRASH,
					SummaryHtml: "<pre>test error</pre>",
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "test error",
						Errors: []*pb.FailureReason_Error{
							{Message: "test error"},
						},
					},
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 4},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "4"),
						pbutil.StringPair("bug_component", "crbug:Blink>JavaScript>WebAssembly"),
						pbutil.StringPair("test_harness", "Tauto"),
					},
					TestMetadata: &pb.TestMetadata{
						BugComponent: &pb.BugComponent{
							System: &pb.BugComponent_Monorail{
								Monorail: &pb.MonorailComponent{
									Project: "chromium",
									Value:   "Blink>JavaScript>WebAssembly",
								},
							},
						},
						PropertiesSchema: metadataSchema,
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							executionOrderTag: {Kind: &structpb.Value_StringValue{StringValue: "4"}},
							"bug_component":   {Kind: &structpb.Value_StringValue{StringValue: "crbug:Blink>JavaScript>WebAssembly"}},
							"test_harness":    {Kind: &structpb.Value_StringValue{StringValue: "Tauto"}},
						}},
					},
				},
				{
					TestId:      "tauto.test5",
					Expected:    false,
					Status:      pb.TestStatus_ABORT,
					SummaryHtml: "<pre>test abort</pre>",
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "test abort",
						Errors: []*pb.FailureReason_Error{
							{Message: "test abort"},
						},
					},
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 4},
					Tags: []*pb.StringPair{
						pbutil.StringPair(executionOrderTag, "5"),
					},
					TestMetadata: &pb.TestMetadata{},
				},
			}
			assert.Loosely(t, testResults, should.HaveLength(5))
			assert.Loosely(t, testResults, should.Match(expected))
		})

		t.Run(`check the oversize failure reason`, func(t *ftt.Test) {
			// Creates an oversize random failure reason.
			letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
			failureReason := make([]byte, maxSummaryHtmlBytes+100)
			for i := range failureReason {
				failureReason[i] = letterBytes[rand.Intn(len(letterBytes))]
			}
			str := `{
				"autotest_result": {
					"test_cases": [
					{
						"verdict": "VERDICT_FAIL",
						"name": "test3",
						"human_readable_summary": "%s"
					}
					]
				}
			}`

			resultString := fmt.Sprintf(str, failureReason)
			results := &TestRunnerResult{}
			results.ConvertFromJSON(strings.NewReader(resultString))
			testResults, err := results.ToProtos(ctx, "", "")

			// Checks if the test result conversion succeeded and size limitation was set properly.
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, len(testResults[0].SummaryHtml), should.BeLessThanOrEqual(maxSummaryHtmlBytes))
			assert.Loosely(t, len(testResults[0].FailureReason.PrimaryErrorMessage), should.BeLessThanOrEqual(maxErrorMessageBytes))
		})

		t.Run(`check an expected skip test`, func(t *ftt.Test) {
			str := `{
				"autotest_result": {
					"test_cases": [
					{
						"verdict": "VERDICT_NO_VERDICT",
						"name": "test3",
						"human_readable_summary": "Test failed"
					}
					]
				}
			}`

			results := &TestRunnerResult{}
			results.ConvertFromJSON(strings.NewReader(str))
			testResults, err := results.ToProtos(ctx, "", "")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, testResults[0].Status, should.Match(pb.TestStatus_SKIP))
			assert.Loosely(t, testResults[0].Expected, should.Match(true))
		})

		t.Run("When running one test case should upload all artifacts under that test", func(t *ftt.Test) {
			artName1 := "test_artifact_1.txt"
			artName2 := "test_artifact_2.txt"
			wantArtifacts := map[string]*sinkpb.Artifact{
				artName1: {
					Body: &sinkpb.Artifact_FilePath{FilePath: filepath.Join(artBaseDir, artName1)},
				},
				artName2: {
					Body: &sinkpb.Artifact_FilePath{FilePath: filepath.Join(artBaseDir, artName2)},
				},
			}
			tc := []TestRunnerTestCase{
				{
					Name:      "cros_test_result",
					Verdict:   "VERDICT_PASS",
					StartTime: parseTime("2021-07-26T18:53:33.983328614Z"),
					EndTime:   parseTime("2021-07-26T18:53:37.983328614Z"),
				},
			}
			results := TestRunnerResult{Autotest: TestRunnerAutotest{
				TestCases: tc,
			}}

			gotTestResults, err := results.ToProtos(ctx, "", artBaseDir)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotTestResults, should.HaveLength(1))
			assert.Loosely(t, gotTestResults[0].GetArtifacts(), should.Match(wantArtifacts))
		})

		t.Run("Skips test artifacts upload when result dir is invalid", func(t *ftt.Test) {
			tc := []TestRunnerTestCase{
				{
					Name:      "test1",
					Verdict:   "VERDICT_PASS",
					StartTime: parseTime("2021-07-26T18:53:33.983328614Z"),
					EndTime:   parseTime("2021-07-26T18:53:37.983328614Z"),
				},
			}
			results := TestRunnerResult{Autotest: TestRunnerAutotest{
				TestCases: tc,
			}}

			gotTestResults, err := results.ToProtos(ctx, "", "invalid_dir")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotTestResults, should.HaveLength(1))
			assert.Loosely(t, gotTestResults[0].GetArtifacts(), should.BeEmpty)
		})
	})
}
