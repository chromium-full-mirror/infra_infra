// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"testing"

	findingspb "go.chromium.org/luci/common/proto/findings"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func analyzeJSONTestFile(t testing.TB, filePath string) []*findingspb.Finding {
	// Mock current time for testing
	inputPath := filepath.Join(inputDir, filePath)
	f := openFileOrDie(inputPath)
	defer closeFileOrDie(f)
	return analyzeFieldTrialTestingConfig(f, filePath)
}

func TestConfigCheck(t *testing.T) {
	ftt.Run("Analyze Config JSON file with no errors: one experiment", t, func(t *ftt.Test) {
		results := analyzeJSONTestFile(t, "configs/one_experiment.json")
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze Config JSON file with no errors: many configs one experiment", t, func(t *ftt.Test) {
		results := analyzeJSONTestFile(t, "configs/many_configs_one_exp.json")
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze Config JSON file with warning: many experiments", t, func(t *ftt.Test) {
		results := analyzeJSONTestFile(t, "configs/many_experiments.json")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(manyExperimentsFinding.message, "TestConfig1"),
				SeverityLevel: manyExperimentsFinding.level,
				Location: &findingspb.Location{
					FilePath: "configs/many_experiments.json",
					Range: &findingspb.Location_Range{
						StartLine: 7,
						EndLine:   7,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze Config JSON file with two warnings: many configs many experiments", t, func(t *ftt.Test) {
		results := analyzeJSONTestFile(t, "configs/many_configs_many_exp.json")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(manyExperimentsFinding.message, "TestConfig1"),
				SeverityLevel: manyExperimentsFinding.level,
				Location: &findingspb.Location{
					FilePath: "configs/many_configs_many_exp.json",
					Range: &findingspb.Location_Range{
						StartLine: 7,
						EndLine:   7,
					},
				},
			},
			{
				Category:      category,
				Message:       fmt.Sprintf(manyExperimentsFinding.message, "TestConfig1"),
				SeverityLevel: manyExperimentsFinding.level,
				Location: &findingspb.Location{
					FilePath: "configs/many_configs_many_exp.json",
					Range: &findingspb.Location_Range{
						StartLine: 26,
						EndLine:   26,
					},
				},
			},
		}))
	})
}
