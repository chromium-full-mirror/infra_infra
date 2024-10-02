// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/libs/git"
)

type SrcConfig struct {
	DefaultSpecs []ProblemSpec           `json:"_default_specs"`
	BucketSpecs  map[string]BuilderSpecs `json:"specs"` // e.g. ci -> {}
}
type BuilderSpecs map[string]BuilderSpec // e.g. linux-rel -> {}
type BuilderSpec struct {
	ContactTeamEmail string        `json:"contact_team_email"`
	ProblemSpecs     []ProblemSpec `json:"problem_specs"` // e.g. UNHEALTHY -> {}
}
type ProblemSpec struct {
	Name       string     `json:"name"` // This name will be shown in Milo when a builder is affected by this Problem.
	Score      int        `json:"score"`
	PeriodDays int        `json:"period_days"`
	Thresholds Thresholds `json:"thresholds"`
}
type Thresholds struct {
	Default         string               `json:"_default"` // if set to the sentinel value "_default", then use the defaults
	BuildTime       PercentileThresholds `json:"build_time"`
	FailRate        AverageThresholds    `json:"fail_rate"`
	InfraFailRate   AverageThresholds    `json:"infra_fail_rate"`
	PendingTime     PercentileThresholds `json:"pending_time"`
	TestPendingTime PercentileThresholds `json:"test_pending_time"`
}

type PercentileThresholds struct {
	P50Mins float32 `json:"p50_mins"`
	P95Mins float32 `json:"p95_mins"`
	P99Mins float32 `json:"p99_mins"`
}

type AverageThresholds struct {
	Average float32 `json:"average"`
}

const HEALTHY_SCORE = 10
const UNHEALTHY_SCORE = 5
const LOW_VALUE_SCORE = 1
const UNSET_SCORE = 0

const UNSET_THRESHOLD = float32(0)

func getSrcConfig(buildCtx context.Context, gerritHost string, repoHost string, repoName string) (*SrcConfig, error) {
	var err error
	step, ctx := build.StartStep(buildCtx, "Get Src Config")
	defer func() { step.End(err) }()

	step.SetSummaryMarkdown(fmt.Sprintf("Reading src config from https://%s/%s/+/refs/heads/main/infra/config/generated/health-specs/health-specs.json", repoHost, repoName))

	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{Scopes: []string{gitiles.OAuthScope}})
	httpClient, err := authenticator.Client()
	if err != nil {
		step.SetSummaryMarkdown("Error in Initializing Auth")
		return nil, errors.Annotate(err, "Initializing Auth").Err()
	}

	client, err := git.NewClient(ctx, httpClient, gerritHost, repoHost, repoName, "main")
	if err != nil {
		step.SetSummaryMarkdown("Error in Initializing Gitiles client")
		return nil, errors.Annotate(err, "Initializing Gitiles client").Err()
	}

	srcConfigString, err := client.GetFile(ctx, "infra/config/generated/health-specs/health-specs.json")
	if err != nil {
		step.SetSummaryMarkdown("Error in Downloading src config")
		return nil, errors.Annotate(err, "Downloading src config").Err()
	}

	var srcConfig SrcConfig
	err = json.Unmarshal([]byte(srcConfigString), &srcConfig)
	if err != nil {
		step.SetSummaryMarkdown("Error in Unmarshalling src config")
		return nil, errors.Annotate(err, "Unmarshalling src config").Err()
	}

	return &srcConfig, nil
}

func compareThresholds(ctx context.Context, row *Row, problemSpec *ProblemSpec) error {
	if row.HealthScore == UNSET_SCORE {
		row.HealthScore = HEALTHY_SCORE
	}
	// TODO: make metric.Threshold a list, right now it just takes the lowest problem spec score threshold
	var stepErr error
	for _, metric := range row.Metrics {
		threshold := float32(UNSET_THRESHOLD)
		switch metric.Type {
		case "build_mins_p50":
			threshold = problemSpec.Thresholds.BuildTime.P50Mins
		case "build_mins_p95":
			threshold = problemSpec.Thresholds.BuildTime.P95Mins
		case "fail_rate":
			threshold = problemSpec.Thresholds.FailRate.Average
		case "infra_fail_rate":
			threshold = problemSpec.Thresholds.InfraFailRate.Average
		case "pending_mins_p50":
			threshold = problemSpec.Thresholds.PendingTime.P50Mins
		case "pending_mins_p95":
			threshold = problemSpec.Thresholds.PendingTime.P95Mins
		// TODO: add checks for Test Pending Time once the data is added to the DB query
		default:
			metric.HealthScore = UNSET_SCORE
			err := fmt.Errorf("Found unknown metric type %s in BigQuery", metric.Type)

			// Log all, return just the last
			logging.Errorf(ctx, "%s", err)
			stepErr = err
			continue
		}
		compareThresholdsHelper(row, problemSpec, metric, threshold)

		if metric.HealthScore != UNSET_SCORE && metric.HealthScore < row.HealthScore {
			row.HealthScore = metric.HealthScore
		}
	}

	return stepErr
}

func compareThresholdsHelper(row *Row, problemSpec *ProblemSpec, metric *Metric, threshold float32) {
	if threshold == UNSET_THRESHOLD {
		return
	}

	if metric.HealthScore == UNSET_SCORE {
		metric.HealthScore = HEALTHY_SCORE
	}
	metric.Threshold = threshold
	if metric.Value > threshold {
		metric.HealthScore = problemSpec.Score
	}
}

// Used for problem precedence
func sortProblemSpecs(problemSpecs []ProblemSpec) {
	sort.Slice(problemSpecs, func(i, j int) bool {
		return problemSpecs[i].Score > problemSpecs[j].Score
	})
}
