// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

func analyzeJSONTestFile(t testing.TB, filePath string) []*tricium.Data_Comment {
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
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:  category + "/Experiments",
				Message:   fmt.Sprintf(manyExperimentsWarning, "TestConfig1"),
				StartLine: 7,
				Path:      "configs/many_experiments.json",
			},
		}))
	})

	ftt.Run("Analyze Config JSON file with two warnings: many configs many experiments", t, func(t *ftt.Test) {
		results := analyzeJSONTestFile(t, "configs/many_configs_many_exp.json")
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:  category + "/Experiments",
				Message:   fmt.Sprintf(manyExperimentsWarning, "TestConfig1"),
				StartLine: 7,
				Path:      "configs/many_configs_many_exp.json",
			},
			{
				Category:  category + "/Experiments",
				Message:   fmt.Sprintf(manyExperimentsWarning, "TestConfig1"),
				StartLine: 26,
				Path:      "configs/many_configs_many_exp.json",
			},
		}))
	})
}
