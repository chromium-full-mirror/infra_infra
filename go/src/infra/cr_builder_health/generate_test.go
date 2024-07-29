// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cr_builder_health/healthpb"
)

type bbClientMock struct {
	setHealthCalls int
}

var existantBuilder = "existant-builder"
var nonExistantBuilder = "non-existant-builder"

func (c *bbClientMock) SetBuilderHealth(ctx context.Context, in *buildbucketpb.SetBuilderHealthRequest, opts ...grpc.CallOption) (*buildbucketpb.SetBuilderHealthResponse, error) {
	c.setHealthCalls += 1
	result := &buildbucketpb.SetBuilderHealthResponse{
		Responses: []*buildbucketpb.SetBuilderHealthResponse_Response{},
	}

	for _, req := range in.Health {
		if req.Id.Builder == existantBuilder {
			result.Responses = append(result.Responses, &buildbucketpb.SetBuilderHealthResponse_Response{
				Response: &buildbucketpb.SetBuilderHealthResponse_Response_Result{},
			})
		} else {
			result.Responses = append(result.Responses, &buildbucketpb.SetBuilderHealthResponse_Response{
				Response: &buildbucketpb.SetBuilderHealthResponse_Response_Error{
					Error: &status.Status{
						Code:    400,
						Message: "Invalid builder name",
					},
				},
			})
		}
	}

	return result, nil
}

func TestIsWeekend(t *testing.T) {
	t.Parallel()
	ftt.Run("Test isWeekend function", t, func(t *ftt.Test) {
		date1 := civil.Date{
			Year:  2023,
			Month: time.December,
			Day:   1,
		}
		date2 := civil.Date{
			Year:  2023,
			Month: time.December,
			Day:   2,
		}
		date3 := civil.Date{
			Year:  2023,
			Month: time.December,
			Day:   3,
		}
		date4 := civil.Date{
			Year:  2023,
			Month: time.December,
			Day:   4,
		}
		assert.Loosely(t, isWeekend(date1), should.Equal(false))
		assert.Loosely(t, isWeekend(date2), should.Equal(true))
		assert.Loosely(t, isWeekend(date3), should.Equal(true))
		assert.Loosely(t, isWeekend(date4), should.Equal(false))
	})
}

func TestBuilderID(t *testing.T) {
	t.Parallel()
	ftt.Run("Test BuilderID function", t, func(t *ftt.Test) {
		assert.Loosely(t, builderID("chromium", "ci", "builder1"), should.Equal("chromium/ci/builder1"))
		assert.Loosely(t, builderID("chrome", "try", "builder2"), should.Equal("chrome/try/builder2"))
	})
}

func TestCalculateIndicators(t *testing.T) {
	t.Parallel()

	var srcConfig = map[string]SrcConfig{
		"project": {
			BucketSpecs: map[string]BuilderSpecs{
				"bucket": {
					"existant-builder": BuilderSpec{
						ProblemSpecs: []ProblemSpec{
							{
								Name:       "Unhealthy",
								PeriodDays: 7,
								Score:      UNHEALTHY_SCORE,
								Thresholds: Thresholds{
									FailRate: AverageThresholds{Average: 0.2},
								},
							},
							{
								Name:       "Low Value",
								PeriodDays: 7,
								Score:      LOW_VALUE_SCORE,
								Thresholds: Thresholds{
									FailRate: AverageThresholds{Average: 0.9},
								},
							},
						},
					},
				},
			},
		},
	}

	var input = healthpb.InputParams{
		Date: timestamppb.New(time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)),
	}

	ftt.Run("Weekend score is discarded", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: HEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   6, // Saturday
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: UNHEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   5,
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		// As 2024/01/06, being a Saturday, is excluded from the health score calculation, the final health score should be UNHEALTHY_SCORE
		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
	},
	)

	ftt.Run("Score in out-of-period date is discarded", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: UNHEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   1,
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: HEALTHY_SCORE,
			Date: civil.Date{
				Year:  2023,
				Month: time.December,
				Day:   29, // Friday but out of the 7-day period
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
	},
	)

	ftt.Run("Healthy & Healthy --> Healthy builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: HEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   2,
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: HEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   1,
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(HEALTHY_SCORE))
	},
	)

	ftt.Run("Healthy & Unhealthy --> Healthy builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: HEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   2,
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: UNHEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   1,
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(HEALTHY_SCORE))
	},
	)

	ftt.Run("Unhealthy & Unhealthy --> Unhealthy builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: UNHEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   2,
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: UNHEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   1,
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
	},
	)

	ftt.Run("Unhealthy & Low-value --> Unhealthy builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: UNHEALTHY_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   2,
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: LOW_VALUE_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   1,
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(UNHEALTHY_SCORE))
	},
	)

	ftt.Run("Low-value & Low-value --> Low-value builder", t, func(t *ftt.Test) {
		ctx := context.Background()
		rowsWithHealthScores := []Row{{
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: LOW_VALUE_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   2,
			},
		}, {
			Project:     "project",
			Bucket:      "bucket",
			Builder:     existantBuilder,
			HealthScore: LOW_VALUE_SCORE,
			Date: civil.Date{
				Year:  2024,
				Month: time.January,
				Day:   1,
			},
		}}

		rowsWithIndicators, err := calculateIndicators(ctx, &input, rowsWithHealthScores, srcConfig)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(rowsWithIndicators), should.Equal(1))

		assert.Loosely(t, rowsWithIndicators[0].HealthScore, should.Equal(LOW_VALUE_SCORE))
	},
	)
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	ftt.Run("RPC Buildbucket is called ok", t, func(t *ftt.Test) {
		ctx := context.Background()
		client := &bbClientMock{}
		rows := []Row{{
			Project: "project",
			Bucket:  "bucket",
			Builder: existantBuilder,
			Metrics: []*Metric{
				{Type: "build_mins_p50", Value: 59},
				{Type: "build_mins_p95", Value: 119},
				{Type: "pending_mins_p50", Value: 59},
				{Type: "pending_mins_p95", Value: 119},
				{Type: "fail_rate", Value: 0.05},
				{Type: "infra_fail_rate", Value: 0},
			},
		}}
		err := rpcBuildbucket(ctx, rows, client)
		assert.Loosely(t, client.setHealthCalls, should.Equal(1))
		assert.Loosely(t, ctx.Err(), should.BeNil)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("RPC Buildbucket is called error", t, func(t *ftt.Test) {
		ctx := context.Background()
		client := &bbClientMock{}
		rows := []Row{
			{
				Project: "project",
				Bucket:  "bucket",
				Builder: nonExistantBuilder,
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
				Bucket:  "bucket",
				Builder: existantBuilder,
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
		err := rpcBuildbucket(ctx, rows, client)
		assert.Loosely(t, client.setHealthCalls, should.Equal(1))
		assert.Loosely(t, ctx.Err(), should.BeNil)
		assert.Loosely(t, err, should.BeNil)
	})
}
