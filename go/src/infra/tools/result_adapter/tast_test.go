// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	. "go.chromium.org/luci/common/testing/assertions"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.99Z", s)
	return t
}

func mockCollect(s string) (map[string]string, error) {
	return map[string]string{
		"foo": s + "/foo",
	}, nil
}

func genJSONLine(m map[string]string) string {
	base := map[string]string{
		"name":         "lacros.Basic",
		"contacts":     `["user1@google.com", "user2@google.com"]`,
		"bugComponent": "b:1234",
		"bundle":       "cros",
		"start":        "2021-07-26T18:53:33.983328614Z",
		"end":          "2021-07-26T18:53:34.983328614Z",
		"outDir":       "/usr/local/autotest/results/lxc_job_folder/tast/results/tests/lacros.Basic",
	}
	for k, v := range m {
		base[k] = v
	}
	jsonStr := ""
	for k, v := range base {
		if k == "errors" || k == "searchFlags" || k == "contacts" {
			jsonStr += fmt.Sprintf("\"%s\":%s,", k, v)
			continue
		}
		jsonStr += fmt.Sprintf("\"%s\":\"%s\",", k, v)
	}
	return fmt.Sprintf("{%s}", strings.TrimSuffix(jsonStr, ","))
}

func TestTastConversions(t *testing.T) {
	t.Parallel()

	Convey(`From JSON works`, t, func() {
		r := &TastResults{}
		Convey(`Basic`, func() {
			jsonLine := genJSONLine(map[string]string{
				"skipReason":  "skipped",
				"searchFlags": `[{"key":"testKey", "value":"testValue"}]`,
			})
			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			So(r.Cases[0], ShouldResemble, TastCase{
				Name:         "lacros.Basic",
				Contacts:     []string{"user1@google.com", "user2@google.com"},
				BugComponent: "b:1234",
				OutDir:       "/usr/local/autotest/results/lxc_job_folder/tast/results/tests/lacros.Basic",
				SkipReason:   "skipped",
				Errors:       nil,
				Start:        parseTime("2021-07-26T18:53:33.983328614Z"),
				End:          parseTime("2021-07-26T18:53:34.983328614Z"),
				SearchFlags:  []*pb.StringPair{{Key: "testKey", Value: "testValue"}},
			})
		})
		Convey(`Errors`, func() {
			jsonLine := genJSONLine(map[string]string{
				"errors": `[{ "time": "2021-07-26T18:54:38.153491776Z", "file": "dummy.go", "reason": "Failed due to dummy error", "stack": "Dummy Failure" }]`,
			})
			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			So(r.Cases[0].Errors[0], ShouldResemble, TastError{
				parseTime("2021-07-26T18:54:38.153491776Z"),
				"Failed due to dummy error",
				"dummy.go",
				"Dummy Failure",
			})
		})
	})

	Convey(`ToProtos works`, t, func() {
		ctx := context.Background()
		Convey(`Basic`, func() {
			testhausBaseUrl := "https://tests.chromeos.goog/p/chromeos/logs/unified/build-12345"
			jsonLine := genJSONLine(map[string]string{
				"searchFlags": `[{"key":"testKey", "value":"testValue"}]`,
			})
			r := &TastResults{
				BaseDir: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test",
			}
			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			got, err := r.ToProtos(ctx, "", mockCollect, testhausBaseUrl)
			So(err, ShouldBeNil)
			So(got[0], ShouldResembleProto, &sinkpb.TestResult{
				TestId:   "tast.lacros.Basic",
				Expected: true,
				Status:   pb.TestStatus_PASS,
				Artifacts: map[string]*sinkpb.Artifact{
					"foo": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test/tast/results/tests/lacros.Basic/foo"},
					},
					"testhaus_logs": {
						Body:        &sinkpb.Artifact_Contents{Contents: []byte(fmt.Sprintf("%s?treeQuery=lacros.Basic&test=tast.lacros.Basic", testhausBaseUrl))},
						ContentType: "text/x-uri",
					},
				},
				Tags: []*pb.StringPair{
					pbutil.StringPair("contacts", "user1@google.com,user2@google.com"),
					pbutil.StringPair("testKey", "testValue"),
					pbutil.StringPair(executionOrderTag, "1"),
					pbutil.StringPair("bug_component", "b:1234"),
				},
				TestMetadata: &pb.TestMetadata{
					Name: "tast.lacros.Basic",
					BugComponent: &pb.BugComponent{
						System: &pb.BugComponent_IssueTracker{
							IssueTracker: &pb.IssueTrackerComponent{
								ComponentId: 1234,
							},
						},
					},
				},
				StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
				Duration:  &duration.Duration{Seconds: 1},
			})
		})
		Convey(`With metadata`, func() {
			r := &TastResults{
				BaseDir: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test",
			}
			err := r.ConvertFromJSON(strings.NewReader(genJSONLine(nil) + "\n" + genJSONLine(map[string]string{
				"name":   "lacros.Migrate",
				"outDir": "/usr/local/autotest/results/lxc_job_folder/tast/results/tests/lacros.Migrate",
			})))
			So(err, ShouldBeNil)
			got, err := r.ToProtos(ctx, "./test_data/tast/test_metadata.json", mockCollect, "")
			So(err, ShouldBeNil)
			expected := []*sinkpb.TestResult{
				{
					TestId:   "tast.lacros.Basic",
					Expected: true,
					Status:   pb.TestStatus_PASS,
					Artifacts: map[string]*sinkpb.Artifact{
						"foo": {
							Body: &sinkpb.Artifact_FilePath{FilePath: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test/tast/results/tests/lacros.Basic/foo"},
						},
					},
					Tags: []*pb.StringPair{
						pbutil.StringPair("contacts", "user1@google.com,user2@google.com"),
						pbutil.StringPair(executionOrderTag, "1"),
						pbutil.StringPair("owners", "owner1@test.com,owner2@test.com"),
						pbutil.StringPair("bug_component", "b:1234"),
						pbutil.StringPair("life_cycle_stage", "LIFE_CYCLE_OWNER_MONITORED"),
						pbutil.StringPair("tags", "group:mainline,wificell-qa"),
						pbutil.StringPair("test_harness", "Tast"),
					},
					TestMetadata: &pb.TestMetadata{
						Name: "tast.lacros.Basic",
						BugComponent: &pb.BugComponent{
							System: &pb.BugComponent_IssueTracker{
								IssueTracker: &pb.IssueTrackerComponent{
									ComponentId: 1234,
								},
							},
						},
						PropertiesSchema: metadataSchema,
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							"contacts":         {Kind: &structpb.Value_StringValue{StringValue: "user1@google.com,user2@google.com"}},
							executionOrderTag:  {Kind: &structpb.Value_StringValue{StringValue: "1"}},
							"owners":           {Kind: &structpb.Value_StringValue{StringValue: "owner1@test.com,owner2@test.com"}},
							"bug_component":    {Kind: &structpb.Value_StringValue{StringValue: "b:1234"}},
							"life_cycle_stage": {Kind: &structpb.Value_StringValue{StringValue: "LIFE_CYCLE_OWNER_MONITORED"}},
							"tags":             {Kind: &structpb.Value_StringValue{StringValue: "group:mainline,wificell-qa"}},
							"test_harness":     {Kind: &structpb.Value_StringValue{StringValue: "Tast"}},
						}},
					},
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 1},
				},
				{
					TestId:   "tast.lacros.Migrate",
					Expected: true,
					Status:   pb.TestStatus_PASS,
					Artifacts: map[string]*sinkpb.Artifact{
						"foo": {
							Body: &sinkpb.Artifact_FilePath{FilePath: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test/tast/results/tests/lacros.Migrate/foo"},
						},
					},
					Tags: []*pb.StringPair{
						pbutil.StringPair("contacts", "user1@google.com,user2@google.com"),
						pbutil.StringPair(executionOrderTag, "2"),
						pbutil.StringPair("requirements", "requirement 1 in a very long list of requirements,requirement 2 in a very long list of requirements,requirement 3 in a very long list of requirements,requirement 4 in a very long list of requirements,requirement 5 in a very long list of requirements,req..."),
						pbutil.StringPair("bug_component", "crbug:OS>LaCrOS"),
						pbutil.StringPair("criteria", "A very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo..."),
						pbutil.StringPair("test_harness", "Tast"),
					},
					TestMetadata: &pb.TestMetadata{
						Name: "tast.lacros.Migrate",
						BugComponent: &pb.BugComponent{
							System: &pb.BugComponent_Monorail{
								Monorail: &pb.MonorailComponent{
									Project: "chromium",
									Value:   "OS>LaCrOS",
								},
							},
						},
						PropertiesSchema: metadataSchema,
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							"contacts":        {Kind: &structpb.Value_StringValue{StringValue: "user1@google.com,user2@google.com"}},
							executionOrderTag: {Kind: &structpb.Value_StringValue{StringValue: "2"}},
							"criteria":        {Kind: &structpb.Value_StringValue{StringValue: "A very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo..."}},
							"bug_component":   {Kind: &structpb.Value_StringValue{StringValue: "crbug:OS>LaCrOS"}},
							"requirements":    {Kind: &structpb.Value_StringValue{StringValue: "requirement 1 in a very long list of requirements,requirement 2 in a very long list of requirements,requirement 3 in a very long list of requirements,requirement 4 in a very long list of requirements,requirement 5 in a very long list of requirements,req..."}},
							"test_harness":    {Kind: &structpb.Value_StringValue{StringValue: "Tast"}},
						}},
					},
					StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 1},
				},
			}
			So(got, ShouldHaveLength, 2)
			So(got, ShouldResemble, expected)
		})
		Convey(`Skipped`, func() {
			jsonLine := genJSONLine(map[string]string{
				"skipReason": "skipped",
				"outDir":     "",
			})
			r := &TastResults{
				BaseDir: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test",
			}
			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			got, err := r.ToProtos(ctx, "", mockCollect, "")
			So(err, ShouldBeNil)
			So(got[0], ShouldResembleProto, &sinkpb.TestResult{
				TestId:      "tast.lacros.Basic",
				Expected:    true,
				Status:      pb.TestStatus_SKIP,
				SummaryHtml: "<text-artifact artifact-id=\"Skip Reason\" />",
				Artifacts: map[string]*sinkpb.Artifact{
					"Skip Reason": {
						Body: &sinkpb.Artifact_Contents{
							Contents: []byte("skipped"),
						},
						ContentType: "text/plain",
					},
				},
				Tags: []*pb.StringPair{
					pbutil.StringPair("contacts", "user1@google.com,user2@google.com"),
					pbutil.StringPair(executionOrderTag, "1"),
					pbutil.StringPair("bug_component", "b:1234"),
				},
				TestMetadata: &pb.TestMetadata{
					Name: "tast.lacros.Basic",
					BugComponent: &pb.BugComponent{
						System: &pb.BugComponent_IssueTracker{
							IssueTracker: &pb.IssueTrackerComponent{
								ComponentId: 1234,
							},
						},
					},
				},
				FailureReason: &pb.FailureReason{
					PrimaryErrorMessage: "skipped",
					Errors: []*pb.FailureReason_Error{
						{Message: "skipped"},
					},
				},
				StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
				Duration:  &duration.Duration{Seconds: 1},
			})
		})
		Convey(`Unexpectedly Skipped`, func() {
			jsonLine := genJSONLine(map[string]string{
				"skipReason": "",
				"errors":     `[{ "time": "2021-07-26T18:54:38.153491776Z", "file": "dummy.go", "reason": "Test did not run", "stack": "Dummy Failure" }]`,
			})
			r := &TastResults{
				BaseDir: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test",
			}
			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			got, err := r.ToProtos(ctx, "", mockCollect, "")
			So(err, ShouldBeNil)
			So(got[0], ShouldResembleProto, &sinkpb.TestResult{
				TestId:      "tast.lacros.Basic",
				Expected:    false,
				Status:      pb.TestStatus_SKIP,
				SummaryHtml: "<text-artifact artifact-id=\"Test Log\" />",
				Artifacts: map[string]*sinkpb.Artifact{
					"foo": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test/tast/results/tests/lacros.Basic/foo"},
					},
					"Test Log": {
						Body: &sinkpb.Artifact_Contents{
							Contents: []byte("Dummy Failure"),
						},
						ContentType: "text/plain",
					},
				},
				Tags: []*pb.StringPair{
					pbutil.StringPair("contacts", "user1@google.com,user2@google.com"),
					pbutil.StringPair(executionOrderTag, "1"),
					pbutil.StringPair("bug_component", "b:1234"),
				},
				TestMetadata: &pb.TestMetadata{
					Name: "tast.lacros.Basic",
					BugComponent: &pb.BugComponent{
						System: &pb.BugComponent_IssueTracker{
							IssueTracker: &pb.IssueTrackerComponent{
								ComponentId: 1234,
							},
						},
					},
				},
				FailureReason: &pb.FailureReason{
					PrimaryErrorMessage: TestDidNotRunErr,
					Errors: []*pb.FailureReason_Error{
						{Message: TestDidNotRunErr},
					},
				},
				StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
				Duration:  &duration.Duration{Seconds: 1},
			})
		})
		Convey(`Errors`, func() {
			jsonLine := genJSONLine(map[string]string{
				"errors": `[
					{ "time": "2021-07-26T18:54:38.153491776Z", "file": "dummy.go", "reason": "Failed due to dummy error", "stack": "Dummy Failure" },
					{ "time": "2021-07-26T18:55:48.153491787Z", "file": "dummy.go", "reason": "Failed due to dummy error (2)", "stack": "Dummy Failure (2)" }
				]`,
			})
			r := &TastResults{
				BaseDir: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test",
			}
			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			got, err := r.ToProtos(ctx, "", mockCollect, "")
			So(err, ShouldBeNil)
			So(got[0].Duration, ShouldResemble, &duration.Duration{Seconds: 1})
			So(got[0], ShouldResembleProto, &sinkpb.TestResult{
				TestId:      "tast.lacros.Basic",
				Expected:    false,
				Status:      pb.TestStatus_FAIL,
				SummaryHtml: "<text-artifact artifact-id=\"Test Log\" />",
				Artifacts: map[string]*sinkpb.Artifact{
					"foo": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test/tast/results/tests/lacros.Basic/foo"},
					},
					"Test Log": {
						Body: &sinkpb.Artifact_Contents{
							Contents: []byte("Dummy Failure\nDummy Failure (2)"),
						},
						ContentType: "text/plain",
					},
				},
				Tags: []*pb.StringPair{
					pbutil.StringPair("contacts", "user1@google.com,user2@google.com"),
					pbutil.StringPair(executionOrderTag, "1"),
					pbutil.StringPair("bug_component", "b:1234"),
				},
				TestMetadata: &pb.TestMetadata{
					Name: "tast.lacros.Basic",
					BugComponent: &pb.BugComponent{
						System: &pb.BugComponent_IssueTracker{
							IssueTracker: &pb.IssueTrackerComponent{
								ComponentId: 1234,
							},
						},
					},
				},
				FailureReason: &pb.FailureReason{
					PrimaryErrorMessage: "Failed due to dummy error",
					Errors: []*pb.FailureReason_Error{
						{Message: "Failed due to dummy error"},
						{Message: "Failed due to dummy error (2)"},
					},
				},
				StartTime: timestamppb.New(parseTime("2021-07-26T18:53:33.983328614Z")),
				Duration:  &duration.Duration{Seconds: 1},
			})
		})
		Convey(`Truncate errors for failed tests`, func() {
			maxErrorMessage := strings.Repeat(".", 1024)
			jsonLine := genJSONLine(map[string]string{
				"errors": fmt.Sprintf(`[
					{ "time": "2021-07-26T18:54:38.153491776Z", "file": "dummy.go", "reason": "%s", "stack": "Dummy Failure" },
					{ "time": "2021-07-26T18:55:48.153491787Z", "file": "dummy.go", "reason": "%s", "stack": "Dummy Failure (2)" },
					{ "time": "2021-07-26T18:55:48.153491787Z", "file": "dummy.go", "reason": "%s", "stack": "Dummy Failure (3)" },
					{ "time": "2021-07-26T18:55:48.153491787Z", "file": "dummy.go", "reason": "%s", "stack": "Dummy Failure (4)" }
				]`, maxErrorMessage, maxErrorMessage, maxErrorMessage,
					maxErrorMessage),
			})
			r := &TastResults{
				BaseDir: "/usr/local/autotest/results/swarming-55970dfb3e7ef210/1/autoserv_test",
			}

			err := r.ConvertFromJSON(strings.NewReader(jsonLine))
			So(err, ShouldBeNil)
			got, err := r.ToProtos(ctx, "", mockCollect, "")

			// Only 3 errors are stored while 1 error is truncated.
			So(err, ShouldBeNil)
			So(got[0].FailureReason, ShouldResemble, &pb.FailureReason{
				PrimaryErrorMessage: maxErrorMessage,
				Errors: []*pb.FailureReason_Error{
					{Message: maxErrorMessage},
					{Message: maxErrorMessage},
					{Message: maxErrorMessage},
				},
				TruncatedErrorsCount: 1,
			})
		})
	})
}
