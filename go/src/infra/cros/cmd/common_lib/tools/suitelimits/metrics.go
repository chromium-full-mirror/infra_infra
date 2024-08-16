// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package suitelimits

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// trackingMetric is the entry to be used when outputting logs for the suite
// tracking feature. Each of these fields corresponds to a column in the
// resulting csv.
type trackingMetric struct {
	requestKey      string
	suiteName       string
	taskName        string
	taskBBID        int64
	lastSeen        time.Time
	currentlySeenAt time.Time
	delta           time.Duration
}

var (
	metricsChan     = make(chan trackingMetric)
	incrementalLogs = map[string][][]string{}
	finalTotalLogs  = [][]string{}
)

// LogMetrics captures and filters the logs passed in through the logChannel.
// Once the channel is closed it writes all the data to individual CSVs per suite.
func LogMetrics(ctx context.Context) {
	// Filtered suite logs.
	suiteLogs := make(map[string][]trackingMetric)

	// Filter all the logs. Range breaks once we close the channel at the end of
	// the CTP run.
	for metric := range metricsChan {
		if list, ok := suiteLogs[metric.suiteName]; ok {
			suiteLogs[metric.suiteName] = append(list, metric)
		} else {
			fmt.Printf("Suite Metrics: Log created for request %s\n", metric.suiteName)
			suiteLogs[metric.suiteName] = []trackingMetric{metric}
		}
	}

	flushMetrics(suiteLogs)
}

// flushMetrics writes all logs to local csv files.
func flushMetrics(suiteLogs map[string][]trackingMetric) {
	incrementalLogs = flushIncrementalUpdates(suiteLogs)

	finalTotalLogs = flushTotals()
}

// flushIncrementalUpdates converts the received metric updates to csv form and
// writes to local storage.
func flushIncrementalUpdates(suiteLogs map[string][]trackingMetric) map[string][][]string {
	incrementalMap := map[string][][]string{}

	// Flush the incremental updates
	for suiteName, logs := range suiteLogs {
		// This 2D array is what the CSV needs to write to a file. The first row is
		// the headers that we are building from the struct type trackingMetric.
		data := [][]string{
			{"requestKey", "suiteName", "taskName", "lastSeen", "currentlySeenAt", "delta"},
		}

		// Create the data array for the CSV writer.
		fmt.Printf("Suite Metrics: FLUSHING %d logs\n", len(suiteLogs))
		for _, log := range logs {
			data = append(data, []string{
				log.requestKey,
				log.suiteName,
				log.taskName,
				log.lastSeen.UTC().String(),
				log.currentlySeenAt.UTC().String(),
				strconv.FormatFloat(log.delta.Seconds(), 'f', 2, 64),
			})
		}
		incrementalMap[suiteName] = data
	}

	return incrementalMap
}

// flushTotals writes the final per suite execution totals to a CSV.
func flushTotals() [][]string {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	// This is stored in CSV format, the following are the headers and their
	// respective types before casting to string.
	//
	// 	suiteName                   string
	// 	taskName                    string
	// 	totalTestExecutionSeconds   float64
	// 	exceededExecutionLimit      bool
	// 	exceptionGranted			bool
	data := [][]string{
		{"requestKey", "taskName", "suiteName", "totalTestExecutionSeconds", "exceededExecutionLimit", "exceptionGranted"},
	}

	// Create the data array for the CSV writer.
	for requestKey, hwMap := range suiteLimitsTracker.cache {
		for taskName, entry := range hwMap {
			// This line was added because some testing doesn't properly fill in the taskset map.
			exceptionGranted := entry.exemptionGranted
			data = append(data, []string{
				// Field: suiteName
				string(requestKey),

				// Field: taskName
				string(taskName),

				// Field: suiteName
				entry.suiteName,

				// Field: totalTestExecutionSeconds
				strconv.FormatFloat(float64(entry.totalDUTHours.Seconds()), 'f', 2, 64),

				// Field: exceededExecutionLimit
				strconv.FormatBool(entry.totalDUTHours.Seconds() > DutHourMaximumSeconds),

				// Field: exceptionGranted
				strconv.FormatBool(exceptionGranted),
			})

		}
	}

	return data
}

func GetIncrementalLogs() map[string][][]string {
	return incrementalLogs
}

func GetTotalsLogs() [][]string {
	return finalTotalLogs
}

func CloseMetricChan() {
	close(metricsChan)
}
