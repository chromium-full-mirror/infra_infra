// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"

	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cr_builder_health/healthpb"
)

type Row struct {
	HealthScore      int        `bigquery:"health_score"`
	ScoreExplanation string     `bigquery:"score_explanation"`
	Date             civil.Date `bigquery:"date"`
	Project          string     `bigquery:"project"`
	Bucket           string     `bigquery:"bucket"`
	Builder          string     `bigquery:"builder"`
	Rotation         string     `bigquery:"rotation"`
	ContactTeamEmail string     `bigquery:"contact_team_email"`
	N                int        `bigquery:"n"`
	Metrics          []*Metric  `bigquery:"metrics"`
}

type Metric struct {
	Type        string  `bigquery:"type"`
	Value       float32 `bigquery:"value"`
	Threshold   float32 `bigquery:"threshold"`
	HealthScore int     `bigquery:"health_score"`
}

type BBClient interface {
	SetBuilderHealth(ctx context.Context, in *buildbucketpb.SetBuilderHealthRequest, opts ...grpc.CallOption) (*buildbucketpb.SetBuilderHealthResponse, error)
}

func generate(ctx context.Context, input *healthpb.InputParams) error {
	bqClient, err := setup(ctx, input)
	if err != nil {
		return errors.Annotate(err, "Setup").Err()
	}
	defer bqClient.Close()

	srcConfigs := make(map[string]SrcConfig)
	chromiumSrcConfig, err := getSrcConfig(ctx, "chromium-review.googlesource.com", "chromium.googlesource.com", "chromium/src")
	if err == nil {
		srcConfigs["chromium"] = *chromiumSrcConfig
	}

	chromeSrcConfig, err := getSrcConfig(ctx, "chrome-internal-review.googlesource.com", "chrome-internal.googlesource.com", "chrome/src-internal")
	if err == nil {
		srcConfigs["chrome"] = *chromeSrcConfig
	}

	if len(srcConfigs) == 0 {
		return errors.Fmt("Get Src Config: %w", err)
	}

	rows, err := getMetrics(ctx, bqClient, input)
	if err != nil {
		return errors.Annotate(err, "Get metrics").Err()
	}

	rowsWithHealthScores, err := calculateIntermediateHealthScores(ctx, rows, srcConfigs)
	if err != nil {
		return errors.Annotate(err, "Calculate intermediate health scores").Err()
	}

	rowsWithIndicators, err := calculateIndicators(ctx, input, rowsWithHealthScores, srcConfigs)
	if err != nil {
		return errors.Annotate(err, "Calculate indicators").Err()
	}

	err = logIndicators(ctx, rowsWithIndicators)
	if err != nil {
		return errors.Annotate(err, "Log indicators").Err()
	}

	if input.DryRun {
		return nil
	}

	client, err := bbClient(ctx)
	if err != nil {
		return errors.Annotate(err, "Make BB client").Err()
	}

	err = rpcBuildbucket(ctx, rowsWithIndicators, client)
	if err != nil {
		return errors.Annotate(err, "RPC buildbucket").Err()
	}

	// Write out to BQ
	if err = writeIndicators(ctx, bqClient, rowsWithIndicators); err != nil {
		return errors.Annotate(err, "Write indicators").Err()
	}

	return nil
}

func setup(buildCtx context.Context, input *healthpb.InputParams) (*bigquery.Client, error) {
	var err error
	step, _ := build.StartStep(buildCtx, "Setup")
	defer func() { step.End(err) }()

	step.SetSummaryMarkdown(fmt.Sprintf("Date is %s", input.Date.AsTime().String()))

	bqClient, err := bigquery.NewClient(buildCtx, "cr-builder-health-indicators")
	if err != nil {
		return nil, errors.Annotate(err, "Initializing BigQuery client").Err()
	}

	return bqClient, nil
}

func getMetrics(buildCtx context.Context, bqClient *bigquery.Client, input *healthpb.InputParams) ([]Row, error) {
	var err error
	step, ctx := build.StartStep(buildCtx, "Get metrics")
	defer func() { step.End(err) }()

	q := bqClient.Query(`
	SELECT
	  DATE(b.create_time) as date,
	  b.builder.project,
	  b.builder.bucket,
	  b.builder.builder,
	  IFNULL(JSON_VALUE_ARRAY(ANY_VALUE(b.input.properties), '$.sheriff_rotations')[OFFSET(0)], '') as rotation,
	  COUNT(*) as n,
	  [
		STRUCT('fail_rate' as type, COUNTIF(b.status = 'FAILURE') / COUNT(*) as value),
		STRUCT('infra_fail_rate' as type, COUNTIF(b.status = 'INFRA_FAILURE') / COUNT(*) as value),
		STRUCT('pending_mins_p50' as type, IFNULL(APPROX_QUANTILES(TIMESTAMP_DIFF(start_time, create_time, SECOND), 100)[OFFSET(50)]/60, 0) as value),
		STRUCT('pending_mins_p95' as type, IFNULL(APPROX_QUANTILES(TIMESTAMP_DIFF(start_time, create_time, SECOND), 100)[OFFSET(95)]/60, 0) as value),
		STRUCT('build_mins_p50' as type, IFNULL(APPROX_QUANTILES(TIMESTAMP_DIFF(end_time, start_time, SECOND), 100)[OFFSET(50)]/60, 0) as value),
		STRUCT('build_mins_p95' as type, IFNULL(APPROX_QUANTILES(TIMESTAMP_DIFF(end_time, start_time, SECOND), 100)[OFFSET(95)]/60, 0) as value)
	  ] as metrics
    ` +
		"FROM `cr-buildbucket.chrome.builds` as b" + `
	WHERE
		b.create_time < @input_date
		AND b.create_time >= TIMESTAMP_SUB(@input_date, INTERVAL 90 DAY)
		AND b.builder.bucket = 'ci'
		AND b.builder.project IN ('chromium', 'chrome')
	GROUP BY
	  b.builder.project, b.builder.bucket, b.builder.builder, date
	ORDER BY
	  rotation,
	  LOWER(builder) ASC,
	  date DESC
	`)
	q.Parameters = []bigquery.QueryParameter{
		{
			Name:  "input_date",
			Value: input.Date.AsTime().UTC().Format(iso8601Format),
		},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "BQ query").Err()
	}

	var rows []Row

	for {
		var row Row
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			logging.Errorf(ctx, "Partially parsed bad row: %+v", row)
			return nil, errors.Annotate(err, "Row iterator").Err()
		}
		logging.Debugf(ctx, "%+v", row)
		rows = append(rows, row)
	}

	step.SetSummaryMarkdown(fmt.Sprintf("Queried %d builders", len(rows)))

	return rows, nil
}

func calculateIntermediateHealthScores(buildCtx context.Context, rows []Row, srcConfigs map[string]SrcConfig) ([]Row, error) {
	var stepErr error
	step, ctx := build.StartStep(buildCtx, "Calculate intermediate health scores")
	defer func() { step.End(stepErr) }()

	// Log an error only once when the src config is missing for a builder
	srcConfigNotFound := make(map[string]bool)

	failedBuilders := 0
	for i, row := range rows {
		if srcConfig, ok := srcConfigs[row.Project]; !ok {
			rows[i].HealthScore = UNSET_SCORE
			if !srcConfigNotFound[builderID(row.Project, "", "")] {
				logging.Errorf(ctx, "Src Config not found for project: %s", row.Project)
				srcConfigNotFound[builderID(row.Project, "", "")] = true
			}
			continue
		} else if bucketSpec, ok := srcConfig.BucketSpecs[row.Bucket]; !ok {
			rows[i].HealthScore = UNSET_SCORE
			if !srcConfigNotFound[builderID(row.Project, row.Bucket, "")] {
				logging.Errorf(ctx, "Src Config not found for project: %s, bucket: %s", row.Project, row.Bucket)
				srcConfigNotFound[builderID(row.Project, row.Bucket, "")] = true
			}
			continue
		} else if builderSpec, ok := bucketSpec[row.Builder]; !ok {
			rows[i].HealthScore = UNSET_SCORE
			if !srcConfigNotFound[builderID(row.Project, row.Bucket, row.Builder)] {
				logging.Errorf(ctx, "Src Config not found for project: %s, bucket: %s, builder: %s", row.Project, row.Bucket, row.Builder)
				srcConfigNotFound[builderID(row.Project, row.Bucket, row.Builder)] = true
			}
			continue
		} else {
			if len(builderSpec.ProblemSpecs) == 0 {
				rows[i].HealthScore = UNSET_SCORE
				rows[i].ScoreExplanation = fmt.Sprintf("Src Config error: Bucket: %s, Builder: %s has no ProblemSpecs", row.Bucket, row.Builder)
				logging.Errorf(ctx, "Src Config error: Bucket: %s, Builder: %s has no ProblemSpecs", row.Bucket, row.Builder)
				failedBuilders += 1
				continue
			}

			sortProblemSpecs(builderSpec.ProblemSpecs) // to give lower scores precedence
			for _, problemSpec := range builderSpec.ProblemSpecs {
				if problemSpec.Thresholds.Default == "_default" {
					// default set, check if any other thresholds erroneously specified
					if (problemSpec.Thresholds.BuildTime != PercentileThresholds{} ||
						problemSpec.Thresholds.TestPendingTime != PercentileThresholds{} ||
						problemSpec.Thresholds.PendingTime != PercentileThresholds{} ||
						problemSpec.Thresholds.FailRate != AverageThresholds{} ||
						problemSpec.Thresholds.InfraFailRate != AverageThresholds{}) {
						rows[i].HealthScore = UNSET_SCORE
						rows[i].ScoreExplanation = "Threshold config error: default sentinel and custom thresholds cannot both be set."
						logging.Errorf(ctx, "%s Bucket: %s. Builder: %s.", rows[i].ScoreExplanation, row.Bucket, row.Builder)
						failedBuilders += 1
						continue
					}

					// _default set, look for a matching default spec
					found := false
					for _, defaultSpec := range srcConfig.DefaultSpecs {
						if defaultSpec.Name == problemSpec.Name {
							problemSpec.Thresholds = defaultSpec.Thresholds
							found = true
							break
						}
					}
					if !found {
						rows[i].HealthScore = UNSET_SCORE
						rows[i].ScoreExplanation = "Threshold config error: default sentinel but no matching default found"
						logging.Errorf(ctx, "%s Bucket: %s. Builder: %s.", rows[i].ScoreExplanation, row.Bucket, row.Builder)
						failedBuilders += 1
						continue
					}
				} else if problemSpec.Thresholds.Default != "" {
					// Unknown sentinel
					rows[i].HealthScore = UNSET_SCORE
					rows[i].ScoreExplanation = fmt.Sprintf("Threshold config error: Default set to unknown sentinel value: %s.", problemSpec.Thresholds.Default)
					logging.Errorf(ctx, "%s Bucket: %s. Builder %s.", rows[i].ScoreExplanation, row.Bucket, row.Builder)
					failedBuilders += 1
					continue
				}
				// Happy path, compare thresholds
				stepErr = errors.Join(stepErr, compareThresholds(ctx, &rows[i], &problemSpec))
			}
			rows[i].ContactTeamEmail = builderSpec.ContactTeamEmail
		}
	}

	if failedBuilders > 0 {
		stepErr = errors.Join(stepErr, fmt.Errorf("Indicator calculation failed for %d builders", failedBuilders))
	}

	return rows, stepErr
}

func builderID(project string, bucket string, builder string) string {
	return project + "/" + bucket + "/" + builder
}

func isWeekend(date civil.Date) bool {
	time, _ := time.Parse(time.RFC3339, date.String()+"T00:00:00Z")
	return time.Weekday() == 0 || time.Weekday() == 6
}

func calcBusinessDays(from time.Time, to time.Time) int {
	days := 0
	for {
		if from.Equal(to) || from.After(to) {
			return days
		}
		if from.Weekday() != time.Saturday && from.Weekday() != time.Sunday {
			days++
		}
		from = from.Add(time.Hour * 24)
	}
}

func calculateIndicators(buildCtx context.Context, input *healthpb.InputParams, rows []Row, srcConfigs map[string]SrcConfig) ([]Row, error) {
	var stepErr error
	step, ctx := build.StartStep(buildCtx, "Calculate indicators")
	defer func() { step.End(stepErr) }()

	type BuilderHealth struct {
		MostRecentRow Row
		Scores        map[ProblemSpec]int
	}
	builderHealth := make(map[string]BuilderHealth)

	for _, row := range rows {
		if isWeekend(row.Date) {
			continue
		}

		var builderID = builderID(row.Project, row.Bucket, row.Builder)
		health, ok := builderHealth[builderID]
		if !ok {
			health = BuilderHealth{
				MostRecentRow: row,
				Scores:        make(map[ProblemSpec]int),
			}
		}
		if srcConfig, ok := srcConfigs[row.Project]; !ok {
			continue
		} else if bucketSpec, ok := srcConfig.BucketSpecs[row.Bucket]; !ok {
			continue
		} else if builderSpec, ok := bucketSpec[row.Builder]; !ok {
			continue
		} else {
			for _, problemSpec := range builderSpec.ProblemSpecs {
				var periodDays = problemSpec.PeriodDays

				diffDate := calcBusinessDays(row.Date.In(time.UTC), input.Date.AsTime())
				if diffDate > periodDays {
					continue
				}

				// Let period_days of the unhealthy spec and low-value spec be 5
				// and 90, respectively, and their scores be 5 and 1,
				// respectively.
				// If any score in the last 5 days is greater than 5, the
				// builder is considered healthy.
				// If any score in the last 90 days is greater than 1, the
				// builder is considered unhealthy.
				// Otherwise, the builder is considered low-value.
				currentScore, exists := health.Scores[problemSpec]
				if !exists || row.HealthScore > currentScore {
					health.Scores[problemSpec] = max(row.HealthScore, problemSpec.Score)
				}
			}
		}

		builderHealth[builderID] = health
	}

	rowsWithIndicators := make([]Row, 0, len(builderHealth))

	inactiveBuilders := 0
	healthyBuilders := 0
	unhealthyBuilders := 0
	lowValueBuilders := 0

	for builderID, health := range builderHealth {
		row := health.MostRecentRow

		// Take the min score between the problem specs
		minScore := 10
		for _, score := range health.Scores {
			if score != 0 && score < minScore {
				minScore = score
			}
		}
		row.HealthScore = minScore
		row.ScoreExplanation = scoreExplanation(row, srcConfigs)

		if row.HealthScore > 5 {
			healthyBuilders += 1
		} else if row.HealthScore > 1 {
			unhealthyBuilders += 1
			logging.Errorf(ctx, "Unhealthy builders: %s", builderID)
		} else if row.HealthScore > 0 {
			lowValueBuilders += 1
			logging.Errorf(ctx, "Low-value builders: %s", builderID)
		} else {
			inactiveBuilders += 1
		}

		rowsWithIndicators = append(rowsWithIndicators, row)
	}

	logging.Errorf(ctx, "Total healthy builders: %d", healthyBuilders)
	logging.Errorf(ctx, "Total unhealthy builders: %d", unhealthyBuilders)
	logging.Errorf(ctx, "Total low-value builders: %d", lowValueBuilders)
	logging.Errorf(ctx, "Total inactive builders: %d", inactiveBuilders)

	return rowsWithIndicators, nil
}

func scoreExplanation(row Row, srcConfigs map[string]SrcConfig) string {
	// Second pass to insert just the worst problem's ScoreExplanation
	if srcConfig, ok := srcConfigs[row.Project]; ok {
		if bucketSpec, ok := srcConfig.BucketSpecs[row.Bucket]; ok {
			if builderSpec, ok := bucketSpec[row.Builder]; ok {
				for _, problemSpec := range builderSpec.ProblemSpecs {
					if problemSpec.Score == row.HealthScore {
						explanation := ""
						for _, metric := range row.Metrics {
							if metric.HealthScore != 0 {
								if explanation != "" {
									explanation += "#013;"
								}
								explanation += fmt.Sprintf("%s of %.2f exceeded the threshold of %.2f for %d days", metric.Type, metric.Value, metric.Threshold, problemSpec.PeriodDays)
							}
						}

						return explanation
					}
				}
			}
		}
	}

	return ""
}

func bbClient(buildCtx context.Context) (buildbucketpb.BuildersClient, error) {
	var err error
	step, _ := build.StartStep(buildCtx, "Make BB client")
	defer func() { step.End(err) }()

	authenticator := auth.NewAuthenticator(buildCtx, auth.SilentLogin, auth.Options{})
	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, errors.Annotate(err, "Initializing Auth").Err()
	}

	return buildbucketpb.NewBuildersPRPCClient(&prpc.Client{
		C:    httpClient,
		Host: "cr-buildbucket.appspot.com",
	}), nil
}

func rpcBuildbucket(buildCtx context.Context, rows []Row, client BBClient) error {
	var err error
	step, ctx := build.StartStep(buildCtx, "RPC Buildbucket")
	defer func() { step.End(err) }()

	healthProtos := make([]*buildbucketpb.SetBuilderHealthRequest_BuilderHealth, len(rows))
	for i, row := range rows {
		simplifiedMetrics := map[string]float32{}
		for _, metric := range row.Metrics {
			simplifiedMetrics[metric.Type] = metric.Value
		}

		dashboardLink := fmt.Sprintf("http://go/builder-health-indicators?f=builder:in:%s&f=bucket:in:%s&f=project:in:%s",
			url.QueryEscape(row.Builder), url.QueryEscape(row.Bucket), url.QueryEscape(row.Project))
		const designDocLink = "http://go/builder-health-metrics-design"
		healthProtos[i] = &buildbucketpb.SetBuilderHealthRequest_BuilderHealth{
			Id: &buildbucketpb.BuilderID{Project: row.Project, Bucket: row.Bucket, Builder: row.Builder},
			Health: &buildbucketpb.HealthStatus{
				HealthScore:   int64(row.HealthScore),
				HealthMetrics: simplifiedMetrics,
				Description:   row.ScoreExplanation,
				DocLinks: map[string]string{
					"":             "https://chromium.googlesource.com/chromium/src/+/refs/heads/main/infra/config/generated/health-specs/health-specs.json",
					"google.com":   designDocLink,
					"chromium.org": designDocLink,
				},
				DataLinks: map[string]string{
					"":             "https://chromium.googlesource.com/chromium/src/+/refs/heads/main/infra/config/generated/health-specs/health-specs.json",
					"google.com":   dashboardLink,
					"chromium.org": dashboardLink,
				},
			},
		}
	}
	req := &buildbucketpb.SetBuilderHealthRequest{
		Health: healthProtos,
	}
	res, err := client.SetBuilderHealth(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "Set builder health error result: %+v. Error: %s", res, err)
		return errors.Annotate(err, "Set builder health").Err()
	}

	nErrors := 0
	for _, resp := range res.Responses {
		if resp.GetError() == nil {
			continue
		}

		nErrors += 1
		logging.Errorf(ctx, "Set builder health error: %s.", resp.GetError().String())
	}

	if nErrors > 0 {
		step.SetSummaryMarkdown(fmt.Sprintf("%d set builder health requests failed", nErrors))
	}

	return nil
}

func writeIndicators(buildCtx context.Context, bqClient *bigquery.Client, rows []Row) error {
	var err error
	step, ctx := build.StartStep(buildCtx, "Write indicators")
	defer func() { step.End(err) }()

	step.SetSummaryMarkdown("Writing to BQ table cr-builder-health-indicators.builder_health_indicators.builder-health-indicators")

	inserter := bqClient.Dataset("builder_health_indicators").Table("builder-health-indicators").Inserter()
	if err := inserter.Put(ctx, rows); err != nil {
		return err
	}

	return nil
}

func logIndicators(buildCtx context.Context, rowsWithIndicators []Row) error {
	var stepErr error
	step, ctx := build.StartStep(buildCtx, "Print indicators")
	defer func() { step.End(stepErr) }()

	for _, row := range rowsWithIndicators {
		logging.Errorf(ctx, "%s/%s/%s: HealthScore: %d.", row.Project, row.Bucket, row.Builder, row.HealthScore)
		for _, metric := range row.Metrics {
			logging.Errorf(ctx, "%+v", *metric)
		}
		logging.Errorf(ctx, "")
	}

	return nil
}
