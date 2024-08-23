// Copyright 2021 The Chromium Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/chromeperf/pinpoint/proto"
)

func TestAnalyzeTelemetryExperiment(t *testing.T) {
	t.Parallel()
	// TODO: add tests for unhappy paths with errors
	// TODO: add more fine-grained unit tests for processing in-memory data
	// structures without requiring files
	m, err := loadManifestFromPath("testdata/11ac8128320000/manifest.yaml")
	ftt.Run("Given a telemetry experiment manifest with known significant differences", t, func(t *ftt.Test) {
		assert.Loosely(t, err, should.BeNil)
		t.Run("When we analyze the artifacts", func(t *ftt.Test) {
			rootDir, err := filepath.Abs("testdata/11ac8128320000")
			assert.Loosely(t, err, should.BeNil)
			r, err := analyzeExperiment(m, rootDir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r, should.NotBeNil)
			assert.Loosely(t, len(r.Reports), should.Equal(2))
			t.Run("Then we verify the overall p-value", func(t *ftt.Test) {
				assert.Loosely(t, r.OverallPValue, should.AlmostEqual(0.0053, 0.0001))
				t.Run("And we verify the p-values of the individual metrics", func(t *ftt.Test) {
					opt_metric := r.Reports["Optimize-Background:count"]
					assert.Loosely(t, *opt_metric.PValue, should.AlmostEqual(0.0026, 0.0001))
					parse_metric := r.Reports["Parse-Background:count"]
					assert.Loosely(t, *parse_metric.PValue, should.AlmostEqual(0.914, 0.001))
				})
			})
		})
		t.Run("When we use the mixin to analyze the artifacts", func(t *ftt.Test) {
			m := &analyzeExperimentMixin{analyzeExperiment: true}
			ctx := context.Background()
			// This is the minimal Job definition that's associated with the
			// manifest/artifacts in the test data.
			j := &proto.Job{
				Name: "jobs/legacy-11ac8128320000",
				JobSpec: &proto.JobSpec{
					JobKind: &proto.JobSpec_Experiment{
						Experiment: &proto.Experiment{},
					},
					Arguments: &proto.JobSpec_TelemetryBenchmark{
						TelemetryBenchmark: &proto.TelemetryBenchmark{},
					},
				},
			}
			wd, err := filepath.Abs("testdata")
			assert.Loosely(t, err, should.BeNil)

			r, err := m.doAnalyzeExperiment(ctx, wd, j)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r, should.NotBeNil)
			assert.Loosely(t, r.OverallPValue, should.NotEqual(0.0))
			assert.Loosely(t, r.Reports, should.NotBeNil)
		})
	})

	ftt.Run("Report serializes to JSON", t, func(t *ftt.Test) {
		m, err := loadManifestFromPath("testdata/11ac8128320000/manifest.yaml")
		assert.Loosely(t, err, should.BeNil)
		t.Run("When we analyze the artifacts", func(t *ftt.Test) {
			rootDir, err := filepath.Abs("testdata/11ac8128320000")
			assert.Loosely(t, err, should.BeNil)
			r, err := analyzeExperiment(m, rootDir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r, should.NotBeNil)
			t.Run("The report struct should encode to JSON without errors", func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				enc := json.NewEncoder(buf)
				err := enc.Encode(r)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(buf.Bytes()), should.BeGreaterThan(0))
			})
		})

	})

}
