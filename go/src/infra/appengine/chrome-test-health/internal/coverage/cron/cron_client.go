// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/appengine/chrome-test-health/datastorage"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage/entities"
)

const (
	chromiumHost = "chromium-review.googlesource.com"
)

type CronClient struct {
	// Refers to Findit's cloud project
	FinditCloudProject string
	// Refers to Chrome-test-health's cloud project
	ChromeTestHealthCloudProject string
	// References to Findit-for-me's datastore
	coverageV1DsClient datastorage.IDataClient
	// References to Chrome-test-health's datastore
	coverageV2DsClient datastorage.IDataClient
}

type IncrementalCoverageData struct {
	CoveredFiles int
	TotalFiles   int
	IsDir        bool
}

func NewClient(ctx context.Context, finditCloudProject string, chromeTestHealthCloudProject string) (*CronClient, error) {
	c := CronClient{
		FinditCloudProject:           finditCloudProject,
		ChromeTestHealthCloudProject: chromeTestHealthCloudProject,
	}

	covV1DsClient, err := datastorage.NewDataStoreClient(ctx, finditCloudProject)
	if err != nil {
		logging.Errorf(ctx, "Error connecting to %s", finditCloudProject)
		return nil, coverage.ErrInternalServerError
	}
	c.coverageV1DsClient = covV1DsClient

	covV2DsClient, err := datastorage.NewDataStoreClient(ctx, chromeTestHealthCloudProject)
	if err != nil {
		logging.Errorf(ctx, "Error connecting to %s", chromeTestHealthCloudProject)
		return nil, coverage.ErrInternalServerError
	}
	c.coverageV2DsClient = covV2DsClient

	return &c, nil
}

// UpdatePresubmitData fetches presubmit data for the last 24 hours, processes
// it and stores it to the datastore
func (c *CronClient) UpdatePresubmitData(ctx context.Context) error {
	presubmitData, err := c.getPresubmitReportsOneDay(ctx)
	if err != nil {
		logging.Errorf(ctx, "Error fetching Presubmit Reports")
		return err
	}
	patchsetMap := c.getMaxPatchsetToChangeMap(presubmitData)

	for _, data := range presubmitData {
		allTestsData := c.splitSinglePresubmitData(&data, patchsetMap, false)
		err := c.createCqSummaryData(ctx, data.UpdateTimestamp, data.Change, data.Patchset, false, allTestsData)
		if err != nil {
			logging.Errorf(ctx, "Error storing CQ Cov data for this Presubmit Report: %s", err)
			return err
		}

		unitTestsData := c.splitSinglePresubmitData(&data, patchsetMap, true)
		err = c.createCqSummaryData(ctx, data.UpdateTimestamp, data.Change, data.Patchset, true, unitTestsData)
		if err != nil {
			logging.Errorf(ctx, "Error storing CQ Cov data for this Presubmit Report: %s", err)
			return err
		}
	}

	return nil
}

// getPresubmitReportsOneDay gets the Presubmit Coverage Reports from the
// datastore for the last 24 hours
func (c *CronClient) getPresubmitReportsOneDay(
	ctx context.Context,
) ([]entities.PresubmitCoverageData, error) {
	records := []entities.PresubmitCoverageData{}
	queryFilters := []datastorage.QueryFilter{
		{Field: "cl_patchset.server_host", Operator: "=", Value: chromiumHost},
		{Field: "update_timestamp", Operator: ">", Value: time.Now().Add(-time.Hour * 24)},
	}

	if err := c.coverageV1DsClient.Query(
		ctx, &records, "PresubmitCoverageData",
		queryFilters, nil, 0,
	); err != nil {
		logging.Errorf(ctx, "PresubmitCoverageData: %w", err)
		return nil, coverage.ErrInternalServerError
	}

	return records, nil
}

// getMaxPatchsetToChangeMap returns a map. The key is the CL number and value is
// the latest patchset number for which we have presubmit reports
func (c *CronClient) getMaxPatchsetToChangeMap(
	presubmitData []entities.PresubmitCoverageData,
) map[int64]int64 {
	highestPatchsetMap := make(map[int64]int64)
	for _, data := range presubmitData {
		change := data.Change
		patchset := data.Patchset
		if _, ok := highestPatchsetMap[change]; !ok {
			highestPatchsetMap[change] = patchset
		}

		if patchset > highestPatchsetMap[change] {
			highestPatchsetMap[change] = patchset
		}
	}
	return highestPatchsetMap
}

// splitSinglePresubmitData splits the incremental coverage percentage list
// for the given Presubmit entity into individual elements comprising of
// file/dir path and their incremental coverage statistics.
func (c *CronClient) splitSinglePresubmitData(presubmit *entities.PresubmitCoverageData, maxPatchsetMap map[int64]int64, isUnitTest bool) map[string]IncrementalCoverageData {
	// Changelog number
	change := presubmit.Change
	// Patchset number
	patchset := presubmit.Patchset
	// Ignore if patchset is not latest
	if maxPatchsetMap[change] != patchset {
		return nil
	}

	pathData := map[string]IncrementalCoverageData{}
	// For this presubmit data, IncrementalPercentages is an array that stores
	// the files changed along with their incremental coverage stats
	percentages := presubmit.IncrementalPercentages
	if isUnitTest {
		percentages = presubmit.IncrementalPercentagesUnit
	}

	// Go over each changed file
	for _, pathStats := range percentages {
		path := pathStats.Path
		percentage := (pathStats.CoveredLines * 100) / pathStats.TotalLines
		// For each path walk over the path in reverse and store the files
		// changed & covered.
		coveredFiles := 0
		if percentage >= 70 {
			coveredFiles = 1
		}

		pathData[path] = IncrementalCoverageData{
			CoveredFiles: coveredFiles, TotalFiles: 1, IsDir: false,
		}
		curr := path
		parent := getDir(path)
		for parent != curr {
			if p, ok := pathData[parent]; ok {
				incData := pathData[parent]
				incData.CoveredFiles = p.CoveredFiles + pathData[path].CoveredFiles
				incData.TotalFiles = p.TotalFiles + pathData[path].TotalFiles
				incData.IsDir = true
				pathData[parent] = incData
			} else {
				pathData[parent] = IncrementalCoverageData{
					CoveredFiles: pathData[path].CoveredFiles,
					TotalFiles:   pathData[path].TotalFiles,
					IsDir:        true,
				}
			}
			curr = parent
			parent = getDir(curr)
		}
	}

	// Return the pathData map. Examples of this map are shown below:
	// {
	//    "//dir1/dir2/file.cc" => {CoveredFiles: 1, TotalFiles: 1, IsDir: false}
	//    "//dir1/dir2/" => {CoveredFiles: 1, TotalFiles: 2, IsDir: true}
	// }
	return pathData
}

// getDir takes in a directory path in the format: "//a/b/" or "//a/b/file.ext"
// returns the parent directory of the path. For the above examples this
// function returns "//a/" and "//a/b/" respectively
// We are implementing this ourselves because the os lib function is OS
// specific, and our paths being UNIX format does not work for Windows.
func getDir(path string) string {
	if path == "//" {
		return "//"
	}

	parts := strings.Split(path, "/")
	partsLen := len(parts)
	// Path is a filepath
	if parts[partsLen-1] != "" {
		// Path is in format: "//file.c"
		if partsLen == 3 {
			return "//"
		}

		// Path is in format: "//dir/file.c"
		return fmt.Sprintf("//%s/", strings.Join(parts[2:partsLen-1], "/"))
	}

	// Path is in format: "//dir/"
	if partsLen == 4 {
		return "//"
	}

	// Path is in format: "//dir1/dir2/"
	return fmt.Sprintf("//%s/", strings.Join(parts[2:partsLen-2], "/"))
}

// createCqSummaryData takes in the new set of paths for which
// incremental code coverage data need to be created on the datastore
// This function also takes in few other arguments like timestamp, CL change
// number and the patchset to pinpoint the exact change where the files/dirs were
// changed
func (c *CronClient) createCqSummaryData(ctx context.Context, timestamp time.Time, change int64, patchset int64, isUnitTest bool, pathData map[string]IncrementalCoverageData) error {
	newSummaryCovData := []*entities.CQSummaryCoverageData{}
	keys := []*datastore.Key{}

	for path, data := range pathData {
		dataType := "files"
		if data.IsDir {
			dataType = "dirs"
		}

		// By default add the given file path and it's coverage data to the final set
		// of CQ summary data to be created
		newSummaryCovData = append(newSummaryCovData, &entities.CQSummaryCoverageData{
			Timestamp:         timestamp,
			Change:            change,
			Patchset:          patchset,
			IsUnitTest:        isUnitTest,
			Path:              path,
			DataType:          dataType,
			FilesCovered:      int64(data.CoveredFiles),
			TotalFilesChanged: int64(data.TotalFiles),
		})

		isUnitTestsLiteral := "all"
		if isUnitTest {
			isUnitTestsLiteral = "unit"
		}

		key := datastore.NameKey(
			"CQSummaryCoverageData",
			fmt.Sprintf("%d$%s$%s$%s", change, dataType, path, isUnitTestsLiteral),
			nil,
		)
		keys = append(keys, key)
	}

	// Put all the entities (in the final-set) in the datastore
	// If an entity with the key exists, it will update that entity
	err := c.coverageV2DsClient.BatchPut(ctx, newSummaryCovData, keys)
	if err != nil {
		logging.Errorf(ctx, "Error putting entities in datastore: %s", err)
		return err
	}
	return nil
}
