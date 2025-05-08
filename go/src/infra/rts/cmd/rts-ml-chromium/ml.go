// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/rts/cmd/rts-ml-chromium/proto"
)

var mlCli string = "ml_cli_logit.py"

// A single test's stability for a day (Timestamp) including its identify
// information
type bqStabilityRow struct {
	TestId      string    `bigquery:"test_id"`
	VariantHash string    `bigquery:"variant_hash"`
	Stability   stability `bigquery:"stability"`
	TestSuite   string    `bigquery:"test_suite"`
	Timestamp   time.Time `bigquery:"day"`
}

// The stability information for a single test on a single day
type stability struct {
	SixMonthFailCount int64 `bigquery:"six_month_fail_count"`
	SixMonthRunCount  int64 `bigquery:"six_month_run_count"`
	OneMonthFailCount int64 `bigquery:"one_month_fail_count"`
	OneMonthRunCount  int64 `bigquery:"one_month_run_count"`
	OneWeekFailCount  int64 `bigquery:"one_week_fail_count"`
	OneWeekRunCount   int64 `bigquery:"one_week_run_count"`
}

// A single example that the model will make a prediction against
type mlExample struct {
	TestId            string
	TestSuite         string
	SixMonthFailCount int64
	SixMonthRunCount  int64
	OneMonthFailCount int64
	OneMonthRunCount  int64
	OneWeekFailCount  int64
	OneWeekRunCount   int64
	GitDistance       float64
	UseGitDistance    bool
	FileDistance      float64
	UseFileDistance   bool
}

// Convert the stability information into an mlExample for the ML model
func (r bqStabilityRow) mlExample() *mlExample {
	return &mlExample{
		TestId:            r.TestId,
		TestSuite:         r.TestSuite,
		SixMonthFailCount: r.Stability.SixMonthFailCount,
		SixMonthRunCount:  r.Stability.SixMonthRunCount,
		OneMonthFailCount: r.Stability.OneMonthFailCount,
		OneMonthRunCount:  r.Stability.OneMonthRunCount,
		OneWeekFailCount:  r.Stability.OneWeekFailCount,
		OneWeekRunCount:   r.Stability.OneWeekRunCount,
		GitDistance:       0,
		UseGitDistance:    false,
		FileDistance:      0,
		UseFileDistance:   false,
	}
}

// Trains a model by calling the cli with the provided file
func trainMlModel(ctx context.Context, trainingDataFile string, modelDir string) error {
	cmd := exec.Command("vpython3",
		filepath.Join(modelDir, mlCli),
		"train",
		"--train-data",
		trainingDataFile,
		"--output",
		filepath.Join(modelDir, "saved_model"),
	)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()

	if err != nil {
		logging.Infof(ctx, "stdout from cli:\n", string(stdoutBuf.String()))
		logging.Infof(ctx, "stderr from cli:\n", string(errBuf.String()))
		return err
	}

	return nil
}

// Uses the ml cli to make predictions. Passes the dataframes to the cli through
// a file to avoid command line argument limits
func fileInferMlModel(ctx context.Context, rows []*mlExample, modelDir string) ([]float64, error) {
	predictions_dir, err := filepath.Abs("predictions")
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(predictions_dir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	featuresFile, err := ioutil.TempFile(predictions_dir, "PsFeatures_thread*.csv")
	if err != nil {
		return nil, err
	}

	featureFileName, err := filepath.Abs(featuresFile.Name())
	if err != nil {
		return nil, err
	}
	defer os.Remove(featureFileName)

	predictionsFileName, err := filepath.Abs(strings.Replace(featureFileName, ".csv", "_predict.csv", -1))
	if err != nil {
		return nil, err
	}

	csvWriter := csv.NewWriter(featuresFile)

	csvData := [][]string{{
		"GitDistance",
		"FileDistance",
		"OneWeekFailCount",
		"OneWeekRunCount",
		"OneMonthFailCount",
		"OneMonthRunCount",
		"SixMonthFailCount",
		"SixMonthRunCount"},
	}

	for _, row := range rows {
		gitDistance := ""
		if row.UseGitDistance {
			gitDistance = fmt.Sprintf("%v", row.GitDistance)
		}
		fileDistance := ""
		if row.UseFileDistance {
			fileDistance = fmt.Sprintf("%v", row.FileDistance)
		}
		csvData = append(csvData,
			[]string{
				gitDistance,
				fileDistance,
				fmt.Sprintf("%v", row.OneWeekFailCount),
				fmt.Sprintf("%v", row.OneWeekRunCount),
				fmt.Sprintf("%v", row.OneMonthFailCount),
				fmt.Sprintf("%v", row.OneMonthRunCount),
				fmt.Sprintf("%v", row.SixMonthFailCount),
				fmt.Sprintf("%v", row.SixMonthRunCount),
			})
	}
	csvWriter.WriteAll(csvData)

	cmd := exec.Command("vpython3",
		filepath.Join(modelDir, mlCli),
		"predict",
		"--file",
		featureFileName,
		"--output",
		predictionsFileName,
		"--model",
		filepath.Join(modelDir, "saved_model"),
	)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err = cmd.Run()

	if err != nil {
		logging.Infof(ctx, "stdout from cli:\n", outBuf.String())
		logging.Infof(ctx, "stderr from cli:\n", errBuf.String())
		return nil, err
	}

	fileText, err := ioutil.ReadFile(predictionsFileName)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(fileText)), "\n")

	if len(rows) != len(lines) {
		logging.Infof(ctx, "ML inference returned too many predictions\n")
		return nil, errors.New("ML inference returned too many predictions")
	}

	outDistances := make([]float64, len(rows))
	for i, line := range lines {

		distance, err := strconv.ParseFloat(line, 64)
		if err != nil {
			return nil, errors.Annotate(err, "failed to parse a prediction from %q", line).Err()
		}

		// "Distance" and "Failure" have reverse meaning on whether to run
		outDistances[i] = 1. - distance
	}
	return outDistances, nil
}

type stabilityMapKey struct {
	date   time.Time
	testID string
}

// Returns the test fail rates as they were between the provided time period
// as a map of the day
func getTestIdToStabilityRowMap(ctx context.Context, bqClient *bigquery.Client, builder string, testSuite string, start time.Time, stop time.Time) (map[stabilityMapKey]*mlExample, error) {
	testStabilityMap := make(map[stabilityMapKey]*mlExample)

	err := queryStability(ctx, bqClient, builder, testSuite, start, stop, func(row *bqStabilityRow) {
		key := stabilityMapKey{date: row.Timestamp, testID: row.TestId}
		testStabilityMap[key] = row.mlExample()
	})
	if err != nil {
		return nil, err
	}

	return testStabilityMap, err
}

// Get the historic stability for the given time range
func queryStability(ctx context.Context, bqClient *bigquery.Client, builder string, testSuite string, startTime time.Time, endTime time.Time, visitor func(*bqStabilityRow)) error {
	q := bqClient.Query(evalStabilityQuery)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "start_day", Value: startTime},
		{Name: "end_day", Value: endTime},
		{Name: "query_builder", Value: builder},
		{Name: "query_test_suite", Value: testSuite},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return err
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Read the next row.
		row := &bqStabilityRow{}
		switch err := it.Next(row); {
		case err == iterator.Done:
			return ctx.Err()
		case err != nil:
			return err
		}
		visitor(row)
	}
}

// ReadCurrentStability reads TestStability written by writeTestFilesFrom().
func ReadCurrentStability(r io.Reader, callback func(*proto.TestStability) error) error {
	scan := bufio.NewScanner(r)
	line := 0
	scan.Buffer(nil, 1e7) // 10 MB.
	for scan.Scan() {
		line++
		testStability := &proto.TestStability{}
		if err := protojson.Unmarshal(scan.Bytes(), testStability); err != nil {
			errors.Annotate(err, "failed to parse current stability at line %d", line).Err()
			return err
		}
		if err := callback(testStability); err != nil {
			return err
		}
	}
	return scan.Err()
}

// WriteCurrentStability writes TestStability protobuf messages to w in JSON Lines format.
func WriteCurrentStability(ctx context.Context, builder string, testSuite string, bqClient *bigquery.Client, w io.Writer) error {
	// Grab all tests in the past 1 week.
	q := bqClient.Query(`
	WITH test_ids as (
		SELECT
			ds.test_id test_id,
			ds.variant_hash variant_hash,
			ANY_VALUE(ds.test_name) test_name,
			ANY_VALUE(SUBSTR((SELECT v FROM ds.testVariant.variant v WHERE v LIKE 'builder:%'), 9)) as builder,
			ANY_VALUE(SUBSTR((SELECT v FROM ds.testVariant.variant v WHERE v LIKE 'test_suite:%'), 12)) as test_suite,
		FROM chrome-trooper-analytics.test_results.daily_summary ds
		WHERE day BETWEEN
		  TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 181 DAY) AND
		  TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 1 DAY)
		GROUP BY ds.test_id, variant_hash
		HAVING
			("" = @query_test_suite OR test_suite = @query_test_suite)
			AND ("" = @query_builder OR builder = @query_builder)
	)

	SELECT
		tid.test_id TestId,
		tid.test_name TestName,
		IFNULL(tid.builder, "") as Builder,
		IFNULL(tid.test_suite, "") as TestSuite,
		(
		SELECT AS STRUCT
			IFNULL(SUM(ARRAY_LENGTH(dds.patchsets_with_failures)), 0) AS SixMonthFailCount,
			IFNULL(SUM(dds.run_count), 0) AS SixMonthRunCount,
			# Note the extra day is to account for lag between collecting the data and a new model being generated
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 31 DAY), ARRAY_LENGTH(dds.patchsets_with_failures), 0)), 0) AS OneMonthFailCount,
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 31 DAY), dds.run_count, 0)), 0) AS OneMonthRunCount,
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 8 DAY), ARRAY_LENGTH(dds.patchsets_with_failures), 0)), 0) AS OneWeekFailCount,
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 8 DAY), dds.run_count, 0)), 0) AS OneWeekRunCount,
		FROM chrome-trooper-analytics.test_results.daily_summary dds
		WHERE day BETWEEN
			TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 181 DAY) AND
			TIMESTAMP_SUB(TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY), INTERVAL 1 DAY)
			AND dds.test_id = tid.test_id
			AND dds.variant_hash = tid.variant_hash
		) as Stability,
	FROM test_ids tid
	`)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "query_builder", Value: builder},
		{Name: "query_test_suite", Value: testSuite},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return err
	}
	return writeStabilityFrom(ctx, w, it.Next)
}

func writeStabilityFrom(ctx context.Context, w io.Writer, source func(dest any) error) error {
	test := &proto.TestStability{}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Read the next row.
		switch err := source(test); {
		case err == iterator.Done:
			return ctx.Err()
		case err != nil:
			return err
		}

		jsonBytes, err := protojson.Marshal(test)
		if err != nil {
			return err
		}
		if _, err := w.Write(jsonBytes); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
}

const evalStabilityQuery = `
-- Unique test ids that were run during the time period of interest, ignoring
-- the day on which it happened. We can't just look at the tests in the summary
-- on that day since if the test didn't run on a day, it wouldn't appear in the
-- daily summary and we would get gaps
WITH test_ids as (
	SELECT
		ds.test_id test_id,
		ds.variant_hash variant_hash,
		ANY_VALUE(SUBSTR((SELECT v FROM ds.testVariant.variant v WHERE v LIKE 'builder:%'), 9)) as builder,
		ANY_VALUE(SUBSTR((SELECT v FROM ds.testVariant.variant v WHERE v LIKE 'test_suite:%'), 12)) as test_suite,
	FROM chrome-trooper-analytics.test_results.daily_summary ds
	GROUP BY ds.test_id, variant_hash
	HAVING
		("" = @query_test_suite OR test_suite = @query_test_suite)
		AND ("" = @query_builder OR builder = @query_builder)
),

-- The unique days for the period of interest (from 6 months before the earliest
-- day to the day before the latest day)
day_range as (
	SELECT *
	FROM
		UNNEST(GENERATE_TIMESTAMP_ARRAY(TIMESTAMP_SUB(TIMESTAMP_TRUNC(@start_day, DAY), INTERVAL 181 DAY), @end_day, INTERVAL 1 DAY)) AS day
)

-- Collect unique test variants and the stability info for that test over the
-- requested days
SELECT
	dt.day day,
	tid.test_id test_id,
	tid.variant_hash variant_hash,
	IFNULL(tid.builder, "") as Builder,
	IFNULL(tid.test_suite, "") as TestSuite,
	(
		# Aggregate the daily summary leading up to the day in the outer query (dt.day)
		SELECT AS STRUCT
			IFNULL(SUM(ARRAY_LENGTH(dds.patchsets_with_failures)), 0) AS six_month_fail_count,
			IFNULL(SUM(dds.run_count), 0) AS six_month_run_count,
			# Note the extra day is to account for lag between collecting the data and a new model being generated
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 31 DAY), ARRAY_LENGTH(dds.patchsets_with_failures), 0)), 0) AS one_month_fail_count,
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 31 DAY), dds.run_count, 0)), 0) AS one_month_run_count,
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 8 DAY), ARRAY_LENGTH(dds.patchsets_with_failures), 0)), 0) AS one_week_fail_count,
			IFNULL(SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 8 DAY), dds.run_count, 0)), 0) AS one_week_run_count,
		FROM chrome-trooper-analytics.test_results.daily_summary dds
		WHERE
			day BETWEEN
				TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 181 DAY) AND
				TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 1 DAY)
			AND dds.test_id = tid.test_id
			AND dds.variant_hash = tid.variant_hash
	) as stability,
FROM test_ids tid, day_range dt
WHERE
	dt.day BETWEEN @start_day AND @end_day
`
