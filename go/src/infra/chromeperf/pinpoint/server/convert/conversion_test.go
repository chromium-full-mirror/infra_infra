// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package convert

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/convey"
	"go.chromium.org/luci/common/testing/truth/should"

	pinpoint_proto "go.chromium.org/infra/chromeperf/pinpoint/proto"
)

const testPriority = 42

func shouldContainMap(actual interface{}, expected ...interface{}) string {
	v := actual.(url.Values)
	e := expected[0].(map[string]interface{})

	// Go through the list of expected keys and values and compare.
	for key, value := range e {
		actualValue, found := v[key]
		if !found {
			return fmt.Sprintf("expecting key '%s' (but is not there)", key)
		}
		switch value.(type) {
		case string:
			if actualValue[0] != value {
				return fmt.Sprintf("expecting actual['%s'] == %s (got %s instead)", key, value, actualValue[0])
			}
		case float64:
			if actualValue[0] != fmt.Sprintf("%f", value) {
				return fmt.Sprintf("expecting actual['%s'] == %f (got %s instead)", key, value, actualValue)
			}
		default:
			panic("Unsupported type!")
		}
	}
	return ""
}

func TestSimpleConversions(t *testing.T) {
	t.Parallel()
	job := &pinpoint_proto.JobSpec{
		Config:    "some-config",
		Target:    "some-build-target",
		UserAgent: "pinpoint/unittest",
	}

	ftt.Run("We support Bisections without a Patch", t, func(t *ftt.Test) {
		job.JobKind = &pinpoint_proto.JobSpec_Bisection{
			Bisection: &pinpoint_proto.Bisection{
				CommitRange: &pinpoint_proto.GitilesCommitRange{
					Host:         "gitiles-host",
					Project:      "gitiles-project",
					StartGitHash: "c0dec0de",
					EndGitHash:   "f00dc0de",
				}}}

		t.Run("Creating a Performance mode job", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_PERFORMANCE

			t.Run("We support Telemetry specifying a story", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 1000.0,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story"},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we have the user agent.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"user_agent": "pinpoint/unittest",
				}))

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"story":          "some-story",
					"metric":         "some-metric",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "performance",
					"comparison_magnitude": 1000.0,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))

				// Check that priority is unset
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"priority": "0"}))
			})

			t.Run("We support Telemetry specifying priority", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						Priority:            testPriority,
						ComparisonMagnitude: 1000.0,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story"},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"priority": fmt.Sprintf("%d", testPriority)}))
			})

			t.Run("We support Telemetry specifying story tags", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 1000.0,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "performance",
					"comparison_magnitude": 1000.0,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))

			})

			t.Run("We support GTest", func(t *ftt.Test) {
				gtestJob := &pinpoint_proto.JobSpec{
					ComparisonMagnitude: 1000.0,
					Arguments: &pinpoint_proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &pinpoint_proto.GTestBenchmark{
							Benchmark:   "some-benchmark",
							Measurement: "some-metric",
							Test:        "some-test"}}}
				proto.Merge(gtestJob, job)
				v, err := JobToValues(gtestJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check the conversion of values to maps.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark": "some-benchmark",
					"trace":     "some-test",
					"chart":     "some-metric"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "performance",
					"comparison_magnitude": 1000.0,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))
			})

		})

		t.Run("Creating a Functional Comparison", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_FUNCTIONAL

			t.Run("We support Telemetry specifying a story", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 0.2,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story"},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"story":          "some-story",
					"metric":         "some-metric",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "functional",
					"comparison_magnitude": 0.2,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))

			})

			t.Run("We support Telemetry specifying story tags", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 0.2,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "functional",
					"comparison_magnitude": 0.2,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))

			})

			t.Run("We support GTest", func(t *ftt.Test) {
				gtestJob := &pinpoint_proto.JobSpec{
					ComparisonMagnitude: 0.2,
					Arguments: &pinpoint_proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &pinpoint_proto.GTestBenchmark{
							Benchmark:   "some-benchmark",
							Measurement: "some-metric",
							Test:        "some-test"}}}
				proto.Merge(gtestJob, job)
				v, err := JobToValues(gtestJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check the conversion of values to maps.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark": "some-benchmark",
					"trace":     "some-test",
					"chart":     "some-metric"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "functional",
					"comparison_magnitude": 0.2,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))
			})

		})

	})

	ftt.Run("We support Bisections with a Patch", t, func(t *ftt.Test) {
		job.JobKind = &pinpoint_proto.JobSpec_Bisection{
			Bisection: &pinpoint_proto.Bisection{
				CommitRange: &pinpoint_proto.GitilesCommitRange{
					Host:         "gitiles-host",
					Project:      "gitiles-project",
					StartGitHash: "c0dec0de",
					EndGitHash:   "f00dc0de",
				},
				Patch: &pinpoint_proto.GerritChange{
					Host:     "some-gerrit-host",
					Project:  "some-project",
					Change:   12345,
					Patchset: 1}}}

		t.Run("Creating a Performance mode job", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_PERFORMANCE

			t.Run("We support Telemetry specifying a story", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 1000.0,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story"},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"story":          "some-story",
					"metric":         "some-metric",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "performance",
					"comparison_magnitude": 1000.0,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de",
					// Here we're hard-coding the expected URL, as it's required by the legacy
					// Pinpoint API.
					"patch": "https://some-gerrit-host/c/12345/1"}))
			})

			t.Run("We support Telemetry specifying story tags", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 1000.0,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "performance",
					"comparison_magnitude": 1000.0,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de",
					// Here we're hard-coding the expected URL, as it's required by the legacy
					// Pinpoint API.
					"patch": "https://some-gerrit-host/c/12345/1"}))

			})
			t.Run("We support GTest", func(t *ftt.Test) {
				gtestJob := &pinpoint_proto.JobSpec{
					ComparisonMagnitude: 1000.0,
					Arguments: &pinpoint_proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &pinpoint_proto.GTestBenchmark{
							Benchmark:   "some-benchmark",
							Measurement: "some-metric",
							Test:        "some-test"}}}
				proto.Merge(gtestJob, job)
				v, err := JobToValues(gtestJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check the conversion of values to maps.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark": "some-benchmark",
					"trace":     "some-test",
					"chart":     "some-metric"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "performance",
					"comparison_magnitude": 1000.0,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de",
					// Here we're hard-coding the expected URL, as it's required by the legacy
					// Pinpoint API.
					"patch": "https://some-gerrit-host/c/12345/1"}))
			})

		})

		t.Run("Creating a Functional Comparison", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_FUNCTIONAL

			t.Run("We support Telemetry specifying a story", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 0.2,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story"},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"story":          "some-story",
					"metric":         "some-metric",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "functional",
					"comparison_magnitude": 0.2,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de",
					// Here we're hard-coding the expected URL, as it's required by the legacy
					// Pinpoint API.
					"patch": "https://some-gerrit-host/c/12345/1"}))

			})

			t.Run("We support Telemetry specifying story tags", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						ComparisonMagnitude: 0.2,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "functional",
					"comparison_magnitude": 0.2,
				}))

				// Check that we also get the bisection details correct.
				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de",
					// Here we're hard-coding the expected URL, as it's required by the legacy
					// Pinpoint API.
					"patch": "https://some-gerrit-host/c/12345/1"}))

			})

			t.Run("We support GTest", func(t *ftt.Test) {
				gtestJob := &pinpoint_proto.JobSpec{
					ComparisonMagnitude: 0.2,
					Arguments: &pinpoint_proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &pinpoint_proto.GTestBenchmark{
							Benchmark:   "some-benchmark",
							Measurement: "some-metric",
							Test:        "some-test"}}}
				proto.Merge(gtestJob, job)
				v, err := JobToValues(gtestJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check the conversion of values to maps.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark": "some-benchmark",
					"trace":     "some-test",
					"chart":     "some-metric"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":        "some-config",
					"comparison_mode":      "functional",
					"comparison_magnitude": 0.2,
				}))

				// Check that we also get the bisection details correct.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"start_git_hash": "c0dec0de",
					"end_git_hash":   "f00dc0de"}))
			})

		})

	})

	ftt.Run("We fail on experiments with missing inputs", t, func(t *ftt.Test) {
		job.JobKind = &pinpoint_proto.JobSpec_Experiment{
			Experiment: &pinpoint_proto.Experiment{
				BaseCommit: &pinpoint_proto.GitilesCommit{
					Host:    "some-gitiles-host",
					Project: "some-gitiles-project",
					GitHash: "c0dec0de",
				},
				ExperimentPatch: &pinpoint_proto.GerritChange{
					Host:     "some-gerrit-host",
					Project:  "some-gerrit-project",
					Change:   12345,
					Patchset: 0,
				},
			}}
		t.Run("Creating a performance mode job", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_PERFORMANCE
			telemetryJob :=
				&pinpoint_proto.JobSpec{
					Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
						TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
							Benchmark: "some-benchmark",
							StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
								Story: "some-story",
							},
							Measurement:   "some-metric",
							GroupingLabel: "some-grouping-label",
							Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
			proto.Merge(telemetryJob, job)
			t.Run("No base commit", func(t *ftt.Test) {
				telemetryJob.GetExperiment().BaseCommit = nil
				_, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.NotBeNil)
			})
			t.Run("No user configuration", func(t *ftt.Test) {
				telemetryJob.Config = ""
				_, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.NotBeNil)
			})
			t.Run("No target", func(t *ftt.Test) {
				telemetryJob.Target = ""
				_, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.NotBeNil)
			})
		})
	})

	ftt.Run("We support experiments with base commit and experiment patch", t, func(t *ftt.Test) {
		job.JobKind = &pinpoint_proto.JobSpec_Experiment{
			Experiment: &pinpoint_proto.Experiment{
				BaseCommit: &pinpoint_proto.GitilesCommit{
					Host:    "some-gitiles-host",
					Project: "some-gitiles-project",
					GitHash: "c0dec0de",
				},
				ExperimentPatch: &pinpoint_proto.GerritChange{
					Host:     "some-gerrit-host",
					Project:  "some-gerrit-project",
					Change:   23456,
					Patchset: 1,
				}}}

		t.Run("Creating a Performance mode job", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_PERFORMANCE

			t.Run("We support Telemetry specifying a story", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story",
								},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"story":          "some-story",
					"metric":         "some-metric",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration": "some-config",
					// In legacy Pinpoint, an experiment is a "try" comparison mode.
					"comparison_mode": "try",
				}))

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"base_git_hash":    "c0dec0de",
					"experiment_patch": "https://some-gerrit-host/c/23456/1"}))

			})

			t.Run("We support having both the base commit and experiment commit", func(t *ftt.Test) {
				job.GetExperiment().ExperimentCommit = &pinpoint_proto.GitilesCommit{
					Host:    "some-gitiles-host",
					Project: "some-gitiles-project",
					GitHash: "60061ec0de",
				}
				telemetryJob := &pinpoint_proto.JobSpec{
					Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
						TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
							Benchmark: "some-benchmark",
							StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
								Story: "some-story",
							},
							Measurement:   "some-metric",
							GroupingLabel: "some-grouping-label",
							Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"end_git_hash": "60061ec0de",
				}))
			})

			t.Run("We support Telemetry specifying story tags", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration": "some-config",
					// In legacy Pinpoint, an experiment is a "try" comparison mode.
					"comparison_mode": "try",
				}))

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"base_git_hash":    "c0dec0de",
					"experiment_patch": "https://some-gerrit-host/c/23456/1"}))

			})

			t.Run("We support Telemetry specifying story tags and extra args", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE,
								ExtraArgs:     []string{"--browser", "some-browser"},
							}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration": "some-config",
					// In legacy Pinpoint, an experiment is a "try" comparison mode.
					"comparison_mode": "try",
				}))

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"base_git_hash":    "c0dec0de",
					"experiment_patch": "https://some-gerrit-host/c/23456/1",
					"extra_test_args":  `["--browser","some-browser"]`,
				}))

			})

			t.Run("We support jobs with a Batch ID and Attempts", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						BatchId:             "defined-job-id",
						InitialAttemptCount: 42,
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_StoryTags{
									StoryTags: &pinpoint_proto.TelemetryBenchmark_StoryTagList{
										StoryTags: []string{"some-tag", "some-other-tag"},
									}},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE,
								ExtraArgs:     []string{"--browser", "some-browser"},
							}}}
				proto.Merge(telemetryJob, job)
				v, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
					"batch_id":      "defined-job-id",
				}))

				// Check that we have the required Telemetry fields in the JSON.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark":      "some-benchmark",
					"metric":         "some-metric",
					"story_tags":     "some-tag,some-other-tag",
					"grouping_label": "some-grouping-label"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration": "some-config",
					// In legacy Pinpoint, an experiment is a "try" comparison mode.
					"comparison_mode": "try",
				}))

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"base_git_hash":    "c0dec0de",
					"experiment_patch": "https://some-gerrit-host/c/23456/1",
					"extra_test_args":  `["--browser","some-browser"]`,
				}))

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"initial_attempt_count": "42",
				}))
			})

			t.Run("We support GTest", func(t *ftt.Test) {
				gtestJob := &pinpoint_proto.JobSpec{
					Arguments: &pinpoint_proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &pinpoint_proto.GTestBenchmark{
							Benchmark:   "some-benchmark",
							Measurement: "some-metric",
							Test:        "some-test"}}}
				proto.Merge(gtestJob, job)
				v, err := JobToValues(gtestJob, "user@example.com")
				assert.Loosely(t, err, should.BeNil)

				// Check that we support the required fields for all Pinpoint jobs.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"target":        "some-build-target",
					"configuration": "some-config",
				}))

				// Check the conversion of values to maps.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"benchmark": "some-benchmark",
					"trace":     "some-test",
					"chart":     "some-metric"}))

				// Check that we have base job configurations are set.
				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"configuration":   "some-config",
					"comparison_mode": "try",
				}))

				assert.Loosely(t, v, convey.Adapt(shouldContainMap)(map[string]interface{}{
					"base_git_hash":    "c0dec0de",
					"experiment_patch": "https://some-gerrit-host/c/23456/1"}))
			})

		})

		t.Run("Creating a Functional mode job", func(t *ftt.Test) {
			job.ComparisonMode = pinpoint_proto.JobSpec_FUNCTIONAL

			t.Run("Fails for Telemetry (unsupported)", func(t *ftt.Test) {
				telemetryJob :=
					&pinpoint_proto.JobSpec{
						Arguments: &pinpoint_proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &pinpoint_proto.TelemetryBenchmark{
								Benchmark: "some-benchmark",
								StorySelection: &pinpoint_proto.TelemetryBenchmark_Story{
									Story: "some-story"},
								Measurement:   "some-metric",
								GroupingLabel: "some-grouping-label",
								Statistic:     pinpoint_proto.TelemetryBenchmark_NONE}}}
				proto.Merge(telemetryJob, job)
				_, err := JobToValues(telemetryJob, "user@example.com")
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, fmt.Sprintf("%v", err), should.ContainSubstring("functional experiments not supported"))
			})

			t.Run("Fails for GTest (unsupported)", func(t *ftt.Test) {
				gtestJob := &pinpoint_proto.JobSpec{
					Arguments: &pinpoint_proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &pinpoint_proto.GTestBenchmark{
							Benchmark:   "some-benchmark",
							Measurement: "some-metric",
							Test:        "some-test"}}}
				proto.Merge(gtestJob, job)
				_, err := JobToValues(gtestJob, "user@example.com")
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, fmt.Sprintf("%v", err), should.ContainSubstring("functional experiments not supported"))
			})
		})

	})

}

func TestGerritChangeToURL(t *testing.T) {
	t.Parallel()
	ftt.Run("Given valid GerritChange", t, func(t *ftt.Test) {
		c := &pinpoint_proto.GerritChange{
			Host:    "host",
			Project: "project",
			Change:  123456,
		}
		t.Run("When the patchset is provided", func(t *ftt.Test) {
			c.Patchset = 1
			t.Run("Then we see the patchset in the URL", func(t *ftt.Test) {
				u, err := gerritChangeToURL(c)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, u, should.Equal("https://host/c/123456/1"))
			})
		})
		t.Run("When the patset is not provided", func(t *ftt.Test) {
			t.Run("Then we see no patchset in the URL", func(t *ftt.Test) {
				u, err := gerritChangeToURL(c)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, u, should.NotHaveSuffix("/1"))
				assert.Loosely(t, u, should.Equal("https://host/c/123456"))
			})
		})
	})
	ftt.Run("Given an invalidly GerritChange", t, func(t *ftt.Test) {
		c := &pinpoint_proto.GerritChange{
			Host:     "host",
			Project:  "project",
			Change:   123456,
			Patchset: 7,
		}

		t.Run("When it is missing a host", func(t *ftt.Test) {
			c.Host = ""
			t.Run("Then conversion fails", func(t *ftt.Test) {
				_, err := gerritChangeToURL(c)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err.Error(), should.ContainSubstring("host"))
			})
		})
		t.Run("When it is missing a change", func(t *ftt.Test) {
			c.Change = 0
			t.Run("Then conversion fails", func(t *ftt.Test) {
				_, err := gerritChangeToURL(c)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err.Error(), should.ContainSubstring("change"))
			})
		})
	})
}

func TestJobToProto(t *testing.T) {
	t.Parallel()
	ftt.Run("Given a defined experiment", t, func(t *ftt.Test) {
		lj, err := os.ReadFile("../testdata/defined-job-experiment.json")
		assert.Loosely(t, err, should.BeNil)
		t.Run("When we convert the legacy JSON", func(t *ftt.Test) {
			p, err := JobToProto(strings.NewReader(string(lj)))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, p, should.NotBeNil)
			t.Run("Then we find the experiment URLs", func(t *ftt.Test) {
				results := p.GetAbExperimentResults()
				assert.Loosely(t, results, should.NotBeNil)
				assert.Loosely(t, results.AChangeResult.Attempts, should.HaveLength(10))
				assert.Loosely(t, results.BChangeResult.Attempts, should.HaveLength(10))

				// These are typical 3 steps for a legacy job
				quests := []string{"Build", "Test", "Get values"}

				// We know that legacy jobs have 2-3 executions per attempt. This corresponds with the Build, Test,
				// Value quest executions, which is defined for most Pinpoint A/B experiments.
				for _, a := range results.AChangeResult.Attempts {
					assert.Loosely(t, len(a.Executions), should.BeBetweenOrEqual(2, 3))
					for i, e := range a.Executions {
						assert.Loosely(t, e.Label, should.Equal(quests[i]))
					}
				}
				for _, a := range results.BChangeResult.Attempts {
					assert.Loosely(t, len(a.Executions), should.BeBetweenOrEqual(2, 3))
					for i, e := range a.Executions {
						assert.Loosely(t, e.Label, should.Equal(quests[i]))
					}
				}
			})
		})
	})
	ftt.Run("Given an experiment with a batch id", t, func(t *ftt.Test) {
		lj, err := os.ReadFile("../testdata/defined-job-experiment-with-batch-id.json")
		assert.Loosely(t, err, should.BeNil)
		t.Run("When we convert the legacy JSON", func(t *ftt.Test) {
			p, err := JobToProto(strings.NewReader(string(lj)))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, p, should.NotBeNil)
			t.Run("Then we find the batch ID", func(t *ftt.Test) {
				assert.Loosely(t, p.JobSpec.BatchId, should.Equal("batch-id-in-file"))
			})
		})
	})
}
