// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestSrcConfig(t *testing.T) {
	t.Parallel()

	var testSrcConfig = make(map[string]SrcConfig)
	testSrcConfig["project"] = SrcConfig{
		DefaultSpecs: []ProblemSpec{
			{
				Name:       "Unhealthy",
				PeriodDays: 1,
				Score:      UNHEALTHY_SCORE,
				Thresholds: Thresholds{
					TestPendingTime: PercentileThresholds{P50Mins: 60, P95Mins: 120},
					PendingTime:     PercentileThresholds{P50Mins: 60, P95Mins: 120},
					BuildTime:       PercentileThresholds{P50Mins: 60, P95Mins: 120},
					FailRate:        AverageThresholds{Average: 0.2},
					InfraFailRate:   AverageThresholds{Average: 0.1},
				},
			},
			{
				Name:       "Low Value",
				PeriodDays: 1,
				Score:      LOW_VALUE_SCORE,
				Thresholds: Thresholds{
					FailRate:      AverageThresholds{Average: 0.99},
					InfraFailRate: AverageThresholds{Average: 0.99},
				},
			},
		},
		BucketSpecs: map[string]BuilderSpecs{
			"bucket": {
				"builder": BuilderSpec{
					ProblemSpecs: []ProblemSpec{
						{
							Name:       "Unhealthy",
							PeriodDays: 1,
							Score:      UNHEALTHY_SCORE,
							Thresholds: Thresholds{
								Default: "_default",
							},
						},
						{
							Name:       "Low Value",
							PeriodDays: 1,
							Score:      LOW_VALUE_SCORE,
							Thresholds: Thresholds{
								Default: "_default",
							},
						},
					},
				},
			},
			"slow-bucket": {
				"slow-builder": BuilderSpec{
					ProblemSpecs: []ProblemSpec{
						{
							Name:       "Low Value",
							PeriodDays: 1,
							Score:      LOW_VALUE_SCORE,
							Thresholds: Thresholds{
								Default: "_default",
							},
						},
						{
							Name:       "Unhealthy",
							PeriodDays: 1,
							Score:      UNHEALTHY_SCORE,
							Thresholds: Thresholds{
								TestPendingTime: PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								PendingTime:     PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								BuildTime:       PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								FailRate:        AverageThresholds{Average: 0.4},
								InfraFailRate:   AverageThresholds{Average: 0.3},
							},
						},
					},
				},
			},
			"custom-bucket": {
				"custom-builder": BuilderSpec{
					ProblemSpecs: []ProblemSpec{
						{
							Name:       "Unhealthy",
							PeriodDays: 1,
							Score:      UNHEALTHY_SCORE,
							Thresholds: Thresholds{
								TestPendingTime: PercentileThresholds{P50Mins: 60, P95Mins: 120},
								PendingTime:     PercentileThresholds{P50Mins: 60, P95Mins: 120},
								BuildTime:       PercentileThresholds{P50Mins: 60, P95Mins: 120},
								FailRate:        AverageThresholds{Average: 0.4},
								InfraFailRate:   AverageThresholds{Average: 0.3},
							},
						},
						{
							Name:       "Low Value",
							PeriodDays: 1,
							Score:      LOW_VALUE_SCORE,
							Thresholds: Thresholds{
								TestPendingTime: PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								PendingTime:     PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								BuildTime:       PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								FailRate:        AverageThresholds{Average: 0.99},
								InfraFailRate:   AverageThresholds{Average: 0.99},
							},
						},
					},
				},
			},
			"improper-bucket": {
				"improper-builder": BuilderSpec{
					ProblemSpecs: []ProblemSpec{
						{
							Name:       "Unhealthy",
							PeriodDays: 1,
							Score:      UNHEALTHY_SCORE,
							Thresholds: Thresholds{
								Default:         "_default",
								TestPendingTime: PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								PendingTime:     PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								BuildTime:       PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								FailRate:        AverageThresholds{Average: 0.4},
								InfraFailRate:   AverageThresholds{Average: 0.3},
							},
						},
					},
				},
				"improper-builder2": BuilderSpec{
					ProblemSpecs: []ProblemSpec{
						{
							Name:       "Unhealthy",
							PeriodDays: 1,
							Score:      UNHEALTHY_SCORE,
							Thresholds: Thresholds{
								Default:         "not_default",
								TestPendingTime: PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								PendingTime:     PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								BuildTime:       PercentileThresholds{P50Mins: 600, P95Mins: 1200},
								FailRate:        AverageThresholds{Average: 0.4},
								InfraFailRate:   AverageThresholds{Average: 0.3},
							},
						},
					},
				},
				"improper-builder3": BuilderSpec{
					ProblemSpecs: []ProblemSpec{},
				},
			},
		},
	}

	ftt.Run("Healthy builder is healthy, default thresholds", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "bucket",
			Builder: "builder",
			Metrics: []*Metric{
				{Type: "build_mins_p50", Value: 59},
				{Type: "build_mins_p95", Value: 119},
				{Type: "pending_mins_p50", Value: 59},
				{Type: "pending_mins_p95", Value: 119},
				{Type: "fail_rate", Value: 0.05},
				{Type: "infra_fail_rate", Value: 0},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(HEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.BeEmpty)
	})
	ftt.Run("P50 percentile above threshold, default thresholds", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "bucket",
			Builder: "builder",
			Metrics: []*Metric{
				{Type: "build_mins_p50", Value: 61},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))

		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("build_mins_p50"))
	})
	ftt.Run("P95 percentile above thresholds, default thresholds", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "bucket",
			Builder: "builder",
			Metrics: []*Metric{
				{Type: "pending_mins_p95", Value: 121},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("pending_mins_p95"))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Fail rate above thresholds, default thresholds", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "bucket",
			Builder: "builder",
			Metrics: []*Metric{
				{Type: "fail_rate", Value: 0.3},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("fail_rate"))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("P50 build time below thresholds, slow builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "slow-bucket",
			Builder: "slow-builder",
			Metrics: []*Metric{
				{Type: "build_mins_p50", Value: 200},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(HEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.BeEmpty)
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Infra fail rate above thresholds, slow builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "slow-bucket",
			Builder: "slow-builder",
			Metrics: []*Metric{
				{Type: "infra_fail_rate", Value: 0.5},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("infra_fail_rate"))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Default thresholds with custom thresholds error", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "slow-bucket",
			Builder: "slow-builder",
			Metrics: []*Metric{
				{Type: "infra_fail_rate", Value: 0.5},
			},
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("infra_fail_rate"))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Multiple healthy builders", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{
			{
				Project: "project",
				Bucket:  "bucket",
				Builder: "builder",
				Metrics: []*Metric{
					{Type: "build_mins_p50", Value: 59},
					{Type: "build_mins_p95", Value: 119},
					{Type: "pending_mins_p50", Value: 59},
					{Type: "pending_mins_p95", Value: 119},
					{Type: "fail_rate", Value: 0.05},
					{Type: "infra_fail_rate", Value: 0},
				},
			},
			{
				Project: "project",
				Bucket:  "slow-bucket",
				Builder: "slow-builder",
				Metrics: []*Metric{
					{Type: "build_mins_p50", Value: 59},
					{Type: "build_mins_p95", Value: 119},
					{Type: "pending_mins_p50", Value: 59},
					{Type: "pending_mins_p95", Value: 119},
					{Type: "fail_rate", Value: 0.05},
					{Type: "infra_fail_rate", Value: 0},
				},
			},
		}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(2))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(HEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.BeEmpty)
		assert.Loosely(t, outputRows[1].HealthScore, should.Equal(HEALTHY_SCORE))
		explanation = scoreExplanation(outputRows[1], savedThresholds)
		assert.Loosely(t, explanation, should.BeEmpty)
	})
	ftt.Run("One healthy, one unhealthy builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{
			{
				Project: "project",
				Bucket:  "bucket",
				Builder: "builder",
				Metrics: []*Metric{
					{Type: "build_mins_p50", Value: 61},
					{Type: "build_mins_p95", Value: 121},
					{Type: "pending_mins_p50", Value: 61},
					{Type: "pending_mins_p95", Value: 121},
					{Type: "fail_rate", Value: 0.3},
					{Type: "infra_fail_rate", Value: 0.2},
				},
			},
			{
				Project: "project",
				Bucket:  "slow-bucket",
				Builder: "slow-builder",
				Metrics: []*Metric{
					{Type: "build_mins_p50", Value: 59},
					{Type: "build_mins_p95", Value: 119},
					{Type: "pending_mins_p50", Value: 59},
					{Type: "pending_mins_p95", Value: 119},
					{Type: "fail_rate", Value: 0.05},
					{Type: "infra_fail_rate", Value: 0},
				},
			},
		}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(2))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("build_mins"))
		assert.Loosely(t, explanation, should.ContainSubstring("infra_fail_rate"))
		assert.Loosely(t, outputRows[1].HealthScore, should.Equal(HEALTHY_SCORE))
		explanation = scoreExplanation(outputRows[1], savedThresholds)
		assert.Loosely(t, explanation, should.BeEmpty)
	})
	ftt.Run("One low value, one unhealthy", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{
			{
				Project: "project",
				Bucket:  "bucket",
				Builder: "builder",
				Metrics: []*Metric{
					{Type: "build_mins_p50", Value: 61},
					{Type: "build_mins_p95", Value: 121},
					{Type: "pending_mins_p50", Value: 61},
					{Type: "pending_mins_p95", Value: 121},
					{Type: "fail_rate", Value: 1.0},
					{Type: "infra_fail_rate", Value: 1.0},
				},
			},
			{
				Project: "project",
				Bucket:  "slow-bucket",
				Builder: "slow-builder",
				Metrics: []*Metric{
					{Type: "build_mins_p50", Value: 61},
					{Type: "build_mins_p95", Value: 121},
					{Type: "pending_mins_p50", Value: 61},
					{Type: "pending_mins_p95", Value: 121},
					{Type: "fail_rate", Value: 0.90},
					{Type: "infra_fail_rate", Value: 0.90},
				},
			},
		}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(2))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(LOW_VALUE_SCORE))
		explanation := scoreExplanation(outputRows[0], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("fail_rate"))
		assert.Loosely(t, outputRows[1].HealthScore, should.Equal(UNHEALTHY_SCORE))
		explanation = scoreExplanation(outputRows[1], savedThresholds)
		assert.Loosely(t, explanation, should.ContainSubstring("fail_rate"))
	})
	ftt.Run("Improper threshold config, both default and custom thresholds", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "improper-bucket",
			Builder: "improper-builder",
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].ScoreExplanation, should.ContainSubstring("default"))
		assert.Loosely(t, outputRows[0].ScoreExplanation, should.ContainSubstring("custom"))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNSET_SCORE))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Improper threshold config, Default set to unknown sentinel value", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "improper-bucket",
			Builder: "improper-builder2",
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].ScoreExplanation, should.ContainSubstring("unknown sentinel"))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNSET_SCORE))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Improper ProblemSpecs, no ProblemSpecs", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "improper-bucket",
			Builder: "improper-builder3",
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].ScoreExplanation, should.ContainSubstring("no ProblemSpecs"))
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNSET_SCORE))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Unconfigured builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rows := []Row{{
			Project: "project",
			Bucket:  "unconfigured-bucket",
			Builder: "unconfigured-builder",
		}}
		savedThresholds := testSrcConfig
		outputRows, err := calculateIntermediateHealthScores(ctx, rows, testSrcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(outputRows), should.Equal(1))
		assert.Loosely(t, outputRows[0].ScoreExplanation, should.BeBlank)
		assert.Loosely(t, outputRows[0].HealthScore, should.Equal(UNSET_SCORE))
		assert.Loosely(t, savedThresholds, should.Match(testSrcConfig))
	})
	ftt.Run("Sort ProblemSpecs", t, func(t *ftt.Test) {
		ps := []ProblemSpec{
			{
				Name:  "Low Value",
				Score: 1,
				Thresholds: Thresholds{
					Default: "Low Value",
				},
			},
			{
				Name:  "Unhealthy",
				Score: UNHEALTHY_SCORE,
				Thresholds: Thresholds{
					Default: "Unhealthy",
				},
			},
		}
		sortProblemSpecs(ps)
		assert.Loosely(t, ps[0].Score, should.Equal(UNHEALTHY_SCORE))
	})
	ftt.Run("Compare Thresholds Helper", t, func(t *ftt.Test) {
		ctx := context.Background()

		const unhealthyIndex = 0
		const lowValueIndex = 1
		row := Row{
			Project: "project",
			Bucket:  "custom-bucket",
			Builder: "custom-builder",
			Metrics: []*Metric{
				{Type: "fail_rate", Value: 1.0},
			},
		}
		ps := testSrcConfig["project"].BucketSpecs[row.Bucket][row.Builder].ProblemSpecs

		compareThresholds(ctx, &row, &ps[unhealthyIndex])
		assert.Loosely(t, row.HealthScore, should.Equal(UNHEALTHY_SCORE))

		compareThresholds(ctx, &row, &ps[lowValueIndex])
		assert.Loosely(t, row.HealthScore, should.Equal(LOW_VALUE_SCORE))
	})
}
