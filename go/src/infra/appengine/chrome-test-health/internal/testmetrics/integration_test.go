// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//go:build integration
// +build integration

package testmetrics

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/appengine/chrome-test-health/api"
)

// Runs the integration tests returning an error if any fail to run or a
// check fails
func TestIntegrationTest(t *testing.T) {
	ctx := context.Background()

	// Setup the test environment
	bqClient, err := bigquery.NewClient(ctx, testProject)
	if err != nil {
		t.Fail()
	}

	client, err := setupClient(ctx, bqClient, testDataset, testProject)
	if err != nil {
		t.Fail()
	}

	if err := ensureTables(ctx, bqClient); err != nil {
		t.Fail()
	}

	rf := &resultFactory{}

	ftt.Run("no duplicate rows are created", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table
		testPartition := "2023-07-02"
		rf.defaultRuntime = (time.Hour * 24).Seconds()
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		tf := taskFactory{
			defaultCores:    1,
			defaultDuration: (time.Hour * 24).Seconds(),
		}

		// Generate the fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult(),
			rf.createResult(),
			rf.createResult().AddTime(time.Hour * 24),
			rf.createResult().AddTime(time.Hour * 24 * 2),
			rf.createResult().AddTime(time.Hour * 24 * 6),
			// Ensure the corrections table doesn't cause dupes
			rf.createResult().WithBuilder("fake-builder").WithTestSuite("fake-suite").WithPlatform("fake-platform").InBuild("build1"),
		}, []*fakeTask{
			tf.createTask().OnDay(rf.timePartition).WithId("build1"),
		}); err != nil {
			t.Fail()
		}

		// Even if new data appears after the first roll up, that data needs to
		// be included in existing rows, not as new ones
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult(),
			rf.createResult().AddTime(time.Hour * 24),
			rf.createResult().AddTime(time.Hour * 24 * 2),
			rf.createResult().AddTime(time.Hour * 24 * 6),
			rf.createResult().WithBuilder("fake-builder").WithTestSuite("fake-suite").WithPlatform("changed-fake-platform").InBuild("build2"),
		}, []*fakeTask{
			tf.createTask().OnDay(rf.timePartition).WithId("build2"),
		}); err != nil {
			t.Fail()
		}

		// Check the rolled up tables
		err := checkForDuplicateRows(ctx, bqClient)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("fetch", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table
		testPartition := "2023-06-01"
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult().AddTime(-time.Second),
			rf.createResult(),
			rf.createResult(),
			rf.createResult().Failed(),
			rf.createResult().WithBuilder("different_builder"),
			rf.createResult().AddTime(time.Hour * 24),
		}, nil); err != nil {
			t.Fail()
		}

		// Start checking the fetches
		resp, err := client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics:    []api.MetricType{api.MetricType_NUM_RUNS},
				PageSize:   10,
			},
		)

		assert.Loosely(t, err, should.BeNil)

		// Check the test_id rollup is correct
		testSummary := getTestIdFromResponse(resp, defaultTestId)
		testMetricData := testSummary.Metrics[testPartition].Data[0]
		assert.Loosely(t, testMetricData.MetricType, should.Equal(api.MetricType_NUM_RUNS))
		assert.Loosely(t, testMetricData.MetricValue, should.Equal(4))

		variant := getBuilderVariantFromTest(testSummary, "builder")
		variantMetricData := variant.Metrics[testPartition].Data[0]
		assert.Loosely(t, variantMetricData.MetricType, should.Equal(api.MetricType_NUM_RUNS))
		assert.Loosely(t, variantMetricData.MetricValue, should.Equal(3))

		variant = getBuilderVariantFromTest(testSummary, "different_builder")
		variantMetricData = variant.Metrics[testPartition].Data[0]
		assert.Loosely(t, variantMetricData.MetricType, should.Equal(api.MetricType_NUM_RUNS))
		assert.Loosely(t, variantMetricData.MetricValue, should.Equal(1))
	})

	ftt.Run("total runtime", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table. Use a Sunday to make weekly tests easier
		testPartition := "2023-05-07"

		// Setup defaults for rdb data
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		rf.defaultFilename = "//dir/name/filename.go"
		rf.defaultRuntime = 1.0

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult(),
			rf.createResult(),
			rf.createResult().WithFilename("//dir/other/name/filename.go"),
			rf.createResult().WithBuilder("different_builder"),
			rf.createResult().WithBuilder("different_builder").WithFilename("//dir/other/name/filename.go"),
		}, nil); err != nil {
			t.Fail()
		}

		// Start checking the fetches
		testResp, err := client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_TOTAL_RUNTIME,
				},
				PageSize: 10,
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary := getTestIdFromResponse(testResp, defaultTestId)
		metric, err := getMetric(testSummary.Metrics[testPartition], api.MetricType_TOTAL_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Each test is 1 second, 5 tests on this day should be 5 total runtime
		assert.Loosely(t, metric, should.Equal(5))

		testResp, err = client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_TOTAL_RUNTIME,
				},
				Filter:   "different_builder",
				PageSize: 10,
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary = getTestIdFromResponse(testResp, defaultTestId)
		metric, err = getMetric(testSummary.Metrics[testPartition], api.MetricType_TOTAL_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Each test is 1 second, only 2 tests on this day use
		// "different_builder" should be 2 total runtime
		assert.Loosely(t, metric, should.Equal(2))

		// Start checking the fetches
		dirResp, err := client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_TOTAL_RUNTIME,
				},
				ParentIds: []string{"/"},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_TOTAL_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Each test is 1 second, 5 tests on this day should be 5 total runtime
		assert.Loosely(t, metric, should.Equal(5))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_TOTAL_RUNTIME,
				},
				ParentIds: []string{"/"},
				Filter:    "different_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_TOTAL_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Each test is 1 second, only 2 tests on this day use
		// "different_builder" should be 2 total runtime
		assert.Loosely(t, metric, should.Equal(2))
	})

	ftt.Run("avg cores unfiltered", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table. Use a Sunday to make weekly tests easier
		testPartition := "2023-04-02"

		// Setup defaults for rdb data
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		rf.defaultFilename = "//dir/name/filename.go"
		// Make tests run all day for 7 days so all avg cores will be 1
		rf.defaultRuntime = (time.Hour * 24).Seconds()
		tf := taskFactory{
			defaultCores:    1,
			defaultDuration: (time.Hour * 24).Seconds(),
		}

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult().AddTime(time.Hour * 24 * 0).InBuild("build1"),
			rf.createResult().AddTime(time.Hour * 24 * 1).InBuild("build2"),
			rf.createResult().AddTime(time.Hour * 24 * 2).InBuild("build3"),
			rf.createResult().AddTime(time.Hour * 24 * 3).InBuild("build4"),
			rf.createResult().AddTime(time.Hour * 24 * 4).InBuild("build5"),
			rf.createResult().AddTime(time.Hour * 24 * 5).InBuild("build6"),
			rf.createResult().AddTime(time.Hour * 24 * 6).InBuild("build7"),
			// Force the weekly cores to be 2
			rf.createResult().AddTime(time.Hour * 24 * 6).InBuild("build8").WithDuration(time.Hour * 24 * 7),
		}, []*fakeTask{
			tf.createTask().OnDay(rf.timePartition.AddDays(0)).WithId("build1"),
			tf.createTask().OnDay(rf.timePartition.AddDays(1)).WithId("build2"),
			tf.createTask().OnDay(rf.timePartition.AddDays(2)).WithId("build3"),
			tf.createTask().OnDay(rf.timePartition.AddDays(3)).WithId("build4"),
			tf.createTask().OnDay(rf.timePartition.AddDays(4)).WithId("build5"),
			tf.createTask().OnDay(rf.timePartition.AddDays(5)).WithId("build6"),
			tf.createTask().OnDay(rf.timePartition.AddDays(6)).WithId("build7"),
			tf.createTask().OnDay(rf.timePartition.AddDays(6)).WithId("build8").WithDuration(time.Hour * 24 * 7),
		}); err != nil {
			t.Fail()
		}

		// Start checking the fetches
		testResp, err := client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition, "2023-04-08"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				PageSize: 10,
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary := getTestIdFromResponse(testResp, defaultTestId)
		metric, err := getMetric(testSummary.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The test ran for 24 hours on the Sunday consuming 1 core the whole time
		assert.Loosely(t, metric, should.Equal(1))
		metric, err = getMetric(testSummary.Metrics["2023-04-08"], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The test ran for 1 + 7 days on the Saturday over 2 results
		assert.Loosely(t, metric, should.Equal(8))

		// Verify weekly
		testResp, err = client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_WEEK,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				PageSize: 10,
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary = getTestIdFromResponse(testResp, defaultTestId)
		metric, err = getMetric(testSummary.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The total runtime should be 14 days which over 7 days is 2 cores
		assert.Loosely(t, metric, should.Equal(2))

		dirResp, err := client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition, "2023-04-08"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				ParentIds: []string{"/"},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The test ran for 24 hours on the Sunday consuming 1 core the whole time
		assert.Loosely(t, metric, should.Equal(1))
		metric, err = getMetric(dirResp.Nodes[0].Metrics["2023-04-08"], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The test ran for 1 + 7 days on the Saturday over 2 results
		assert.Loosely(t, metric, should.Equal(8))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_WEEK,
				Components: []string{"component"},
				Dates:      []string{testPartition, "2023-04-08"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				ParentIds: []string{"/"},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The total runtime should be 14 days which over 7 days is 2 cores
		assert.Loosely(t, metric, should.Equal(2))
	})

	ftt.Run("avg cores filtered", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table. Use a Sunday to make weekly tests easier
		testPartition := "2023-03-19"

		// Setup defaults for rdb data
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		rf.defaultFilename = "//dir/name/filename.go"
		// Make tests run all day for 7 days so all avg cores will be 1
		rf.defaultRuntime = (time.Hour * 24).Seconds()

		tf := taskFactory{
			defaultCores: 1,
		}

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult().AddTime(time.Hour * 24 * 6).InBuild("build1"),
			// Force the weekly cores to be 1 split when filter is "other_builder"
			rf.createResult().AddTime(time.Hour * 24 * 6).InBuild("build2").WithDuration(time.Hour * 24 * 7).WithBuilder("other_builder"),
		}, []*fakeTask{
			// Force the correction factor to 1 (rdb time == swarming time)
			tf.createTask().WithId("build1").OnDay(rf.timePartition.AddDays(6)).WithDuration(time.Hour * 24),
			tf.createTask().WithId("build2").OnDay(rf.timePartition.AddDays(6)).WithDuration(time.Hour * 24 * 7),
		}); err != nil {
			t.Fail()
		}

		testResp, err := client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition, "2023-03-25"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				PageSize: 10,
				Filter:   "other_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary := getTestIdFromResponse(testResp, defaultTestId)
		// other_builder variant did not run Sunday, we shouldn't get anything for this day
		assert.Loosely(t, testSummary.Metrics, should.NotContainKey(testPartition))
		metric, err := getMetric(testSummary.Metrics["2023-03-25"], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The other_builder test ran for 7 days on the Saturday over 1 day is 7 cores
		assert.Loosely(t, metric, should.Equal(7))

		// Verify weekly
		testResp, err = client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_WEEK,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				PageSize: 10,
				Filter:   "other_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary = getTestIdFromResponse(testResp, defaultTestId)
		metric, err = getMetric(testSummary.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The total runtime for other_builder should be 7 days over 7 days which is 1 core
		assert.Loosely(t, metric, should.Equal(1))

		dirResp, err := client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition, "2023-03-25"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				ParentIds: []string{"/"},
				Filter:    "other_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)
		// other_builder variant did not run Sunday, we shouldn't get anything for this day
		assert.Loosely(t, dirResp.Nodes[0].Metrics, should.NotContainKey(testPartition))
		metric, err = getMetric(dirResp.Nodes[0].Metrics["2023-03-25"], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The other_builder test ran for 7 days on the Saturday over 1 day is 7 cores
		assert.Loosely(t, metric, should.Equal(7))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_WEEK,
				Components: []string{"component"},
				Dates:      []string{testPartition, "2023-03-25"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				ParentIds: []string{"/"},
				Filter:    "other_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)
		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The total runtime for other_builder should be 7 days over 7 days which is 1 core
		assert.Loosely(t, metric, should.Equal(1))
	})

	ftt.Run("avg cores corrections", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table. Use a Sunday to make weekly tests easier
		testPartition := "2023-02-26"

		// Setup defaults for rdb data
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		rf.defaultFilename = "//dir/name/filename.go"
		// Make tests run all day for day each so all avg cores will be 1
		rf.defaultRuntime = (time.Hour * 24).Seconds()

		tf := taskFactory{
			defaultCores:    1,
			defaultDuration: (time.Hour * 24).Seconds(),
			defaultEndTime:  rf.timePartition,
		}

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			rf.createResult().InBuild("build1").WithBuilder("builder1"),
			rf.createResult().InBuild("build2").WithBuilder("builder2"),
			rf.createResult().InBuild("build3").WithBuilder("builder3"),
			rf.createResult().InBuild("build4").WithBuilder("builder4").WithId("other_test"),
			rf.createResult().InBuild("build4").WithBuilder("builder4"),
			rf.createResult().InBuild("build5").WithBuilder("builder5"),
		}, []*fakeTask{
			// Swarming time == Rdb time
			tf.createTask().WithId("build1"),
			// Swarming cores make swarming time 2x rdb time
			tf.createTask().WithId("build2").WithCores(2),
			// Swarming time is 3x rdb time
			tf.createTask().WithId("build3").WithDuration(time.Hour * 24 * 3),
			// Swarming time == rdb time over 2 tests (correction is 1)
			tf.createTask().WithId("build4").WithDuration(time.Hour * 24 * 2),
			// Swarming time is half rdb time the correction should be .5
			tf.createTask().WithId("build5").WithDuration(time.Hour * 12),
		}); err != nil {
			t.Fail()
		}

		// Start checking the fetches
		testResp, err := client.FetchMetrics(ctx,
			&api.FetchTestMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_AVG_CORES,
				},
				PageSize: 10,
			},
		)
		assert.Loosely(t, err, should.BeNil)

		testSummary := getTestIdFromResponse(testResp, defaultTestId)
		cores, err := getMetric(testSummary.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// The total core-swarming time is the sum of the variants. 7.5 cores
		// would be busy this whole day to run the variants of this test
		assert.Loosely(t, cores, should.Equal(7.5))

		variant1 := getBuilderVariantFromTest(testSummary, "builder1")
		cores, err = getMetric(variant1.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cores, should.Equal(1))

		variant2 := getBuilderVariantFromTest(testSummary, "builder2")
		cores, err = getMetric(variant2.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// builder2 had twice the cores but consumed the same swarming time so
		// it used twice the cores
		assert.Loosely(t, cores, should.Equal(2))

		variant3 := getBuilderVariantFromTest(testSummary, "builder3")
		cores, err = getMetric(variant3.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// builder3 reported the same time to rdb but actually used three times
		// the swarming time and should also be 3
		assert.Loosely(t, cores, should.Equal(3))

		variant4 := getBuilderVariantFromTest(testSummary, "builder4")
		cores, err = getMetric(variant4.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// builder4 had 2x the swarming task but split between 2 test ids
		assert.Loosely(t, cores, should.Equal(1))

		variant5 := getBuilderVariantFromTest(testSummary, "builder5")
		cores, err = getMetric(variant5.Metrics[testPartition], api.MetricType_AVG_CORES)
		assert.Loosely(t, err, should.BeNil)
		// builder5 over reported rdb time, it actually used .5
		assert.Loosely(t, cores, should.Equal(.5))
	})

	ftt.Run("file based component aggregations", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table. Use a Sunday to make weekly tests easier
		testPartition := "2023-03-12"

		// Setup defaults for rdb data
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		rf.defaultFilename = "//dir/name/filename.go"
		rf.defaultRuntime = (time.Hour * 24).Seconds()

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			// All tests exist in the same file but with different component/builder combinations
			rf.createResult().WithId("test1").WithComponent("component1").WithDuration(time.Minute * 1),
			rf.createResult().WithId("test2").WithComponent("component1").WithDuration(time.Minute * 5).WithBuilder("other_builder"),
			rf.createResult().WithId("test3").WithComponent("component2").WithDuration(time.Minute * 10).WithBuilder("other_builder"),
			// Add an entry that shouldn't affect the previous day
			rf.createResult().WithId("test1").WithComponent("component1").WithDuration(time.Hour).AddTime(time.Hour * 24),
			rf.createResult().WithId("test3").WithComponent("component2").WithDuration(time.Hour).AddTime(time.Hour * 24).WithBuilder("other_builder"),
		}, nil); err != nil {
			t.Fail()
		}

		dirResp, err := client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_AVG_RUNTIME,
				},
				ParentIds: []string{"/"},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err := getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// 3 tests in the same file on the first day make the avg runtime of
		// the file the sum of those 3 tests (1 + 5 + 10)
		assert.Loosely(t, metric, should.Equal((time.Minute * 16).Seconds()))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{},
				// Add the next day to exercise the multi-day query
				Dates: []string{testPartition, "2023-03-13"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_RUNTIME,
				},
				ParentIds: []string{"/"},
			},
		)
		assert.Loosely(t, err, should.BeNil)
		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// The first day should not be affected by fetching the second
		assert.Loosely(t, metric, should.Equal((time.Minute * 16).Seconds()))
		metric, err = getMetric(dirResp.Nodes[0].Metrics["2023-03-13"], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, metric, should.Equal((time.Hour * 2).Seconds()))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_AVG_RUNTIME,
				},
				ParentIds: []string{"/"},
				Filter:    "other_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// 2 tests belong to other_builder on the first day (5 + 10)
		assert.Loosely(t, metric, should.Equal((time.Minute * 15).Seconds()))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{},
				// Add the next day to exercise the multi-day query
				Dates: []string{testPartition, "2023-03-13"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_RUNTIME,
				},
				ParentIds: []string{"/"},
				Filter:    "other_builder",
			},
		)
		assert.Loosely(t, err, should.BeNil)
		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// The first day should not be affected by fetching the second
		assert.Loosely(t, metric, should.Equal((time.Minute * 15).Seconds()))
		metric, err = getMetric(dirResp.Nodes[0].Metrics["2023-03-13"], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Only 1 test on the 2nd day for other_builder
		assert.Loosely(t, metric, should.Equal((time.Hour).Seconds()))

		dirResp, err = client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"component1"},
				Dates:      []string{testPartition, "2023-03-13"},
				Metrics: []api.MetricType{
					api.MetricType_AVG_RUNTIME,
				},
				ParentIds: []string{"/"},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		metric, err = getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// 2 tests have component1 on the first day make the avg runtime of
		// the file the sum of those 2 tests (1 + 5)
		assert.Loosely(t, metric, should.Equal((time.Minute * 6).Seconds()))
		metric, err = getMetric(dirResp.Nodes[0].Metrics["2023-03-13"], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Only 1 test on the 2nd day for component1
		assert.Loosely(t, metric, should.Equal((time.Hour).Seconds()))
	})

	ftt.Run("invalid file name summaries", t, func(t *ftt.Test) {
		// Deleting rows with a streaming buffer doesn't work well, instead
		// partition the fake table. Use a Sunday to make weekly tests easier
		testPartition := "2023-02-26"

		// Setup defaults for rdb data
		rf.timePartition, err = civil.ParseDate(testPartition)
		if err != nil {
			t.Fail()
		}
		rf.defaultRuntime = (time.Hour * 24).Seconds()

		// Generate the rollups from fake rdb data.
		if err := createRollupFromResults(ctx, client, testDataset, []*fakeRdbResult{
			// All tests exist in the same file but with different component/builder combinations
			rf.createResult().WithId("test1").WithComponent("Unknown").WithDuration(time.Minute * 1).WithFilename("//dir/name/filename.go"),
			rf.createResult().WithId("test2").WithComponent("Unknown").WithDuration(time.Minute * 3).WithFilename("Unknown File"),
			rf.createResult().WithId("test3").WithComponent("Unknown").WithDuration(time.Minute * 7).WithFilename("Unknown File"),
		}, nil); err != nil {
			t.Fail()
		}

		dirResp, err := client.FetchDirectoryMetrics(ctx,
			&api.FetchDirectoryMetricsRequest{
				Period:     api.Period_DAY,
				Components: []string{"Unknown"},
				Dates:      []string{testPartition},
				Metrics: []api.MetricType{
					api.MetricType_AVG_RUNTIME,
				},
				// "" Should return any file name not in the root "//"
				ParentIds: []string{""},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, len(dirResp.Nodes), should.Equal(1))
		metric, err := getMetric(dirResp.Nodes[0].Metrics[testPartition], api.MetricType_AVG_RUNTIME)
		assert.Loosely(t, err, should.BeNil)
		// Only test2 and test3 with "Unknown File" should appear
		assert.Loosely(t, metric, should.Equal((time.Minute * 10).Seconds()))
	})
}
