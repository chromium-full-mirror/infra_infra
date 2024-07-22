// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package suitelimits

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.chromium.org/luci/common/logging"
)

// trackingMetric is the entry to be used when outputting logs for the suite
// tracking feature. Each of these fields corresponds to a column in the
// resulting csv.
type trackingMetric struct {
	suiteName       string
	taskName        string
	lastSeen        time.Time
	currentlySeenAt time.Time
	delta           time.Duration
	completed       bool
}

const (
	// Directory structure for the metrics.
	logDirectory         = "metric_logs/"
	perSuiteLogDirectory = logDirectory + "per_suites"
	totalsLogDirectory   = logDirectory + "totals"
)

// createDirectoryStructure makes all the directories to be used in the metrics logging.
func createDirectoryStructure(workingDir string) error {
	// Make parent and per suites dir.
	path := filepath.Join(workingDir, perSuiteLogDirectory)
	err := os.MkdirAll(path, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Make the totals directory.
	path = filepath.Join(workingDir, totalsLogDirectory)
	err = os.MkdirAll(path, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

// writeToCSV wrap all the logic needed to write the data to a csv.
func writeToCSV(data [][]string, fileLocation string) error {
	// Check the if the file has been made before. Make it if not.
	file, err := os.Create(fileLocation)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Creates the interface which write to the file we just created.
	writer := csv.NewWriter(file)

	// Write data to file.
	err = writer.WriteAll(data)
	if err != nil {
		return err
	}

	return nil
}

// flushMetrics writes the given logs to the given CSV.
func flushMetrics(ctx context.Context, destPath string, suiteLogs []trackingMetric) error {
	// This 2D array is what the CSV needs to write to a file. The first row is
	// the headers that we are building from the struct type trackingMetric.
	data := [][]string{
		{"suiteName", "taskName", "lastSeen", "currentlySeenAt", "delta", "completed"},
	}

	// Create the data array for the CSV writer.
	logging.Infof(ctx, "Suite Metrics: FLUSHING %d logs @ %s\n", len(suiteLogs), destPath)
	for _, log := range suiteLogs {
		data = append(data, []string{
			log.suiteName,
			log.taskName,
			log.lastSeen.UTC().String(),
			log.currentlySeenAt.UTC().String(),
			strconv.FormatFloat(log.delta.Seconds(), 'f', 2, 64),
			strconv.FormatBool(log.completed),
		})
	}

	err := writeToCSV(data, destPath)
	return err
}

// logMetrics captures and filters the logs passed in through the logChannel.
// Once the channel is closed it writes all the data to individual CSVs per suite.
func logMetrics(ctx context.Context, logChan chan trackingMetric) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	workingDir := filepath.Dir(exePath)

	err = createDirectoryStructure(workingDir)
	if err != nil {
		return fmt.Errorf("error making directory structure for metrics logging: %s", err.Error())
	}

	// Filtered suite logs.
	suiteLogs := make(map[string][]trackingMetric)

	// Filter all the logs. Range breaks once we close the channel at the end of
	// the CTP run.
	for metric := range logChan {
		if list, ok := suiteLogs[metric.suiteName]; ok {
			suiteLogs[metric.suiteName] = append(list, metric)
		} else {
			logging.Infof(ctx, "Suite Metrics: Log created for suite %s\n", metric.suiteName)
			suiteLogs[metric.suiteName] = []trackingMetric{metric}
		}
	}

	// Launch all CSV writes in parallel.
	for suite, logs := range suiteLogs {
		logging.Infof(ctx, "Flushing metrics for suite: %s\n", suite)
		err := flushMetrics(ctx, filepath.Join(workingDir, perSuiteLogDirectory, suite+".csv"), logs)
		if err != nil {
			return err
		}

	}

	return nil
}

// logTotals writes the final per suite execution totals to a CSV.
func logTotals(ctx context.Context, workingDirectory string, taskSets map[string]*suiteLimitEntry) error {
	destPath := filepath.Join(workingDirectory, totalsLogDirectory, "final.csv")
	// This 2D array is what the CSV needs to write to a file. The first row is
	// the headers that will make up the columns of the CSV:
	//
	// 	requestName                 string
	// 	totalTestExecutionSeconds   float64
	// 	exceededExecutionLimit      bool
	// 	exceptionGranted			bool
	data := [][]string{
		{"requestName", "totalTestExecutionSeconds", "exceededExecutionLimit", "exceptionGranted"},
	}

	// Create the data array for the CSV writer.
	for requestName, entry := range suiteLimitsTracker.cache {
		// This line was added because some testing doesn't properly fill in the taskset map.
		exceptionGranted := taskSets[requestName].exemptionGranted
		data = append(data, []string{
			// Field: suiteName
			requestName,

			// Field: totalTestExecutionSeconds
			strconv.FormatFloat(float64(entry.totalDUTHours.Seconds()), 'f', 2, 64),

			// Field: exceededExecutionLimit
			strconv.FormatBool(entry.totalDUTHours.Seconds() > dutHourMaximumSeconds),

			// Field: exceptionGranted
			strconv.FormatBool(exceptionGranted),
		})
	}

	logging.Infof(ctx, "Writing totals to %s", destPath)
	return writeToCSV(data, destPath)
}
