// Copyright 2020 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/golang/protobuf/ptypes/duration"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func TestJSONConversions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run(`From JSON works`, t, func(t *ftt.Test) {
		buf := []byte(`
		{
			"version": 3,
			"interrupted": false,
			"path_delimiter": "::",
			"metadata": {"test_name_prefix": "prefix."},
			"builder_name": "Linux Tests",
			"build_number": "82046",
			"tests": {
				"c1": {
					"c2": {
						"t1.html": {
							"actual": "PASS PASS PASS",
							"expected": "PASS",
							"time": 0.3,
							"times": [0.2, 0.1]
						},
						"t2.html": {
							"actual": "PASS FAIL PASS",
							"expected": "PASS FAIL",
							"times": [0.05]
						}
					}
				},
				"c2": {
					"t3.html": {
						"actual": "FAIL",
						"expected": "PASS",
						"artifacts": {
							"log": ["relative/path/to/log"]
						}
					}
				},
				"c3": {
					"time": {
						"time-t1.html": {
							"actual": "PASS",
							"expected": "PASS",
							"time": 0.4,
							"artifacts": {
								"reason": "inlined string"
							}
						}
					}
				}
			}
		}`)

		t.Run(`Works`, func(t *ftt.Test) {
			results := &JSONTestResults{}
			err := results.ConvertFromJSON(bytes.NewReader(buf))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, results, should.NotBeNil)
			assert.Loosely(t, results.Version, should.Equal(3))
			assert.Loosely(t, results.Interrupted, should.Equal(false))
			assert.Loosely(t, results.PathDelimiter, should.Equal("::"))
			assert.Loosely(t, results.BuildNumber, should.Equal("82046"))
			assert.Loosely(t, results.BuilderName, should.Equal("Linux Tests"))
			assert.Loosely(t, results.Tests, should.Resemble(map[string]*TestFields{
				"prefix.c1::c2::t1.html": {
					Actual:   "PASS PASS PASS",
					Expected: "PASS",
					Time:     0.3,
					Times:    []float64{0.2, 0.1},
				},
				"prefix.c1::c2::t2.html": {
					Actual:   "PASS FAIL PASS",
					Expected: "PASS FAIL",
					Times:    []float64{0.05},
				},
				"prefix.c2::t3.html": {
					Actual:   "FAIL",
					Expected: "PASS",
					ArtifactsRaw: map[string]json.RawMessage{
						"log": json.RawMessage(`["relative/path/to/log"]`),
					},
					Artifacts: map[string][]string{
						"log": {"relative/path/to/log"},
					},
				},
				"prefix.c3::time::time-t1.html": {
					Actual:   "PASS",
					Expected: "PASS",
					Time:     0.4,
					ArtifactsRaw: map[string]json.RawMessage{
						"reason": json.RawMessage(`"inlined string"`),
					},
				},
			}))

			t.Run(`with default path delimiter`, func(t *ftt.Test) {
				// Clear the delimiter and already processed and flattened tests.
				results.PathDelimiter = ""
				results.Tests = make(map[string]*TestFields)

				err := results.convertTests("", results.TestsRaw)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, results, should.NotBeNil)

				paths := make([]string, 0, len(results.Tests))
				for path := range results.Tests {
					paths = append(paths, path)
				}
				sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
				assert.Loosely(t, paths, should.Resemble([]string{
					"prefix.c1/c2/t1.html",
					"prefix.c1/c2/t2.html",
					"prefix.c2/t3.html",
					"prefix.c3/time/time-t1.html",
				}))
			})
		})
	})

	ftt.Run(`ToProtos works`, t, func(t *ftt.Test) {
		results := &JSONTestResults{
			Interrupted: true,
			Metadata: map[string]json.RawMessage{
				"tags": json.RawMessage(`["linux", "ubuntu", "desktop"]`),
			},
			Version: 3,
			Tests: map[string]*TestFields{
				"c1/c2/t1.html": {
					Actual:   "PASS PASS PASS",
					Expected: "PASS",
					Time:     0.3,
					Times:    []float64{0.3, 0.2, 0.1},
					Artifacts: map[string][]string{
						"isolate_object_list": {
							"harness/log.txt",
							"harness/retry_1/log.txt",
							"harness/retry_2/log.txt",
							"harness/retry_wat/log.txt",
						},
						"ref_mismatch": {
							"relative/path/to/retry_2/about:blank",
						},
					},
				},
				"c1/c2/t2.html": {
					Actual:   "PASS FAIL PASS CRASH",
					Expected: "PASS FAIL",
					Times:    []float64{0.05, 0.05, 0.05, 0.05},
				},
				"c2/t3.html": {
					Actual:   "FAIL",
					Expected: "PASS",
					Artifacts: map[string][]string{
						"isolate_object": {"relative/path/to/log.txt"},
						"gold_triage_link": {
							"https://chrome-gpu-gold.skia.org/detail?test=foo&digest=beef",
						},
						"isolate_object_list": {
							"relative/path/to/diff.png",
							"unknown",
						},
					},
				},
				"c2/t4.html": {
					Actual:   "PASS PASS PASS",
					Expected: "PASS",
					Time:     0.3,
				},
				"c2/t5.html": {
					Actual:   "SKIP",
					Expected: "PASS",
				},
				"c2/t6.html": {
					Actual:   "PASS",
					Expected: "",
				},
			},
		}

		normPathToFullPath := map[string]string{
			"harness/log.txt":           "/artifacts/harness/log.txt",
			"harness/retry_1/log.txt":   "/artifacts/harness/retry_1/log.txt",
			"harness/retry_2/log.txt":   "/artifacts/harness/retry_2/log.txt",
			"relative/path/to/log.txt":  "/artifacts/relative/path/to/log.txt",
			"relative/path/to/diff.png": "/artifacts/relative/path/to/diff.png",
		}

		testResults, err := results.ToProtos(ctx, normPathToFullPath, true)
		assert.Loosely(t, err, should.BeNil)

		assertTestResultsResemble(t, testResults, []*sinkpb.TestResult{
			// Test 1.
			{
				TestId:   "c1/c2/t1.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Duration: &duration.Duration{Nanos: 3e8},
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c1/c2/t1.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				Artifacts: map[string]*sinkpb.Artifact{
					"isolate_object_list": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/artifacts/harness/log.txt"},
					},
				},
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t1.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t1.html",
					},
				},
			},
			{
				TestId:   "c1/c2/t1.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Duration: &duration.Duration{Nanos: 2e8},
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c1/c2/t1.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				Artifacts: map[string]*sinkpb.Artifact{
					"isolate_object_list": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/artifacts/harness/retry_1/log.txt"},
					},
				},
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t1.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t1.html",
					},
				},
			},
			{
				TestId:   "c1/c2/t1.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Duration: &duration.Duration{Nanos: 1e8},
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c1/c2/t1.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				Artifacts: map[string]*sinkpb.Artifact{
					"isolate_object_list": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/artifacts/harness/retry_2/log.txt"},
					},
				},
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t1.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t1.html",
					},
				},
			},

			// Test 2.
			{
				TestId:   "c1/c2/t2.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Duration: &duration.Duration{Nanos: 5e7},
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c1/c2/t2.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t2.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t2.html",
					},
				},
			},
			{
				TestId:   "c1/c2/t2.html",
				Status:   pb.TestStatus_FAIL,
				Expected: true,
				Duration: &duration.Duration{Nanos: 5e7},
				Tags: pbutil.StringPairs(
					"json_format_status", "FAIL",
					"test_name", "c1/c2/t2.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t2.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t2.html",
					},
				},
			},
			{
				TestId:   "c1/c2/t2.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Duration: &duration.Duration{Nanos: 5e7},
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c1/c2/t2.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t2.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t2.html",
					},
				},
			},
			{
				TestId:   "c1/c2/t2.html",
				Status:   pb.TestStatus_CRASH,
				Expected: false,
				Duration: &duration.Duration{Nanos: 5e7},
				Tags: pbutil.StringPairs(
					"json_format_status", "CRASH",
					"test_name", "c1/c2/t2.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c1/c2/t2.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c1/c2/t2.html",
					},
				},
			},

			// Test 3
			{
				TestId:   "c2/t3.html",
				Status:   pb.TestStatus_FAIL,
				Expected: false,
				Tags: pbutil.StringPairs(
					"json_format_status", "FAIL",
					"test_name", "c2/t3.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				SummaryHtml: `<ul><li><a href="https://chrome-gpu-gold.skia.org/detail?test=foo&digest=beef">gold_triage_link</a></li></ul>`,
				Artifacts: map[string]*sinkpb.Artifact{
					"isolate_object_list": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/artifacts/relative/path/to/diff.png"},
					},
					"isolate_object": {
						Body: &sinkpb.Artifact_FilePath{FilePath: "/artifacts/relative/path/to/log.txt"},
					},
				},
				TestMetadata: &pb.TestMetadata{
					Name: "c2/t3.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c2/t3.html",
					},
				},
			},

			// Test 4
			{
				TestId:   "c2/t4.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Duration: &duration.Duration{Nanos: 3e8},
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c2/t4.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR),
				TestMetadata: &pb.TestMetadata{
					Name: "c2/t4.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c2/t4.html",
					},
				},
			},
			{
				TestId:   "c2/t4.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c2/t4.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c2/t4.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c2/t4.html",
					},
				},
			},
			{
				TestId:   "c2/t4.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c2/t4.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c2/t4.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c2/t4.html",
					},
				},
			},

			// Test 5
			{
				TestId:   "c2/t5.html",
				Status:   pb.TestStatus_SKIP,
				Expected: true,
				Tags: pbutil.StringPairs(
					"json_format_status", "SKIP",
					"test_name", "c2/t5.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c2/t5.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c2/t5.html",
					},
				},
			},

			// Test 6
			{
				TestId:   "c2/t6.html",
				Status:   pb.TestStatus_PASS,
				Expected: true,
				Tags: pbutil.StringPairs(
					"json_format_status", "PASS",
					"test_name", "c2/t6.html",
					"json_format_tag", "linux",
					"json_format_tag", "ubuntu",
					"json_format_tag", "desktop",
					originalFormatTagKey, formatJTR,
				),
				TestMetadata: &pb.TestMetadata{
					Name: "c2/t6.html",
					Location: &pb.TestLocation{
						Repo:     chromiumSrcRepo,
						FileName: "c2/t6.html",
					},
				},
			},
		})
	})
}

func assertTestResultsResemble(t testing.TB, actual, expected []*sinkpb.TestResult) {
	t.Helper()

	assert.Loosely(t, actual, should.HaveLength(len(expected)), truth.LineContext())
	for i := range actual {
		assert.Loosely(t, actual[i], should.Resemble(expected[i]), truth.LineContext())
	}
}

func TestArtifactUtils(t *testing.T) {
	t.Parallel()

	ftt.Run(`Checking subdirs`, t, func(t *ftt.Test) {
		ctx := context.Background()

		normToFull := map[string]string{
			"artifacts/a/stdout.txt":           "/root/artifacts/a/stdout.txt",
			"artifacts/b/stderr.txt":           "\\root\\artifacts\\b\\stderr.txt",
			"layout-test-results/c/stderr.txt": "/root/layout-test-results/c/stderr.txt",
			"d/stderr.txt":                     "/root/d/stderr.txt",
		}
		f := &TestFields{Artifacts: map[string][]string{
			"a": {"a/stdout.txt"},
			"b": {"b\\stderr.txt"},
			"c": {"c/stderr.txt"},
			"d": {"d/stderr.txt"},
		}}

		artifactsPerRun := f.parseArtifacts(ctx, "testID", normToFull)
		assert.Loosely(t, artifactsPerRun, should.HaveLength(1))

		arts := artifactsPerRun[0].artifacts

		expected := map[string]*sinkpb.Artifact{
			"a": {
				Body: &sinkpb.Artifact_FilePath{FilePath: "/root/artifacts/a/stdout.txt"},
			},
			"b": {
				Body: &sinkpb.Artifact_FilePath{FilePath: "\\root\\artifacts\\b\\stderr.txt"},
			},
			"c": {
				Body: &sinkpb.Artifact_FilePath{FilePath: "/root/layout-test-results/c/stderr.txt"},
			},
			"d": {
				Body: &sinkpb.Artifact_FilePath{FilePath: "/root/d/stderr.txt"},
			},
		}

		for i := range arts {
			assert.Loosely(t, arts[i], should.Resemble(expected[i]))
		}
	})
}
