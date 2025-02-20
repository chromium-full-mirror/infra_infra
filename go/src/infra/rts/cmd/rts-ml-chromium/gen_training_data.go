// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/flag"

	"go.chromium.org/infra/rts"
	"go.chromium.org/infra/rts/filegraph/git"
	"go.chromium.org/infra/rts/internal/chromium"
)

func cmdGenTrainingData(authOpt *auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `gen-training-data -out <path> -day <date> -builder <builder>`,
		ShortDesc: "Generate features and labels for ML model",
		LongDesc: text.Doc(`
			Generate features and labels for ML model

			Flags -out -from -to -file-graph-dir are required.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &genTrainingDataRun{}
			r.authOpt = authOpt
			r.Flags.StringVar(&r.fileGraphDir, "file-graph-dir", "", text.Doc(`
				Path to the directory with the model files.
				Normally it is coming from CIPD package "chromium/rts/model"
				and precomputed by "rts-chromium create-model" command.
			`))
			r.Flags.StringVar(&r.out, "out", "", text.Doc(`
				Filename to write csv training data to. If it already exists
				the file will be appended to.
			`))
			r.Flags.StringVar(&r.testSuite, "test-suite", "", text.Doc(`
				Test suite to get training data for.
			`))
			r.Flags.StringVar(&r.builder, "builder", "", text.Doc(`
				Builder to get training data for.
			`))
			r.Flags.Var(flag.Date(&r.startDate), "from", text.Doc(`
				Fetch results starting on this day. Stability information will
				be gathered based on this day.
				format: yyyy-mm-dd
			`))
			r.Flags.Var(flag.Date(&r.endDate), "to", text.Doc(`
				Fetch results up to this date. By default only the start-day
				will be gathered
				format: yyyy-mm-dd
			`))
			r.Flags.IntVar(&r.downSample, "down-sample", 1000, text.Doc(`
				The factor to down sample passes by to increase the number of
				failures. A value less than or equal to 0 will result in no
				down sampling
			`))
			r.Flags.IntVar(&r.maxClFailures, "max-cl-failures", 100, text.Doc(`
				The max failures in a single CL to include the CL. Default 100
			`))
			r.Flags.BoolVar(&r.ignorePassedBuilds, "ignore-passed-builds", false,
				"Whether or not to ignore results from builds that passed")
			r.Flags.BoolVar(&r.onlyTestFailures, "only-test-failures", false, text.Doc(`
				"Only return failure entries (intended for creating test sets to
				compare to RTS framework)
			`))
			return r
		},
	}
}

type genTrainingDataRun struct {
	baseCommandRun

	startDate          time.Time
	endDate            time.Time
	testSuite          string
	builder            string
	downSample         int
	maxClFailures      int
	ignorePassedBuilds bool
	onlyTestFailures   bool
	out                string
	fileGraphDir       string

	strategy git.SelectionStrategy
	file     *os.File

	authOpt       *auth.Options
	authenticator *auth.Authenticator
	http          *http.Client
}

func (r *genTrainingDataRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := r.ValidateFlags(); err != nil {
		return r.done(err)
	}

	ctx := cli.GetContext(a, r, env)

	if err := r.GenTrainData(ctx); err != nil {
		return r.done(err)
	}
	return 0
}

func (r *genTrainingDataRun) ValidateFlags() error {
	switch {
	case r.out == "":
		return errors.New("-out is required")
	case r.startDate.IsZero():
		return errors.New("-from is required")
	case r.endDate.IsZero():
		return errors.New("-to is required")
	case r.endDate.Before(r.startDate):
		return errors.New("-to must be after -from")
	case r.fileGraphDir == "":
		return errors.New("the -file-graph-dir is required")
	default:
		return nil
	}
}

// Generates training data and creates a csv file intended for a ml model
// to be trained on
func (r *genTrainingDataRun) GenTrainData(ctx context.Context) error {
	err := r.loadAffinityFileGraph(ctx)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(r.out), 0755)
	if err != nil {
		return err
	}
	file, err := os.Create(r.out)
	if err != nil {
		return err
	}
	csvWriter := csv.NewWriter(file)
	defer file.Close()
	r.writeCsvHeader(csvWriter)

	bqClient, err := chromium.NewBQClient(ctx, auth.NewAuthenticator(ctx, auth.InteractiveLogin, *r.authOpt))
	if err != nil {
		return errors.Annotate(err, "failed to create BigQuery client").Err()
	}

	currentClCount := 0
	fmt.Printf("Querying for entries %s through %s\n", r.startDate.String(), r.endDate.String())
	changelists, err := r.queryResults(ctx, bqClient)
	if err != nil {
		return errors.Annotate(err, "failed to retrieve query results").Err()
	}

	fmt.Printf("Calculating distances\n")
	for _, changelist := range changelists {
		r.calcChangelistDistances(changelist)

		currentClCount += 1
		if currentClCount%100 == 0 {
			fmt.Printf("Processed %d patchsets\n", currentClCount)
		}
		r.writeCsvRows(changelist, csvWriter)
	}
	csvWriter.Flush()

	return nil
}

// Calculates the git and file distances for a single changelist based
// on its list of changed files
func (r *genTrainingDataRun) calcChangelistDistances(cl bqChangelist) {
	// Find distances for the last patchset
	if cl.AffectedFilesCount > 100 {
		for _, row := range cl.TestResults {
			row.UseGitDistance = false
			row.UseFileDistance = false
		}
		return
	}

	var testFileToResults = make(map[string][]*testResult)
	for i := range cl.TestResults {
		testFileToResults[cl.TestResults[i].Filename] = append(testFileToResults[cl.TestResults[i].Filename], &cl.TestResults[i])
	}
	// Get the git based distance
	r.strategy.EdgeReader = &git.EdgeReader{
		ChangeLogDistanceFactor:     1,
		FileStructureDistanceFactor: 0,
	}
	foundFiles := 0
	r.strategy.RunQuery(cl.AffectedFiles, func(name string, af rts.Affectedness) (keepGoing bool) {
		if entry, ok := testFileToResults[name]; ok {
			for _, result := range entry {
				result.GitDistance = af.Distance
				result.UseGitDistance = true
			}
			foundFiles += 1

			if len(testFileToResults) == foundFiles {
				return false
			}
		}
		return true
	})
	// Get the file based distance
	foundFiles = 0
	r.strategy.EdgeReader = &git.EdgeReader{
		ChangeLogDistanceFactor:     0,
		FileStructureDistanceFactor: 1,
	}
	r.strategy.RunQuery(cl.AffectedFiles, func(name string, af rts.Affectedness) (keepGoing bool) {
		if entry, ok := testFileToResults[name]; ok {
			for _, result := range entry {
				result.FileDistance = af.Distance
				result.UseFileDistance = true
			}
			foundFiles += 1

			if len(testFileToResults) == foundFiles {
				return false
			}
		}
		return true
	})
}

// Writes the header to the csv file. This must stay in sync with writeCsvRows
func (r *genTrainingDataRun) writeCsvHeader(csvWriter *csv.Writer) error {
	return csvWriter.Write([]string{
		"Change",
		"ResultId",
		"Day",
		"TestName",
		"TestId",
		"OneWeekFailCount",
		"OneWeekRunCount",
		"OneMonthFailCount",
		"OneMonthRunCount",
		"SixMonthFailCount",
		"SixMonthRunCount",
		"GitDistance",
		"FileDistance",
		"Failed",
	})
}

// Writes all the rows for a single changelist
func (s *genTrainingDataRun) writeCsvRows(changelist bqChangelist, csvWriter *csv.Writer) {
	for _, testResult := range changelist.TestResults {
		gitDistance := ""
		if testResult.UseGitDistance {
			gitDistance = fmt.Sprintf("%v", testResult.GitDistance)
		}
		fileDistance := ""
		if testResult.UseFileDistance {
			fileDistance = fmt.Sprintf("%v", testResult.FileDistance)
		}
		row := []string{
			strings.ReplaceAll(changelist.ChangeId, ",", ";"),
			strings.ReplaceAll(testResult.ResultId, ",", ";"),
			strings.ReplaceAll(changelist.Day.String(), ",", ";"),
			strings.ReplaceAll(testResult.TestName, ",", ";"),
			strings.ReplaceAll(testResult.TestId, ",", ";"),
			fmt.Sprintf("%v", testResult.OneWeekFailCount),
			fmt.Sprintf("%v", testResult.OneWeekRunCount),
			fmt.Sprintf("%v", testResult.OneMonthFailCount),
			fmt.Sprintf("%v", testResult.OneMonthRunCount),
			fmt.Sprintf("%v", testResult.SixMonthFailCount),
			fmt.Sprintf("%v", testResult.SixMonthRunCount),
			gitDistance,
			fileDistance,
			fmt.Sprintf("%v", testResult.Failed),
		}
		csvWriter.Write(row)
	}
}

func (r *genTrainingDataRun) queryResults(ctx context.Context, bqClient *bigquery.Client) ([]bqChangelist, error) {
	q := bqClient.Query(filtersQuery)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "start_day", Value: r.startDate},
		{Name: "end_day", Value: r.endDate},
		{Name: "query_test_suite", Value: r.testSuite},
		{Name: "query_builder", Value: r.builder},
		{Name: "down_sample", Value: r.downSample},
		{Name: "ignore_passed_builds", Value: r.ignorePassedBuilds},
		{Name: "only_test_failures", Value: r.onlyTestFailures},
		{Name: "max_failures_per_ps", Value: r.maxClFailures},
		{Name: "max_cl_entries", Value: 1000},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}

	rows := []bqChangelist{}
	row := &bqChangelist{}
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Read the next row.
		switch err := it.Next(row); {
		case err == iterator.Done:
			return rows, ctx.Err()
		case err != nil:
			return nil, err
		}

		rows = append(rows, *row)
	}
}

func (r *genTrainingDataRun) loadAffinityFileGraph(ctx context.Context) error {
	fmt.Printf("Loading filegraph model\n")
	gitGraphDir := filepath.Join(r.fileGraphDir, "git-file-graph")
	if err := r.loadGraph(filepath.Join(gitGraphDir, "graph.fg")); err != nil {
		return errors.Annotate(err, "failed to load file graph").Err()
	}
	// Note: Not using the edge reader since each distance is created as a
	// separate feature
	return nil
}

func (r *genTrainingDataRun) loadGraph(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	r.strategy.Graph = &git.Graph{}
	return r.strategy.Graph.Read(bufio.NewReader(f))
}

type bqChangelist struct {
	ChangeId           string       `bigquery:"change_id"`
	FailedTestsCount   int          `bigquery:"failed_tests_count"`
	AffectedFilesCount int64        `bigquery:"affected_files_count"`
	AffectedFiles      []string     `bigquery:"affected_files"`
	TestResults        []testResult `bigquery:"test_results"`
	Day                time.Time    `bigquery:"day"`
}

type testResult struct {
	ResultId          string `bigquery:"result_id"`
	TestName          string `bigquery:"test_name"`
	TestId            string `bigquery:"test_id"`
	OneWeekFailCount  int64  `bigquery:"one_week_fail_count"`
	OneWeekRunCount   int64  `bigquery:"one_week_run_count"`
	OneMonthFailCount int64  `bigquery:"one_month_fail_count"`
	OneMonthRunCount  int64  `bigquery:"one_month_run_count"`
	SixMonthFailCount int64  `bigquery:"six_month_fail_count"`
	SixMonthRunCount  int64  `bigquery:"six_month_run_count"`
	Failed            bool   `bigquery:"failed"`
	Filename          string `bigquery:"file_name"`
	GitDistance       float64
	UseGitDistance    bool
	FileDistance      float64
	UseFileDistance   bool
}

// TODO(sshrimp): use a common query string to reduce maintenance with ml.go
const filtersQuery = `
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
		(@query_test_suite = "" OR @query_test_suite = test_suite)
		AND (@query_builder = "" OR @query_builder = builder)
),

-- The unique days for the period of interest (from 6 months before the earliest
-- day to the day before the latest day)
day_range as (
	SELECT *
	FROM
		UNNEST(GENERATE_TIMESTAMP_ARRAY(TIMESTAMP_SUB(TIMESTAMP_TRUNC(@start_day, DAY), INTERVAL 181 DAY), @end_day, INTERVAL 1 DAY)) AS day
),

-- Collect the stability info for each day to get how often the test was failing
-- leading up to that day
day_fail_rates as (
	SELECT
		dt.day day,
		tid.test_id test_id,
		tid.variant_hash variant_hash,
		tid.builder as builder,
		tid.test_suite as test_suite,
		(
			# Aggregate the daily summary leading up to the day in the outer query (dt.day)
			SELECT AS STRUCT
				IFNULL(SUM(ARRAY_LENGTH(dds.patchsets_with_failures)), 0) AS six_month_fail_count,
				IFNULL(SUM(dds.run_count), 0) AS six_month_run_count,
				# Note the extra day is to account for lag between collecting the data and a new model being generated
				SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 31 DAY), ARRAY_LENGTH(dds.patchsets_with_failures), 0)) AS one_month_fail_count,
				SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 31 DAY), dds.run_count, 0)) AS one_month_run_count,
				SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 8 DAY), ARRAY_LENGTH(dds.patchsets_with_failures), 0)) AS one_week_fail_count,
				SUM(IF(dds.day >= TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 8 DAY), dds.run_count, 0)) AS one_week_run_count,
			FROM chrome-trooper-analytics.test_results.daily_summary dds
			WHERE day BETWEEN
				TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 181 DAY) AND
				TIMESTAMP_SUB(TIMESTAMP_TRUNC(dt.day, DAY), INTERVAL 1 DAY)
				AND dds.test_id = tid.test_id
				AND dds.variant_hash = tid.variant_hash
		) as stability,
	FROM test_ids tid, day_range dt
	WHERE
		dt.day BETWEEN @start_day AND @end_day
),

-- Builder job id's and their associated attempt info (change + ps)
tryjobs AS (
	SELECT
		TIMESTAMP_TRUNC(partition_time, DAY) day,
		b.id,
		ps.change,
		ps.earliest_equivalent_patchset as patchset,
		partition_time as ps_approx_timestamp,
	FROM commit-queue.chromium.attempts a, a.gerrit_changes ps, a.builds b
	WHERE
		TIMESTAMP_TRUNC(partition_time, DAY) BETWEEN TIMESTAMP_TRUNC(@start_day, DAY) AND TIMESTAMP_TRUNC(@end_day, DAY)
),

-- The actual build jobs to be combined with their attempt. This will include
-- changed files to be used to determin distances
bb_tryjobs AS (
	SELECT
		id,
		status,
		IF(JSON_EXTRACT(b.output.properties, "$.affected_files.total_count") IS NULL, 0, CAST(CAST(JSON_EXTRACT(b.output.properties, "$.affected_files.total_count") AS FLOAT64) AS INT)) affected_files_count,
		ARRAY(SELECT REGEXP_REPLACE(REPLACE(file, '"', ""), r'^src/', '//') FROM UNNEST(JSON_EXTRACT_ARRAY(b.output.properties, "$.affected_files.first_100")) file) affected_files
	FROM cr-buildbucket.chromium.builds b
	WHERE
		create_time BETWEEN TIMESTAMP_SUB(@start_day, INTERVAL 1 DAY) AND TIMESTAMP_ADD(@end_day, INTERVAL 1 DAY)
		AND builder.bucket = 'try'
		# Exclude experimental builders because they may fail for reasons
		# unrelated to the CL, and are not required for the CL to land.
		AND STRUCT('cq_experimental', 'true') NOT IN UNNEST(b.tags)
		AND (not @ignore_passed_builds or b.status = "FAILURE")
),

-- The combined jobs, their status, and the affected file information
tryjobs_with_status AS (
	SELECT t.*,
	bb.status,
	bb.affected_files,
	bb.affected_files_count
	FROM tryjobs t
	JOIN bb_tryjobs bb USING (id)
),

-- The base test results and variant information
test_results_base AS (
	SELECT
		tr.result_id,
		tr.name test_name,
		tr.test_id,
		CAST(REGEXP_EXTRACT(exported.id, r'build-(\d+)') as INT64) as build_id,
		IF(tr.test_metadata.location.file_name IS NULL, "", REGEXP_REPLACE(tr.test_metadata.location.file_name, r'^src/', r'//'))  file_name,
		(SELECT v.value FROM tr.variant v where v.key = 'test_suite') AS test_suite,
		(SELECT v.value FROM tr.variant v where v.key = 'builder') AS builder,
		variant_hash,
		expected,
		exonerated,
		status,
		duration,
	FROM chrome-luci-data.chromium.try_test_results tr
	-- Read prev-day and next-day results too to ensure that we have ALL
	-- results of a given CQ attempt.
	WHERE partition_time BETWEEN TIMESTAMP_SUB(@start_day, INTERVAL 1 DAY) AND TIMESTAMP_ADD(@end_day, INTERVAL 1 DAY)
		# Exclude third-party tests (except Web Tests) because they test code
		# which isn't in src.git.
		# As of January 2021, this excludes ~2% of test results.
		AND (
			test_metadata.location.file_name NOT LIKE '%/third_party/%'
			OR test_metadata.location.file_name LIKE '//third_party/blink/%'
		)
),

-- Group all test results by patchset, test_id and variant_hash
-- in order to analyze individual test variants in each patchset,
-- and in particular exclude flaky tests.
test_variants_per_ps AS (
	SELECT
		# The result_id is only used to get a unique hash. Min to ensure consistency
		MIN(result_id) result_id,
		ANY_VALUE(test_name) test_name,
		ANY_VALUE(test_suite) test_suite,
		ANY_VALUE(builder) builder,
		ANY_VALUE(file_name) file_name,
		ANY_VALUE(affected_files) affected_files,
		ANY_VALUE(affected_files_count) affected_files_count,
		test_id,
		change,
		patchset,
		variant_hash,
		ANY_VALUE(day) day,
		LOGICAL_OR(expected) AND LOGICAL_OR(NOT expected) AS flake,

		# Sometimes ResultDB table misses data. For example, if a test
		# flaked, the table might miss the pass, and it might look like the test
		# has failed. Also sometimes builds are incomplete because they
		# infra-failed or were canceled midway, and the test results do not
		# represent the whole picture. In particular, CANCELED might mean that the
		# "without patch" part didn't finish and test results were not properly
		# exonerated.
		# Thus ensure that the build has failed too.
		LOGICAL_AND(NOT expected) AND LOGICAL_AND(tj.status = 'FAILURE') all_unexpected,

		ANY_VALUE(ps_approx_timestamp) AS ps_approx_timestamp,
	FROM tryjobs_with_status tj
	JOIN test_results_base tr ON tj.id = tr.build_id
	WHERE not exonerated  AND tr.status != 'SKIP' -- not needed for RTS purposes
	GROUP BY change, patchset, test_id, variant_hash
),

-- Combines the test result information (the label) with the test stability
-- information (the features)
test_entries_with_stability AS (
	SELECT
		fr.day day,
		rdb.result_id,
		FORMAT("https://crrev.com/c/%d/%d", rdb.change, rdb.patchset) as change_id,
		test_name,
		rdb.variant_hash,
		rdb.test_id,
		rdb.file_name,
		affected_files,
		affected_files_count,
		fr.stability.six_month_fail_count,
		fr.stability.six_month_run_count,
		fr.stability.one_month_fail_count,
		fr.stability.one_month_run_count,
		fr.stability.one_week_fail_count,
		fr.stability.one_week_run_count,
		rdb.all_unexpected as failed,
	FROM
		test_variants_per_ps rdb
		LEFT JOIN day_fail_rates fr
		ON rdb.test_id = fr.test_id AND rdb.variant_hash = fr.variant_hash AND rdb.day = fr.day
	WHERE
		("" = @query_test_suite OR rdb.test_suite = @query_test_suite)
		AND ("" = @query_builder OR rdb.builder = @query_builder)
		AND fr.stability.six_month_run_count IS NOT NULL
		AND fr.stability.six_month_run_count > 0
		# Remove passes if the option is set
		AND (NOT @only_test_failures OR rdb.all_unexpected)
		# Downsample the non-failures if enabled
		AND (@down_sample <= 0
			OR rdb.all_unexpected
			OR MOD(FARM_FINGERPRINT(rdb.result_id), @down_sample) = 0)
)

-- Aggregate the ML examples (rows) into an array to keep from duplicating the
-- attempt info
SELECT
	te.change_id,
	COUNTIF(te.failed) as failed_tests_count,
	ANY_VALUE(te.day) as day,
	ANY_VALUE(te.affected_files) as affected_files,
	ANY_VALUE(te.affected_files_count) as affected_files_count,
	ARRAY_AGG(STRUCT(
		te.result_id as result_id,
		te.test_name as test_name,
		te.test_id as test_id,
		te.six_month_fail_count as six_month_fail_count,
		te.six_month_run_count as six_month_run_count,
		te.one_month_fail_count as one_month_fail_count,
		te.one_month_run_count as one_month_run_count,
		te.one_week_fail_count as one_week_fail_count,
		te.one_week_run_count as one_week_run_count,
		te.failed as failed,
		te.file_name as file_name
	)
	# Order by the failed status to avoid cutting out failures
	ORDER BY NOT te.failed LIMIT @max_cl_entries) as test_results
FROM test_entries_with_stability te
GROUP BY change_id
HAVING
	(NOT @ignore_passed_builds OR failed_tests_count > 0)
	AND failed_tests_count < @max_failures_per_ps
`
