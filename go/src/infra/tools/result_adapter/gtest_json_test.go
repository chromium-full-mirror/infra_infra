// Copyright 2022 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func TestGTestJsonConversions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run(`From JSON works`, t, func(t *ftt.Test) {
		str := `{
				"testsuites": [
					{
						"testsuite": [
							{
								"name": "TestDoBar",
								"classname": "FooTest",
								"time": "0.12s",
								"result": "COMPLETED",
								"failures":[{
									"failure":"file.cc:5\nThis is a failure message."
								}]
							},
							{
								"name": "TestDoBarDisabled",
								"classname": "FooTest",
								"time": "0s",
								"result": "SUPPRESSED"
							}
						]
					},
					{
						"testsuite": [
							{
								"name": "TestDoBaz1",
								"classname": "FooTest",
								"time": "37s",
								"result": "SKIPPED"
							},
							{
								"name": "TestDoBaz2",
								"classname": "FooTest",
								"time": "6s",
								"result": "COMPLETED",
								"failures":[{}]
							}
						]
					}
				]
			}`

		results := &GTestJsonResults{}
		err := results.ConvertFromJSON(strings.NewReader(str))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, results.TestSuites, should.Resemble([]*GTestJsonTestSuites{
			{
				TestSuite: []*GTestJsonTestSuite{
					{
						ClassName: "FooTest",
						Name:      "TestDoBar",
						Result:    "COMPLETED",
						Time:      "0.12s",
						Failures: []*GTestJsonFailures{
							{
								Failure: "file.cc:5\nThis is a failure message.",
							},
						},
					},
					{
						ClassName: "FooTest",
						Name:      "TestDoBarDisabled",
						Result:    "SUPPRESSED",
						Time:      "0s",
					},
				},
			},
			{
				TestSuite: []*GTestJsonTestSuite{
					{
						ClassName: "FooTest",
						Name:      "TestDoBaz1",
						Result:    "SKIPPED",
						Time:      "37s",
					},
					{
						ClassName: "FooTest",
						Name:      "TestDoBaz2",
						Result:    "COMPLETED",
						Time:      "6s",
						Failures:  []*GTestJsonFailures{{}},
					},
				},
			},
		}))
	})

	ftt.Run("convertTestResult", t, func(t *ftt.Test) {
		convert := func(result *GTestJsonTestSuite) *sinkpb.TestResult {
			r := &GTestJsonResults{}
			buf := &bytes.Buffer{}
			tr, err := r.convertTestResult(ctx, buf, "testId", result)
			assert.Loosely(t, err, should.BeNil)
			return tr
		}

		t.Run("SUPPRESSED", func(t *ftt.Test) {
			tr := convert(&GTestJsonTestSuite{Result: "SUPPRESSED"})
			assert.Loosely(t, tr.Status, should.Equal(pb.TestStatus_SKIP))
			assert.Loosely(t, tr.Expected, should.BeTrue)
		})

		t.Run("Duration", func(t *ftt.Test) {
			tr := convert(&GTestJsonTestSuite{
				Result: "COMPLETED",
				Time:   "1s",
			})
			assert.Loosely(t, tr.Duration.GetSeconds(), should.Equal(1))
			assert.Loosely(t, tr.Duration.GetNanos(), should.BeZero)
		})

		t.Run("failure reason", func(t *ftt.Test) {
			t.Run("first failure takes precedence", func(t *ftt.Test) {
				tr := convert(&GTestJsonTestSuite{
					Result: "COMPLETED",
					Failures: []*GTestJsonFailures{
						{
							Failure: "This is a failure message.",
						},
						{
							Failure: "This is a second failure message.",
						},
					},
				})
				assert.Loosely(t, tr.FailureReason, should.Resemble(
					&pb.FailureReason{
						PrimaryErrorMessage: `This is a failure message.`,
						Errors: []*pb.FailureReason_Error{
							{Message: `This is a failure message.`},
						},
					}))
			})
			t.Run("empty", func(t *ftt.Test) {
				tr := convert(&GTestJsonTestSuite{
					Result:   "COMPLETED",
					Failures: []*GTestJsonFailures{},
				})
				assert.Loosely(t, tr.FailureReason, should.BeNil)
			})
		})
	})

	ftt.Run(`ToProtos`, t, func(t *ftt.Test) {
		t.Run("Works", func(t *ftt.Test) {
			results := &GTestJsonResults{
				TestSuites: []*GTestJsonTestSuites{
					{
						TestSuite: []*GTestJsonTestSuite{
							{
								ClassName: "BazTest",
								Name:      "DoesQux1",
								Result:    "COMPLETED",
							},
							{
								ClassName: "BazTest",
								Name:      "DoesQux2",
								Result:    "COMPLETED",
								Failures: []*GTestJsonFailures{
									{
										Failure: "file.cc:5\nThis is a failure message.",
									},
								},
							},
							{
								ClassName: "FooTest",
								Name:      "TestDoBarDisabled",
								Result:    "SUPPRESSED",
							},
						},
					},
					{
						TestSuite: []*GTestJsonTestSuite{
							{
								ClassName: "FooTest",
								Name:      "DoesBar",
								Result:    "SKIPPED",
							},
						},
					},
				},
			}

			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:   "BazTest.DoesQux1",
					Expected: true,
					Status:   pb.TestStatus_PASS,
					Tags: pbutil.StringPairs(
						"test_name", "BazTest.DoesQux1",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "BazTest.DoesQux1",
					},
				},
				{
					TestId: "BazTest.DoesQux2",
					Status: pb.TestStatus_FAIL,
					Tags: pbutil.StringPairs(
						"test_name", "BazTest.DoesQux2",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "BazTest.DoesQux2",
						Location: &pb.TestLocation{
							Repo:     "https://webrtc.googlesource.com/src/",
							FileName: "//file.cc",
							Line:     5,
						},
					},
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "file.cc:5\nThis is a failure message.",
						Errors: []*pb.FailureReason_Error{
							{Message: "file.cc:5\nThis is a failure message."},
						},
					},
					SummaryHtml: "<p>file.cc:5\nThis is a failure message.</p>",
				},
				{
					TestId:   "FooTest.TestDoBarDisabled",
					Expected: true,
					Status:   pb.TestStatus_SKIP,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.TestDoBarDisabled",
						"disabled_test", "true",
					),
					TestMetadata: &pb.TestMetadata{
						Name: "FooTest.TestDoBarDisabled",
					},
				},
				{
					TestId:   "FooTest.DoesBar",
					Expected: true,
					Status:   pb.TestStatus_SKIP,
					Tags: pbutil.StringPairs(
						"test_name", "FooTest.DoesBar",
						"disabled_test", "true",
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
