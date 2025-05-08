// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	structpb "github.com/golang/protobuf/ptypes/struct"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/appengine/chrome-test-health/api"
	"go.chromium.org/infra/appengine/chrome-test-health/datastorage"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage/entities"
)

const (
	chromiumProject    = "chromium/src"
	chromiumServerHost = "chromium.googlesource.com"
	chromiumRef        = "refs/heads/main"
)

type Client struct {
	// Refers to findit's cloud project
	FinditCloudProject string
	// Refers to chrome-test-health's cloud project
	ChromeTestHealthCloudProject string
	// References to findit-for-me's datastore
	coverageV1DsClient datastorage.IDataClient
	// References to chrome-test-health's datastore
	coverageV2DsClient datastorage.IDataClient
}

func (c *Client) Init(ctx context.Context) error {
	covV1DsClient, err := datastorage.NewDataStoreClient(ctx, c.FinditCloudProject)
	if err != nil {
		logging.Errorf(ctx, "Error connecting to %s", c.FinditCloudProject)
		return ErrInternalServerError
	}
	c.coverageV1DsClient = covV1DsClient

	covV2DsClient, err := datastorage.NewDataStoreClient(ctx, c.ChromeTestHealthCloudProject)
	if err != nil {
		logging.Errorf(ctx, "Error connecting to %s", c.ChromeTestHealthCloudProject)
		return ErrInternalServerError
	}
	c.coverageV2DsClient = covV2DsClient

	return nil
}

type CoveragePerDate struct {
	date    string
	covered float64
	total   float64
}

// getProjectConfig extracts out the code coverage default settings from the
// fetched FinditConfig entity for the given project.
func (c *Client) getProjectConfig(
	ctx context.Context,
	finditConfig *entities.FinditConfig,
	project string,
	config *api.GetProjectDefaultConfigResponse,
) error {
	code_cov_settings := map[string]any{}
	err := json.Unmarshal(finditConfig.CodeCoverageSettings, &code_cov_settings)
	if err != nil {
		logging.Errorf(ctx, "Failed to unmarshall CodeCoverageSettings: %s", err)
		return ErrInternalServerError
	}

	defaultPostSubmitConfig := code_cov_settings["default_postsubmit_report_config"]
	if defaultPostSubmitConfig == nil {
		logging.Errorf(ctx, "Missing default_postsubmit_report_config from FinditConfig")
		return ErrInternalServerError
	}

	projectConfig := defaultPostSubmitConfig.(map[string]any)[project]
	if projectConfig == nil {
		logging.Errorf(ctx, "Missing config for project %s", project)
		return ErrInternalServerError
	}
	projectConfig = projectConfig.(map[string]any)

	config.GitilesHost = projectConfig.(map[string]any)["host"].(string)
	config.GitilesProject = projectConfig.(map[string]any)["project"].(string)
	config.GitilesRef = projectConfig.(map[string]any)["ref"].(string)

	return nil
}

func (c *Client) getBuilderOptions(
	ctx context.Context,
	luciProject string,
	host string,
	project string,
	finditConfig *entities.FinditConfig,
	config *api.GetProjectDefaultConfigResponse,
) error {
	code_cov_settings := map[string]any{}
	err := json.Unmarshal(finditConfig.CodeCoverageSettings, &code_cov_settings)
	if err != nil {
		logging.Errorf(ctx, "Failed to unmarshall CodeCoverageSettings: %s", err)
		return ErrInternalServerError
	}

	postsubmitPlatformInfoMap := code_cov_settings["postsubmit_platform_info_map"]
	if postsubmitPlatformInfoMap == nil {
		logging.Errorf(ctx, "Missing postsubmit_platform_info_map from FinditConfig")
		return ErrInternalServerError
	}

	platformListForProject := postsubmitPlatformInfoMap.(map[string]any)[luciProject]

	var builderConfigDetails []*api.BuilderConfig
	for platform, builderConfigDetail := range platformListForProject.(map[string]any) {
		bucket := builderConfigDetail.(map[string]any)["bucket"].(string)
		builder := builderConfigDetail.(map[string]any)["builder"].(string)
		postsubmitReport := entities.PostsubmitReport{}
		err := postsubmitReport.Filter(ctx, c.coverageV1DsClient, project, host, bucket, builder)
		if err != nil {
			continue
		}

		builderConfigDetails = append(builderConfigDetails, &api.BuilderConfig{
			Platform:       platform,
			Bucket:         bucket,
			Builder:        builder,
			UiName:         builderConfigDetail.(map[string]any)["ui_name"].(string),
			LatestRevision: postsubmitReport.GitilesCommitRevision,
		})
	}

	config.BuilderConfig = builderConfigDetails
	return nil
}

func (c *Client) getModifedBuilder(builder string, unitTestsOnly *bool) string {
	if unitTestsOnly == nil || !(*unitTestsOnly) {
		return builder
	}
	return fmt.Sprintf("%s_unit", builder)
}

// GetProjectDefaultConfig fetches the latest version of FinditConfig from the
// datastore and returns the desired configuration extracted from the entity.
func (c *Client) GetProjectDefaultConfig(
	ctx context.Context,
	req *api.GetProjectDefaultConfigRequest,
) (*api.GetProjectDefaultConfigResponse, error) {
	project := req.LuciProject
	finditConfig := &entities.FinditConfig{}
	if err := finditConfig.Get(ctx, c.coverageV1DsClient); err != nil {
		logging.Errorf(ctx, "%s", err.Error())
		return nil, ErrInternalServerError
	}

	response := &api.GetProjectDefaultConfigResponse{}
	if err := c.getProjectConfig(ctx, finditConfig, project, response); err != nil {
		return nil, err
	}

	if err := c.getBuilderOptions(ctx, project, response.GitilesHost,
		response.GitilesProject, finditConfig, response); err != nil {
		return nil, err
	}

	return response, nil
}

// GetCoverageSummary fetches the code coverage metrics/percentages for the specified
// configuration including the path or component list.
// The path param here can be a dir/file path like //foo/foo1/foo2/.
// Components param should be be a list of monorail components like ["C1>C2", "C3"]
// This endpoint accepts either path or component not both.
func (c *Client) GetCoverageSummary(ctx context.Context, req *api.GetCoverageSummaryRequest) (*api.GetCoverageSummaryResponse, error) {
	host := req.GitilesHost
	project := req.GitilesProject
	ref := req.GitilesRef
	revision := req.GitilesRevision
	path := req.Path
	components := req.Components
	unitTestsOnly := req.UnitTestsOnly
	bucket := req.Bucket
	builder := c.getModifedBuilder(req.Builder, &unitTestsOnly)

	var dataType string
	var nodes []string
	if path != "" {
		dataType = "dirs"
		nodes = append(nodes, path)
	} else {
		dataType = "components"
		nodes = components
	}

	var combinedSummary = [](*structpb.Struct){}
	for _, node := range nodes {
		// Fetch the SummaryCoverageReport entity for the given configuration.
		summary := entities.SummaryCoverageData{}
		err := summary.Get(ctx, c.coverageV1DsClient, host, project, ref, revision, dataType, node, bucket, builder)
		if err != nil {
			logging.Errorf(ctx, "Error fetching SummaryCoverageData: %s", err)
			return nil, ErrInternalServerError
		}

		// Take the Data field out of the summary. The data field is compressed using
		// zlib, the following code decompresses that data and puts it into a struct.
		coverageDetailsStruct := structpb.Struct{}
		err = getStructFromCompressedData(summary.Data, &coverageDetailsStruct)
		if err != nil {
			logging.Errorf(ctx, "Unable to decompress the data: %s", err)
			return nil, ErrInternalServerError
		}

		combinedSummary = append(combinedSummary, &coverageDetailsStruct)
	}

	return &api.GetCoverageSummaryResponse{
		Summary: combinedSummary,
	}, nil
}

// getCoverageReportsForLastYear fetches the absolute code coverage reports
// for the last 365 days. These reports are specific to builder configuration
// which is supplied to this function as builder and bucket.
func (c *Client) getCoverageReportsForLastYear(
	ctx context.Context,
	bucket string,
	builder string,
) ([]entities.PostsubmitReport, error) {
	records := []entities.PostsubmitReport{}
	queryFilters := []datastorage.QueryFilter{
		{Field: "gitiles_commit.project", Operator: "=", Value: chromiumProject},
		{Field: "gitiles_commit.server_host", Operator: "=", Value: chromiumServerHost},
		{Field: "bucket", Operator: "=", Value: bucket},
		{Field: "builder", Operator: "=", Value: builder},
		{Field: "visible", Operator: "=", Value: true},
		{Field: "modifier_id", Operator: "=", Value: 0},
		{Field: "commit_timestamp", Operator: ">", Value: time.Now().Add(-time.Hour * 24 * 365)},
	}

	if err := c.coverageV1DsClient.Query(
		ctx, &records, "PostsubmitReport",
		queryFilters, "-commit_timestamp", 0,
	); err != nil {
		logging.Errorf(ctx, "PostsubmitReport: %w", err)
		return nil, ErrInternalServerError
	}

	return records, nil
}

// getCoverageNumbersForPath fetches absolute code coverage numbers for a given
// path for a given set of builder config & commit hashes. It returns
// per date numbers.
func (c *Client) getCoverageNumbersForPath(
	ctx context.Context,
	reports []entities.PostsubmitReport,
	path string,
	bucket string,
	builder string,
) []CoveragePerDate {
	return c.getCoverageNumbersHelper(ctx, reports, path, bucket, builder, "dirs")
}

// getCoverageNumbersForComponent fetches absolute code coverage numbers for a given
// component for a given set of builder config & commit hashes. It returns
// per date numbers.
func (c *Client) getCoverageNumbersForComponent(
	ctx context.Context,
	reports []entities.PostsubmitReport,
	path string,
	bucket string,
	builder string,
) []CoveragePerDate {
	return c.getCoverageNumbersHelper(ctx, reports, path, bucket, builder, "components")
}

func (c *Client) aggregateCoverageReports(
	aggregatedResults map[string]map[string]int64,
	data []CoveragePerDate,
) map[string]map[string]int64 {
	for _, c := range data {
		if _, ok := aggregatedResults[c.date]; !ok {
			aggregatedResults[c.date] = map[string]int64{
				"covered": 0,
				"total":   0,
			}
		}

		aggregatedResults[c.date]["covered"] = aggregatedResults[c.date]["covered"] + int64(c.covered)
		aggregatedResults[c.date]["total"] = aggregatedResults[c.date]["total"] + int64(c.total)
	}

	return aggregatedResults
}

// GetAbsoluteCoverageDataOneYear returns absolute coverage numbers for the last
// 365 days.
func (c *Client) GetAbsoluteCoverageDataOneYear(
	ctx context.Context,
	req *api.GetAbsoluteCoverageDataOneYearRequest,
) (*api.GetAbsoluteCoverageDataOneYearResponse, error) {
	// TODO (see crbug/1509667): Introduce Pagination & response
	// size optimization for GetAbsoluteCoverageDataOneYear API
	unitTestsOnly := req.UnitTestsOnly
	bucket := req.Bucket
	builder := c.getModifedBuilder(req.Builder, &unitTestsOnly)
	paths := req.Paths
	components := req.Components

	reportsLastYear, err := c.getCoverageReportsForLastYear(ctx, bucket, builder)
	if err != nil {
		return nil, err
	}

	aggregatedResults := make(map[string]map[string]int64)
	for _, path := range paths {
		data := c.getCoverageNumbersForPath(ctx, reportsLastYear, path, bucket, builder)
		aggregatedResults = c.aggregateCoverageReports(aggregatedResults, data)
	}

	for _, component := range components {
		data := c.getCoverageNumbersForComponent(ctx, reportsLastYear, component, bucket, builder)
		aggregatedResults = c.aggregateCoverageReports(aggregatedResults, data)
	}

	finalReports := []*api.AbsoluteCoverage{}
	for date, numbers := range aggregatedResults {
		absCov := &api.AbsoluteCoverage{
			Date:         date,
			LinesCovered: numbers["covered"],
			TotalLines:   numbers["total"],
		}
		finalReports = append(finalReports, absCov)
	}

	return &api.GetAbsoluteCoverageDataOneYearResponse{
		Reports: finalReports,
	}, nil
}

// getIncCoverageReportsForLastYear fetches the incremental code coverage reports
// for the last 365 days.
func (c *Client) getIncCoverageReportsForLastYear(
	ctx context.Context,
	path string,
	unitTestsOnly bool,
) ([]entities.CQSummaryCoverageData, error) {
	records := []entities.CQSummaryCoverageData{}
	queryFilters := []datastorage.QueryFilter{
		{Field: "path", Operator: "=", Value: path},
		{Field: "is_unit_test", Operator: "=", Value: unitTestsOnly},
		{Field: "timestamp", Operator: ">", Value: time.Now().Add(-time.Hour * 24 * 365)},
	}

	if err := c.coverageV2DsClient.Query(
		ctx, &records, "CQSummaryCoverageData",
		queryFilters, "timestamp", 0,
	); err != nil {
		logging.Errorf(ctx, "CQSummaryCoverageData: %w", err)
		return nil, ErrInternalServerError
	}

	logging.Infof(ctx, "CQSummaryCoverageData: fetched %d records", len(records))
	return records, nil
}

// aggregateIncrementalCoverageReports takes in the incremental coverage
// entities and return a map of date => [files covered, total file changes made]
func (c *Client) aggregateIncrementalCoverageReports(
	reports []*entities.CQSummaryCoverageData,
) map[string]map[string]int64 {
	aggregatedResults := map[string]map[string]int64{}
	for _, report := range reports {
		date := report.Timestamp.Format(time.DateOnly)
		filesCovered := report.FilesCovered
		totalFilesChanged := report.TotalFilesChanged

		if _, ok := aggregatedResults[date]; !ok {
			aggregatedResults[date] = map[string]int64{
				"covered": 0,
				"total":   0,
			}
		}

		aggregatedResults[date]["covered"] = aggregatedResults[date]["covered"] + int64(filesCovered)
		aggregatedResults[date]["total"] = aggregatedResults[date]["total"] + int64(totalFilesChanged)
	}

	return aggregatedResults
}

// GetIncrementalCoverageDataOneYear returns incremental coverage numbers for
// the last 365 days.
func (c *Client) GetIncrementalCoverageDataOneYear(
	ctx context.Context,
	req *api.GetIncrementalCoverageDataOneYearRequest,
) (*api.GetIncrementalCoverageDataOneYearResponse, error) {
	// TODO (see crbug/1509667): Introduce Pagination & response
	// size optimization for GetIncrementalCoverageDataOneYear API
	unitTestsOnly := req.UnitTestsOnly
	paths := req.Paths

	reportsLastYear := []*entities.CQSummaryCoverageData{}
	for _, path := range paths {
		reports, err := c.getIncCoverageReportsForLastYear(ctx, path, unitTestsOnly)
		if err != nil {
			return nil, err
		}
		for i := range reports {
			reportsLastYear = append(reportsLastYear, &reports[i])
		}
	}

	aggregatedResults := c.aggregateIncrementalCoverageReports(reportsLastYear)

	finalReports := []*api.IncrementalCoverage{}
	for date, numbers := range aggregatedResults {
		incCov := &api.IncrementalCoverage{
			Date:               date,
			FileChangesCovered: numbers["covered"],
			TotalFileChanges:   numbers["total"],
		}
		finalReports = append(finalReports, incCov)
	}

	return &api.GetIncrementalCoverageDataOneYearResponse{
		Reports: finalReports,
	}, nil
}

// ---------- HELPER FUNCTIONS --------------------
func (c *Client) getCoverageNumbersHelper(
	ctx context.Context,
	reports []entities.PostsubmitReport,
	data string,
	bucket string,
	builder string,
	dataType string,
) []CoveragePerDate {
	coverageNumbers := []CoveragePerDate{}
	for _, report := range reports {
		// TODO: The Get method in these entities should return the object and error.
		// Currently the user has to create an empty entity object and supply it to
		// the function. See crbug/1509133
		summary := entities.SummaryCoverageData{}
		err := summary.Get(
			ctx, c.coverageV1DsClient,
			chromiumServerHost,
			chromiumProject,
			chromiumRef,
			report.GitilesCommitRevision,
			dataType,
			data,
			bucket,
			builder,
		)
		if err != nil {
			continue
		}
		coverageDetailsStruct := structpb.Struct{}
		err = getStructFromCompressedData(summary.Data, &coverageDetailsStruct)
		if err != nil {
			continue
		}
		metrics := coverageDetailsStruct.AsMap()
		for _, metric := range metrics["summaries"].([]any) {
			metricMap := metric.(map[string]any)
			if metricMap["name"] == "line" {
				covNumber := CoveragePerDate{
					date:    report.CommitTimestamp.Format(time.DateOnly),
					covered: metricMap["covered"].(float64),
					total:   metricMap["total"].(float64),
				}
				coverageNumbers = append(coverageNumbers, covNumber)
			}
		}
	}
	return coverageNumbers
}
