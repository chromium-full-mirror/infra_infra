// Copyright 2020 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

const snippetInSummaryHtml = `<p><text-artifact artifact-id="snippet" /></p>`

func TestGTestConversions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	textArt := func(s string) *sinkpb.Artifact {
		a := &sinkpb.Artifact{
			Body:        &sinkpb.Artifact_Contents{Contents: []byte(s)},
			ContentType: "text/plain",
		}
		return a
	}

	ftt.Run(`From JSON works`, t, func(t *ftt.Test) {
		str := `{
				"all_tests": [
					"FooTest.TestDoBar",
					"FooTest.TestDoBaz"
				],
				"disabled_tests": [
					"FooTest.TestDoBarDisabled"
				],
				"global_tags": ["CPU_64_BITS","MODE_RELEASE","OS_WIN"],
				"per_iteration_data": [{
					"FooTest.TestDoBar": [
						{
							"elapsed_time_ms": 1837,
							"losless_snippet": true,
							"output_snippet": "[ RUN      ] FooTest.TestDoBar",
							"output_snippet_base64": "WyBSVU4gICAgICBdIEZvb1Rlc3QuVGVzdERvQmFy",
							"status": "CRASH"
						},
						{
							"elapsed_time_ms": 1856,
							"losless_snippet": false,
							"output_snippet_base64": "c29tZSBkYXRhIHdpdGggACBhbmQg77u/",
							"status": "FAILURE",
							"result_parts":[{
								"summary_base64":"VGhpcyBpcyBhIGZhaWx1cmUgbWVzc2FnZS4=",
								"type":"failure"
							}, {
								"summary_base64":"VGhpcyBpcyBhIGZhdGFsIGZhaWx1cmUgbWVzc2FnZS4=",
								"type":"fatal_failure"
							}]
						}
					],
					"FooTest.TestDoBaz": [
						{
							"elapsed_time_ms": 837,
							"losless_snippet": true,
							"output_snippet": "[ RUN      ] FooTest.TestDoBaz",
							"output_snippet_base64": "WyBSVU4gICAgICBdIEZvb1Rlc3QuVGVzdERvQmF6",
							"status": "SUCCESS",
							"result_parts":[{
								"type":"success"
							}]
						},
						{
							"elapsed_time_ms": 856,
							"losless_snippet": false,
							"output_snippet_base64": "c29tZSBkYXRhIHdpdGggACBhbmQg77u/",
							"status": "SUCCESS",
							"links": {
								"logcat": "https://luci-logdog.appspot.com/v/?s=logcat"
							},
							"tags": {
								"tag_name_1": {
                  "values": ["tag_value_1"]
                },
								"tag_name_2": {
                  "values": ["tag_value_2", "tag_value_3"]
                },
                "tag_name_3": {
                  "value": "tag_value_4"
                }
							},
							"result_parts":[{}]
						}
					]
				}],
				"test_locations": {
					"FooTest.TestDoBar": {
						"file": "../../chrome/browser/foo/test.cc",
						"line": 287
					},
					"FooTest.TestDoBaz": {
						"file": "../../chrome/browser/foo/test.cc",
						"line": 293
					}
				}
			}`

		results := &GTestResults{}
		err := results.ConvertFromJSON(strings.NewReader(str))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, results.AllTests, should.Resemble([]string{"FooTest.TestDoBar", "FooTest.TestDoBaz"}))
		assert.Loosely(t, results.DisabledTests, should.Resemble([]string{"FooTest.TestDoBarDisabled"}))
		assert.Loosely(t, results.GlobalTags, should.Resemble([]string{"CPU_64_BITS", "MODE_RELEASE", "OS_WIN"}))
		assert.Loosely(t, results.PerIterationData, should.Resemble([]map[string][]*GTestRunResult{
			{
				"FooTest.TestDoBar": {
					{
						Status:              "CRASH",
						ElapsedTimeMs:       1837,
						LosslessSnippet:     true,
						OutputSnippetBase64: "WyBSVU4gICAgICBdIEZvb1Rlc3QuVGVzdERvQmFy",
					},
					{
						Status:              "FAILURE",
						ElapsedTimeMs:       1856,
						OutputSnippetBase64: "c29tZSBkYXRhIHdpdGggACBhbmQg77u/",
						ResultParts: []*GTestRunResultPart{
							{
								SummaryBase64: "VGhpcyBpcyBhIGZhaWx1cmUgbWVzc2FnZS4=",
								Type:          "failure",
							},
							{
								SummaryBase64: "VGhpcyBpcyBhIGZhdGFsIGZhaWx1cmUgbWVzc2FnZS4=",
								Type:          "fatal_failure",
							},
						},
					},
				},
				"FooTest.TestDoBaz": {
					{
						Status:              "SUCCESS",
						ElapsedTimeMs:       837,
						LosslessSnippet:     true,
						OutputSnippetBase64: "WyBSVU4gICAgICBdIEZvb1Rlc3QuVGVzdERvQmF6",
						ResultParts: []*GTestRunResultPart{
							{
								Type: "success",
							},
						},
					},
					{
						Status:              "SUCCESS",
						ElapsedTimeMs:       856,
						OutputSnippetBase64: "c29tZSBkYXRhIHdpdGggACBhbmQg77u/",
						Links: map[string]json.RawMessage{
							"logcat": json.RawMessage(
								`"https://luci-logdog.appspot.com/v/?s=logcat"`),
						},
						Tags: map[string]*Tag{
							"tag_name_1": {
								Values: []string{"tag_value_1"},
							},
							"tag_name_2": {
								Values: []string{"tag_value_2", "tag_value_3"},
							},
							"tag_name_3": {
								Value: "tag_value_4",
							},
						},
						ResultParts: []*GTestRunResultPart{{}},
					},
				},
			},
		}))
		assert.Loosely(t, results.TestLocations, should.Resemble(map[string]*Location{
			"FooTest.TestDoBar": {File: "../../chrome/browser/foo/test.cc", Line: 287},
			"FooTest.TestDoBaz": {File: "../../chrome/browser/foo/test.cc", Line: 293},
		}))
	})

	ftt.Run(`all_tests can be empty`, t, func(t *ftt.Test) {
		str := `{
				"all_tests": [],
				"disabled_tests": [],
				"global_tags": ["CPU_64_BITS","MODE_RELEASE","OS_WIN"],
				"per_iteration_data": [{}],
				"test_locations": {}
			}`

		results := &GTestResults{}
		err := results.ConvertFromJSON(strings.NewReader(str))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(results.AllTests), should.BeZero)
	})

	ftt.Run("convertTestResult", t, func(t *ftt.Test) {
		var buf bytes.Buffer
		convert := func(result *GTestRunResult) *sinkpb.TestResult {
			r := &GTestResults{}
			buf := &bytes.Buffer{}
			tr, err := r.convertTestResult(ctx, buf, "testId", "TestName", result)
			assert.Loosely(t, err, should.BeNil)
			return tr
		}
		t.Run("EXCESSIVE_OUTPUT", func(t *ftt.Test) {
			tr := convert(&GTestRunResult{Status: "EXCESSIVE_OUTPUT"})
			assert.Loosely(t, tr.Status, should.Equal(pb.TestStatus_FAIL))
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("gtest_status", "EXCESSIVE_OUTPUT")), should.BeTrue)
		})

		t.Run("NOTRUN", func(t *ftt.Test) {
			tr := convert(&GTestRunResult{Status: "NOTRUN"})
			assert.Loosely(t, tr.Status, should.Equal(pb.TestStatus_SKIP))
			assert.Loosely(t, tr.Expected, should.BeFalse)
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("gtest_status", "NOTRUN")), should.BeTrue)
		})

		t.Run("SKIPPED", func(t *ftt.Test) {
			tr := convert(&GTestRunResult{Status: "SKIPPED"})
			assert.Loosely(t, tr.Status, should.Equal(pb.TestStatus_SKIP))
			assert.Loosely(t, tr.Expected, should.BeTrue)
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("gtest_status", "SKIPPED")), should.BeTrue)
		})

		t.Run("Duration", func(t *ftt.Test) {
			tr := convert(&GTestRunResult{
				Status:        "SUCCESS",
				ElapsedTimeMs: 1e3,
			})
			assert.Loosely(t, tr.Duration.GetSeconds(), should.Equal(1))
			assert.Loosely(t, tr.Duration.GetNanos(), should.BeZero)
		})

		t.Run("snippet", func(t *ftt.Test) {
			t.Run("valid", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status:              "SUCCESS",
					LosslessSnippet:     true,
					OutputSnippetBase64: "WyBSVU4gICAgICBdIEZvb1Rlc3QuVGVzdERvQmFyCigxMCBtcyk=",
				})
				assert.Loosely(t, tr.SummaryHtml, should.Equal(snippetInSummaryHtml))
				assert.Loosely(t, tr.Artifacts["snippet"], should.Resemble(textArt(
					"[ RUN      ] FooTest.TestDoBar\n(10 ms)",
				)))
			})

			t.Run("invalid does not cause a fatal error", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status:              "SUCCESS",
					LosslessSnippet:     true,
					OutputSnippetBase64: "invalid base64",
				})
				assert.Loosely(t, tr.SummaryHtml, should.BeEmpty)
			})

			t.Run("stack trace in artifact", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status:              "FAILURE",
					LosslessSnippet:     true,
					OutputSnippetBase64: "WyBSVU4gICAgICBdIEZvb1Rlc3QuVGVzdERvQmFyCigxMCBtcyk=",
				})
				assert.Loosely(t, tr.SummaryHtml, should.Equal(snippetInSummaryHtml))
				assert.Loosely(t, tr.Artifacts, should.HaveLength(1))
				assert.Loosely(t, tr.Artifacts["snippet"], should.Resemble(textArt(
					"[ RUN      ] FooTest.TestDoBar\n(10 ms)",
				)))
			})
		})

		t.Run("testLocations", func(t *ftt.Test) {
			t.Run(`Works`, func(t *ftt.Test) {
				results := &GTestResults{
					TestLocations: map[string]*Location{
						"TestName": {
							File: "..\\\\TestFile",
							Line: 54,
						},
					},
				}
				tr, err := results.convertTestResult(ctx, &buf, "testId", "TestName", &GTestRunResult{Status: "SUCCESS"})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, tr.TestMetadata, should.Resemble(&pb.TestMetadata{
					Name: "TestName",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "//TestFile",
						Line:     54,
					},
				}))
			})
			t.Run(`Clean path`, func(t *ftt.Test) {
				results := &GTestResults{
					TestLocations: map[string]*Location{
						"TestName": {
							File: "../../TestFile",
							Line: 54,
						},
					},
				}
				tr, err := results.convertTestResult(ctx, &buf, "testId", "TestName", &GTestRunResult{Status: "SUCCESS"})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, tr.TestMetadata.Location, should.Resemble(&pb.TestLocation{
					Repo:     chromiumSrcRepo,
					FileName: "//TestFile",
					Line:     54,
				}))
			})
		})

		t.Run("links", func(t *ftt.Test) {
			tr := convert(&GTestRunResult{
				Status:              "SUCCESS",
				LosslessSnippet:     true,
				OutputSnippetBase64: "invalid base64",
				Links: map[string]json.RawMessage{
					"logcat": json.RawMessage(`{"content": "https://luci-logdog.appspot.com/v/?s=logcat"}`),
				},
			})
			assert.Loosely(t, tr.SummaryHtml, should.Equal(`<ul><li><a href="https://luci-logdog.appspot.com/v/?s=logcat">logcat</a></li></ul>`))
		})

		t.Run("tags", func(t *ftt.Test) {
			tr := convert(&GTestRunResult{
				Status:              "SUCCESS",
				LosslessSnippet:     true,
				OutputSnippetBase64: "invalid base64",
				Tags: map[string]*Tag{
					"tag_name_1": {
						Values: []string{"tag_value_1"},
					},
					"tag_name_2": {
						Values: []string{"tag_value_2", "tag_value_3"},
					},
					"tag_name_3": {
						Value: "tag_value_4",
					},
				},
			})
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("tag_name_1", "tag_value_1")), should.BeTrue)
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("tag_name_2", "tag_value_2")), should.BeTrue)
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("tag_name_2", "tag_value_3")), should.BeTrue)
			assert.Loosely(t, pbutil.StringPairsContain(tr.Tags, pbutil.StringPair("tag_name_3", "tag_value_4")), should.BeTrue)
		})

		t.Run("failure reason", func(t *ftt.Test) {
			t.Run("first failure takes precedence", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							// "This is a failure message."
							SummaryBase64: "VGhpcyBpcyBhIGZhaWx1cmUgbWVzc2FnZS4=",
							Type:          "failure",
							File:          `../some/path/first_failure.cc`,
							Line:          123,
						},
						{
							// "This is a second failure message."
							SummaryBase64: "VGhpcyBpcyBhIHNlY29uZCBmYWlsdXJlIG1lc3NhZ2Uu",
							Type:          "failure",
							File:          `../some/path/second_failure.cc`,
							Line:          456,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: `first_failure.cc(123): ` +
							`This is a failure message.`,
						Errors: []*pb.FailureReason_Error{
							{Message: `first_failure.cc(123): ` +
								`This is a failure message.`},
						},
					}))
			})
			t.Run("first fatal failure takes precedence", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							// "This is a failure message."
							SummaryBase64: "VGhpcyBpcyBhIGZhaWx1cmUgbWVzc2FnZS4=",
							Type:          "failure",
							File:          `../some/path/failure.cc`,
							Line:          123,
						},
						{
							// "This is a fatal failure message."
							SummaryBase64: "VGhpcyBpcyBhIGZhdGFsIGZhaWx1cmUgbWVzc2FnZS4=",
							Type:          "fatal_failure",
							File:          `../some/path/first_fatal.cc`,
							Line:          456,
						},
						{
							// "This is a second fatal failure message."
							SummaryBase64: "VGhpcyBpcyBhIHNlY29uZCBmYXRhbCBmYWlsdXJlIG1lc3NhZ2Uu",
							Type:          "fatal_failure",
							File:          `../some/path/second_fatal.cc`,
							Line:          789,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: `first_fatal.cc(456): This is a ` +
							`fatal failure message.`,
						Errors: []*pb.FailureReason_Error{
							{Message: `first_fatal.cc(456): This is a fatal ` +
								`failure message.`},
						},
					}))
			})
			t.Run("failure result parts take precedence over snippet", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							// "This is a failure message."
							SummaryBase64: "VGhpcyBpcyBhIGZhaWx1cmUgbWVzc2FnZS4=",
							Type:          "failure",
							File:          `../some/path/failure_parts.cc`,
							Line:          456,
						},
					},
					// [FATAL:file_name.cc(123)] Error message.
					OutputSnippetBase64: "W0ZBVEFMOmZpbGVfbmFtZS5jYygxMjMpXSBFcnJvciBtZXNzYWdlLg==",
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: `failure_parts.cc(456): This is ` +
							`a failure message.`,
						Errors: []*pb.FailureReason_Error{
							{Message: `failure_parts.cc(456): This is a ` +
								`failure message.`},
						},
					}))
			})
			t.Run("Google Test trace is removed from failure reason", func(t *ftt.Test) {
				input := "error message\nGoogle Test trace:\nRandom tracing output\n"
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							SummaryBase64: base64.StdEncoding.EncodeToString([]byte(input)),
							Type:          "failure",
							File:          `../some/path/file_name.cc`,
							Line:          123,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: "file_name.cc(123): error message",
						Errors: []*pb.FailureReason_Error{
							{Message: "file_name.cc(123): error message"},
						},
					}))
			})
			t.Run("Leading and trailing spaces are removed from failure reason", func(t *ftt.Test) {
				input := "  error\n message\n  "
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							SummaryBase64: base64.StdEncoding.EncodeToString([]byte(input)),
							Type:          "failure",
							File:          `../some/path/file_name.cc`,
							Line:          123,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: "file_name.cc(123): error\n " +
							"message",
						Errors: []*pb.FailureReason_Error{
							{Message: "file_name.cc(123): error\n message"},
						},
					}))
			})
			t.Run("empty", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status:      "FAILURE",
					ResultParts: []*GTestRunResultPart{},
				})
				assert.Loosely(t, tr.FailureReason, should.BeNil)
			})
			t.Run("primary error message truncated at 1024 bytes", func(t *ftt.Test) {
				var input bytes.Buffer
				var expected bytes.Buffer

				// 18 bytes of header.
				expected.WriteString("filename.cc(123): ")

				// Print 1002 bytes as 334 3-byte runes.
				for i := 0; i < 334; i++ {
					// Use swedish "Place of interest symbol", which encodes as three-bytes, e2 8c 98.
					// See https://blog.golang.org/strings.
					input.WriteRune('\u2318')
					expected.WriteRune('\u2318')
				}
				// Numbers and dots are one byte in UTF-8.
				// Construct input to be (1025 - 18) bytes (18 being the
				// length of the file name and line), which should result
				// in a 1024-byte truncated result.
				input.WriteString("12345")
				expected.WriteString("1...")

				assert.Loosely(t, input.Len(), should.Equal(1025-18))
				assert.Loosely(t, expected.Len(), should.Equal(1024))

				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							SummaryBase64: base64.StdEncoding.EncodeToString(input.Bytes()),
							Type:          "failure",
							File:          `path/file/filename.cc`,
							Line:          123,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: expected.String(),
						Errors: []*pb.FailureReason_Error{
							{Message: expected.String()},
						},
					}))
			})
			t.Run("invalid type does not cause a fatal error", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							// "This is a failure message."
							SummaryBase64: "VGhpcyBpcyBhIGZhaWx1cmUgbWVzc2FnZS4=",
							Type:          "undefined",
							File:          `path/file_name.cc`,
							Line:          123,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.BeNil)
			})
			t.Run("invalid UTF-8 does not cause a fatal error", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							// Encodes (hexadecimal byte) ff, which is invalid UTF-8.
							// See https://www.cl.cam.ac.uk/~mgk25/ucs/examples/UTF-8-test.txt.
							SummaryBase64: "/w8=",
							Type:          "fatal_failure",
							File:          `path/file_name.cc`,
							Line:          123,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.BeNil)
			})
			t.Run("invalid base64 does not cause a fatal error", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					ResultParts: []*GTestRunResultPart{
						{
							SummaryBase64: "Invalid base 64.",
							Type:          "fatal_failure",
							File:          `path/file_name.cc`,
							Line:          123,
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.BeNil)
			})
			t.Run("extracted from snippet", func(t *ftt.Test) {
				tr := convert(&GTestRunResult{
					Status: "FAILURE",
					// [FATAL:file_name.cc(123)] Error message.
					OutputSnippetBase64: "W0ZBVEFMOmZpbGVfbmFtZS5jYygxMjMpXSBFcnJvciBtZXNzYWdlLg==",
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: `file_name.cc(123): Error message.`,
						Errors: []*pb.FailureReason_Error{
							{Message: `file_name.cc(123): Error message.`},
						},
					}))
			})
		})
	})

	ftt.Run("extractFailureReasonFromSnippet", t, func(t *ftt.Test) {
		test := func(input string, expected string) {
			result := extractFailureReasonFromSnippet(ctx, input)
			if expected != "" {
				assert.Loosely(t, result, should.NotBeNil)
				assert.Loosely(t, result.PrimaryErrorMessage, should.Equal(expected))
			} else {
				assert.Loosely(t, result, should.BeNil)
			}
		}
		t.Run("fatal example without DCheck", func(t *ftt.Test) {
			// This example does has in the message and and does not have "Check failed:".
			example := "[70297:775:0716/090328.691561:FATAL:sync_test.cc(928)] AwaitQuiescence() failed."
			test(example, "sync_test.cc(928): AwaitQuiescence() failed.")
		})
		t.Run("fatal DCheck example #1", func(t *ftt.Test) {
			example := "[722:259:FATAL:multiplex_router.cc(181)] Check failed: !client_. "
			test(example, "multiplex_router.cc(181): Check failed: !client_.")
		})
		t.Run("fatal DCheck example #2", func(t *ftt.Test) {
			example := "[27483:27483:0807/204616.124527:FATAL:video_source.mojom.cc(555)] Check failed: !connected. PushVideoStreamSubscription::GetPhotoStateCallback was destroyed"
			test(example, "video_source.mojom.cc(555): Check failed: !connected. PushVideoStreamSubscription::GetPhotoStateCallback was destroyed")
		})
		t.Run("fatal DCheck example #3", func(t *ftt.Test) {
			example := "FATAL ash_unittests[6813:6813]: [display_manager_test_api.cc(160)] Check failed: display_manager_->GetNumDisplays() >= 2U (1 vs. 2)"
			test(example, "display_manager_test_api.cc(160): Check failed: display_manager_->GetNumDisplays() >= 2U (1 vs. 2)")
		})
		t.Run("fatal DCheck example #4", func(t *ftt.Test) {
			example := "[FATAL:gl_context.cc(203)] Check failed: false. "
			test(example, "gl_context.cc(203): Check failed: false.")
		})
		t.Run("non-fatal DCheck example", func(t *ftt.Test) {
			// This example does not have FATAL in the message and relies upon matching "Check failed:".
			example := "../../base/allocator/partition_allocator/partition_root.h(998) Check failed: !slot_span->bucket->is_direct_mapped()"
			test(example, "partition_root.h(998): Check failed: !slot_span->bucket->is_direct_mapped()")
		})
		t.Run("unix line endings", func(t *ftt.Test) {
			example := "blah\n../../base/allocator/partition_allocator/partition_root.h(998) Check failed: !slot_span->bucket->is_direct_mapped()\nblah"
			test(example, "partition_root.h(998): Check failed: !slot_span->bucket->is_direct_mapped()")
		})
		t.Run("windows line endings", func(t *ftt.Test) {
			example := "blah\r\n../../base/allocator/partition_allocator/partition_root.h(998) Check failed: !slot_span->bucket->is_direct_mapped()\r\nblah"
			test(example, "partition_root.h(998): Check failed: !slot_span->bucket->is_direct_mapped()")
		})
		t.Run("GTest Expectation", func(t *ftt.Test) {
			// Test a log with multiple expectation failures, to make
			// sure only one gets picekd up.
			example := `Unrelated log line
../../content/public/test/browser_test_base.cc:718: Failure
Expected equality of these values:
  expected_exit_code_
    Which is: 0
  ContentMain(std::move(params))
    Which is: 1
Stack trace:
#0 0x5640a41b448b content::BrowserTestBase::SetUp()
../../content/public/test/browser_test_base.cc:719: Failure
Expected something else
Stack trace:
#0 0x5640a41b448b content::BrowserTestBase::SetUp()`
			expected := `browser_test_base.cc(718): Expected equality of these values:
  expected_exit_code_
    Which is: 0
  ContentMain(std::move(params))
    Which is: 1`
			test(example, expected)
		})
		t.Run("GTest Expectation (Windows)", func(t *ftt.Test) {
			example := `Unrelated log line
../../chrome/browser/net/network_context_configuration_browsertest.cc(984): error: Expected equality of these values:
  net::ERR_CONNECTION_REFUSED
    Which is: -102
  simple_loader2->NetError()
    Which is: -21
Stack trace:
Backtrace:
	std::__1::unique_ptr<network::ResourceRequest,std::__1::default_delete<network::ResourceRequest> >::reset [0x007A3C5B+7709]`
			expected := `network_context_configuration_browsertest.cc(984): Expected equality of these values:
  net::ERR_CONNECTION_REFUSED
    Which is: -102
  simple_loader2->NetError()
    Which is: -21`
			test(example, expected)
		})
		t.Run("empty snippet", func(t *ftt.Test) {
			example := ""
			test(example, "")
		})
		t.Run("non-matching snippet", func(t *ftt.Test) {
			example := "blah\nblah\n"
			test(example, "")
		})
		t.Run("first fatal error extracted", func(t *ftt.Test) {
			example := "blah\npath/to/file.cc(123) Check failed: bool_expression\n[FATAL:file2.cc(456)] Check failed: second_bool_expression\nblah"
			test(example, "file.cc(123): Check failed: bool_expression")
		})
	})

	ftt.Run("truncateString", t, func(t *ftt.Test) {
		t.Run("one-byte runes", func(t *ftt.Test) {
			t.Run("longer than desired length", func(t *ftt.Test) {
				result := truncateString("12345678", 5)
				assert.Loosely(t, result, should.Equal("12..."))
			})
			t.Run("exactly desired length", func(t *ftt.Test) {
				result := truncateString("12345", 5)
				assert.Loosely(t, result, should.Equal("12345"))
			})
			t.Run("shorter than desired length", func(t *ftt.Test) {
				result := truncateString("1234", 5)
				assert.Loosely(t, result, should.Equal("1234"))
			})
		})
		t.Run("three-byte runes", func(t *ftt.Test) {
			t.Run("longer than desired length", func(t *ftt.Test) {
				result := truncateString("\u2318\u2318\u2318", 7)
				assert.Loosely(t, result, should.Equal("\u2318..."))
			})
			t.Run("exactly desired length", func(t *ftt.Test) {
				result := truncateString("\u2318\u2318", 6)
				assert.Loosely(t, result, should.Equal("\u2318\u2318"))
			})
			t.Run("shorter than desired length", func(t *ftt.Test) {
				result := truncateString("\u2318\u2318", 7)
				assert.Loosely(t, result, should.Equal("\u2318\u2318"))
			})
		})
	})

	ftt.Run(`extractGTestParameters`, t, func(t *ftt.Test) {
		t.Run(`type parametrized`, func(t *ftt.Test) {
			t.Run(`with instantiation`, func(t *ftt.Test) {
				baseID, err := extractGTestParameters("MyInstantiation/FooTest/1.DoesBar")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, baseID, should.Equal("FooTest.DoesBar/MyInstantiation.1"))
			})

			t.Run(`without instantiation`, func(t *ftt.Test) {
				baseID, err := extractGTestParameters("FooTest/1.DoesBar")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, baseID, should.Equal("FooTest.DoesBar/1"))
			})
		})

		t.Run(`value parametrized`, func(t *ftt.Test) {
			t.Run(`with instantiation`, func(t *ftt.Test) {
				baseID, err := extractGTestParameters("MyInstantiation/FooTest.DoesBar/1")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, baseID, should.Equal("FooTest.DoesBar/MyInstantiation.1"))
			})

			t.Run(`without instantiation`, func(t *ftt.Test) {
				baseID, err := extractGTestParameters("FooTest.DoesBar/1")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, baseID, should.Equal("FooTest.DoesBar/1"))
			})
		})

		t.Run(`not parametrized`, func(t *ftt.Test) {
			baseID, err := extractGTestParameters("FooTest.DoesBar")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, baseID, should.Equal("FooTest.DoesBar"))
		})

		t.Run(`with magic prefixes`, func(t *ftt.Test) {
			baseID, err := extractGTestParameters("FooTest.PRE_PRE_MANUAL_DoesBar")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, baseID, should.Equal("FooTest.DoesBar"))
		})

		t.Run(`with JUnit tests`, func(t *ftt.Test) {
			baseID, err := extractGTestParameters("org.chromium.tests#testFoo_sub__param=val")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, baseID, should.Equal("org.chromium.tests#testFoo_sub__param=val"))
		})

		t.Run(`synthetic parameterized test`, func(t *ftt.Test) {
			_, err := extractGTestParameters("GoogleTestVerification.UninstantiatedParamaterizedTestSuite<Suite>")
			assert.Loosely(t, err, should.ErrLike("not a real test"))
			assert.Loosely(t, syntheticTestTag.In(err), should.BeTrue)
		})

		t.Run(`synthetic type parameterized test`, func(t *ftt.Test) {
			_, err := extractGTestParameters("GoogleTestVerification.UninstantiatedTypeParamaterizedTestSuite<Suite>")
			assert.Loosely(t, err, should.ErrLike("not a real test"))
			assert.Loosely(t, syntheticTestTag.In(err), should.BeTrue)
		})

		t.Run(`with unrecognized format`, func(t *ftt.Test) {
			_, err := extractGTestParameters("not_gtest_test")
			assert.Loosely(t, err, should.ErrLike("test id of unknown format"))
		})
	})

	ftt.Run(`ToProtos`, t, func(t *ftt.Test) {
		t.Run("Works", func(t *ftt.Test) {
			results := &GTestResults{
				DisabledTests: []string{"FooTest.TestDoBarDisabled"},
				GlobalTags: []string{
					"OS_LINUX",
				},
				PerIterationData: []map[string][]*GTestRunResult{
					{
						"BazTest.DoesQux": {
							{
								Status: "SUCCESS",
							},
							{
								Status: "FAILURE",
							},
						},
						"GoogleTestVerification.UninstantiatedTypeParamaterizedTestSuite<Suite>": {
							{
								Status: "SUCCESS",
							},
						},
						"FooTest.DoesBar": {
							{
								Status: "EXCESSIVE_OUTPUT",
							},
							{
								Status: "FAILURE_ON_EXIT",
							},
						},
					},
					{
						"BazTest.DoesQux": {
							{
								Status: "SUCCESS",
							},
							{
								Status: "SUCCESS",
							},
						},
						"FooTest.DoesBar": {
							{
								Status: "FAILURE",
							},
							{
								Status: "FAILURE_ON_EXIT",
							},
						},
					},
				},
			}

			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				// Disabled tests.
				{
					TestId:   "FooTest.TestDoBarDisabled",
					Expected: true,
					Status:   pb.TestStatus_SKIP,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.TestDoBarDisabled",
						"disabled_test", "true",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "FooTest.TestDoBarDisabled",
					},
				},
				// Iteration 1.
				{
					TestId:   "BazTest.DoesQux",
					Expected: true,
					Status:   pb.TestStatus_PASS,
					Tags: pbutil.StringPairs(
						"test_name", "BazTest.DoesQux",
						"gtest_status", "SUCCESS",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "BazTest.DoesQux",
					},
				},
				{
					TestId: "BazTest.DoesQux",
					Status: pb.TestStatus_FAIL,
					Tags: pbutil.StringPairs(
						"test_name", "BazTest.DoesQux",
						"gtest_status", "FAILURE",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "BazTest.DoesQux",
					},
				},
				{
					TestId: "FooTest.DoesBar",
					Status: pb.TestStatus_FAIL,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.DoesBar",
						"gtest_status", "EXCESSIVE_OUTPUT",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "FooTest.DoesBar",
					},
				},
				{
					TestId: "FooTest.DoesBar",
					Status: pb.TestStatus_FAIL,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.DoesBar",
						"gtest_status", "FAILURE_ON_EXIT",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "FooTest.DoesBar",
					},
				},

				// Iteration 2.
				{
					TestId:   "BazTest.DoesQux",
					Expected: true,
					Status:   pb.TestStatus_PASS,
					Tags: pbutil.StringPairs(
						"test_name", "BazTest.DoesQux",
						"gtest_status", "SUCCESS",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "BazTest.DoesQux",
					},
				},
				{
					TestId:   "BazTest.DoesQux",
					Expected: true,
					Status:   pb.TestStatus_PASS,
					Tags: pbutil.StringPairs(
						"test_name", "BazTest.DoesQux",
						"gtest_status", "SUCCESS",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "BazTest.DoesQux",
					},
				},
				{
					TestId: "FooTest.DoesBar",
					Status: pb.TestStatus_FAIL,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.DoesBar",
						"gtest_status", "FAILURE",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "FooTest.DoesBar",
					},
				},
				{
					TestId: "FooTest.DoesBar",
					Status: pb.TestStatus_FAIL,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.DoesBar",
						"gtest_status", "FAILURE_ON_EXIT",
						"lossless_snippet", "false",
						"gtest_global_tag", "OS_LINUX",
						"orig_format", "chromium_gtest",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "FooTest.DoesBar",
					},
				},
			}
			assert.Loosely(t, testResults, should.HaveLength(len(expected)))
			for i := range testResults {
				assert.Loosely(t, testResults[i], should.Resemble(expected[i]))
			}
		})
	})
}
