// Copyright 2021 The Chromium Authors.
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

package cli

import (
	"context"
	"os"
	"testing"

	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestLoadPresets(t *testing.T) {
	t.Parallel()

	ftt.Run("Given a simple presets file", t, func(t *ftt.Test) {
		sp, err := os.Open("testdata/simple-presets.yaml")
		assert.Loosely(t, err, should.BeNil)
		defer sp.Close()
		t.Run("When we load the presets", func(t *ftt.Test) {
			pd, err := loadPresets(sp)
			assert.Loosely(t, err, should.BeNil)
			t.Run("Then we can find the \"basic\" preset", func(t *ftt.Test) {
				_, err := pd.GetPreset("basic")
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("And we can find the \"complex\" preset", func(t *ftt.Test) {
				_, err := pd.GetPreset("complex")
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("And we can find the \"summary_report\" preset", func(t *ftt.Test) {
				_, err := pd.GetPreset("summary_report")
				assert.Loosely(t, err, should.BeNil)
			})

			t.Run("And the \"basic\" and \"complex\" presets only differ with extra args", func(t *ftt.Test) {
				b, err := pd.GetPreset("basic")
				assert.Loosely(t, err, should.BeNil)
				c, err := pd.GetPreset("complex")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, b.TelemetryExperiment.ExtraArgs, should.NotMatch(c.TelemetryExperiment.ExtraArgs))
				assert.Loosely(t, b.TelemetryExperiment.Benchmark, should.Equal(c.TelemetryExperiment.Benchmark))
			})

		})
	})

	ftt.Run("Given an invalid presets file", t, func(t *ftt.Test) {
		sp, err := os.Open("testdata/invalid-presets.yaml")
		assert.Loosely(t, err, should.BeNil)
		defer sp.Close()
		t.Run("When we load the presets", func(t *ftt.Test) {
			pd, err := loadPresets(sp)
			assert.Loosely(t, err, should.BeNil)
			t.Run("Then looking up a preset with invalid story selection fails", func(t *ftt.Test) {
				_, err := pd.GetPreset("conflicting-story-selection")
				expected := text.Doc(`
					telemetry experiments must only have exactly one of story
					or story_tags in story_selection
				`)
				assert.Loosely(t, err, should.ErrLike(expected))
				_, err = pd.GetPreset("empty-story-selection")
				assert.Loosely(t, err, should.ErrLike(expected))
				_, err = pd.GetPreset("batch-empty-stories-and-story-tags")
				assert.Loosely(t, err, should.ErrLike("at least one story or story tag should be defined for each benchmark"))
			})
			t.Run("And looking up a preset with no config fails", func(t *ftt.Test) {
				_, err := pd.GetPreset("empty-config")
				assert.Loosely(t, err, should.ErrLike("telemetry experiments must have a non-empty config"))
				_, err = pd.GetPreset("batch-empty-configs")
				assert.Loosely(t, err, should.ErrLike("at least one config should be defined for each benchmark"))
			})
		})
	})

	ftt.Run("Given an unparseable yaml file", t, func(t *ftt.Test) {
		pm := presetsMixin{
			presetFile: "testdata/invalid-yaml.yaml",
			presetName: "preset",
		}
		_, err := pm.getPreset(context.Background())
		assert.Loosely(t, err, should.NotBeNil)
	})

}
