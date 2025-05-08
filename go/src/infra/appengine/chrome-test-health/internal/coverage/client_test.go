// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coverage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	mock "github.com/stretchr/testify/mock"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/appengine/chrome-test-health/api"
	"go.chromium.org/infra/appengine/chrome-test-health/datastorage"
	"go.chromium.org/infra/appengine/chrome-test-health/datastorage/mocks"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage/entities"
)

var (
	ErrInsufficientArgs = errors.New("insufficent arguments")
	ErrConnection       = errors.New("connection error")
	ErrInvalidKey       = errors.New("invalid key")
	ErrInvalidType      = errors.New("invalid type")
	ErrEntityNotFound   = errors.New("entity not found")
	ErrInternal         = errors.New("internal error")
)

func getMockFinditConfigRoot() *entities.FinditConfigRoot {
	return &entities.FinditConfigRoot{
		Key:     datastore.IDKey("FinditConfigRoot", 1, nil),
		Current: 218,
	}
}

func getMockFinditConfig() *entities.FinditConfig {
	mockSettingsData := []byte(`
{
	"default_postsubmit_report_config": {
		"chromium": {
			"project": "chromium/src",
			"platform": "linux",
			"host": "chromium.googlesource.com",
			"ref": "refs/heads/main"
		}
	},
	"postsubmit_platform_info_map": {
		"chromium": {
			"linux": {
				"bucket": "ci",
				"builder": "linux-code-coverage",
				"coverage_tool": "clang",
				"ui_name": "Linux (C/C++)"
			},
			"android-java": {
				"bucket": "ci",
				"builder": "android-code-coverage",
				"coverage_tool": "jacoco",
				"ui_name": "Android (Java)"
			}
		}
	}
}`)

	parent := getMockFinditConfigRoot()
	return &entities.FinditConfig{
		Key:                  datastore.IDKey("FinditConfig", int64(parent.Current), parent.Key),
		CodeCoverageSettings: mockSettingsData,
	}
}

func getMockFinditConfigWithoutAnyProject() *entities.FinditConfig {
	fakeSettingsData := []byte(`
{
	"default_postsubmit_report_config": {}
}`)

	return &entities.FinditConfig{
		CodeCoverageSettings: fakeSettingsData,
	}
}

func getMockPostsubmitReport() []*entities.PostsubmitReport {
	return []*entities.PostsubmitReport{
		{
			GitilesCommitProject:    "chromium/src",
			GitilesCommitServerHost: "chromium.googlesource.com",
			Bucket:                  "ci",
			Builder:                 "linux-code-coverage",
			GitilesCommitRevision:   "12345",
		},
		{
			GitilesCommitProject:    "chromium/src",
			GitilesCommitServerHost: "chromium.googlesource.com",
			Bucket:                  "ci",
			Builder:                 "andr-code-coverage",
			GitilesCommitRevision:   "23456",
		},
		{
			GitilesCommitProject:    "chromium/src",
			GitilesCommitServerHost: "chromium.googlesource.com",
			Bucket:                  "ci",
			Builder:                 "linux-code-coverage",
			GitilesCommitRevision:   "03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a",
			CommitTimestamp:         time.Date(2023, 11, 17, 20, 34, 58, 0, time.UTC),
		},
	}
}

func getMockCQSummaryReport() []*entities.CQSummaryCoverageData {
	return []*entities.CQSummaryCoverageData{
		{
			Timestamp:         time.Now().Add(-time.Hour * 24 * 30 * 4),
			Change:            4042361,
			Patchset:          10,
			IsUnitTest:        false,
			Path:              "//a/b/",
			DataType:          "dirs",
			FilesCovered:      7,
			TotalFilesChanged: 10,
		},
		{
			Timestamp:         time.Now().Add(-time.Hour * 24 * 30 * 6),
			Change:            4042359,
			Patchset:          12,
			IsUnitTest:        false,
			Path:              "//a/b/",
			DataType:          "dirs",
			FilesCovered:      9,
			TotalFilesChanged: 10,
		},
		{
			Timestamp:         time.Now().Add(-time.Hour * 24 * 30 * 24),
			Change:            40423634,
			Patchset:          2,
			IsUnitTest:        false,
			Path:              "//a/b/",
			DataType:          "dirs",
			FilesCovered:      5,
			TotalFilesChanged: 10,
		},
	}
}

func getMockSummaryData() *entities.SummaryCoverageData {
	mockKey := "chromium.googlesource.com$chromium/src$refs/heads/main" +
		"$03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a$dirs$//$ci$linux-code-coverage$0"
	mockSummaryData, _ := compressString(`
{
	"dirs": [
		{
			"name": "a/",
			"path": "//a/",
			"summaries": [
				{
					"covered": 59,
					"name": "line",
					"total": 200
				}
			]
		}
	],
	"files": [
		{
			"name": "file.cc",
			"path": "//file.cc",
			"summaries": [
				{
					"covered": 64,
					"name": "line",
					"total": 100
				}
			]
		}
	],
	"path": "//",
	"summaries": [
		{
			"covered": 123,
			"name": "line",
			"total": 300
		}
	]
}`)

	return &entities.SummaryCoverageData{
		Key:  datastore.NameKey("SummaryCoverageData", mockKey, nil),
		Data: mockSummaryData,
	}
}

func getMockSummaryDataByComponent() []*entities.SummaryCoverageData {
	res := []*entities.SummaryCoverageData{}

	mockKey := "chromium.googlesource.com$chromium/src$refs/heads/main" +
		"$03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a$components$C1$ci$linux-code-coverage$0"
	mockSummaryData, _ := compressString(`
{
	"dirs": [],
	"files": [],
	"path": "C1",
	"summaries": [
		{
			"covered": 59,
			"name": "line",
			"total": 200
		}
	]
}`)
	res = append(res, &entities.SummaryCoverageData{
		Key:  datastore.NameKey("SummaryCoverageData", mockKey, nil),
		Data: mockSummaryData,
	})

	mockKey = "chromium.googlesource.com$chromium/src$refs/heads/main" +
		"$03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a$components$C2>C3$ci$linux-code-coverage$0"
	mockSummaryData, _ = compressString(`
{
	"dirs": [],
	"files": [],
	"path": "C2>C3",
	"summaries": []
}`)
	res = append(res, &entities.SummaryCoverageData{
		Key:  datastore.NameKey("SummaryCoverageData", mockKey, nil),
		Data: mockSummaryData,
	})

	return res
}

func TestGetProjectConfig(t *testing.T) {
	t.Parallel()
	ftt.Run(`Should have valid "FinditConfig" entity`, t, func(t *ftt.Test) {
		client := Client{}
		ctx := context.Background()
		t.Run(`Invalid "CodeCoverageSettings" JSON`, func(t *ftt.Test) {
			fakeFinditConfig := entities.FinditConfig{
				CodeCoverageSettings: []byte(""),
			}
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getProjectConfig(ctx, &fakeFinditConfig, "chromium", config)
			assert.Loosely(t, config, should.Resemble(&api.GetProjectDefaultConfigResponse{}))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		})
		t.Run(`Missing "default_postsubmit_report_config" property`, func(t *ftt.Test) {
			fakeFinditConfig := entities.FinditConfig{
				CodeCoverageSettings: []byte("{}"),
			}
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getProjectConfig(ctx, &fakeFinditConfig, "chromium", config)
			assert.Loosely(t, config, should.Resemble(&api.GetProjectDefaultConfigResponse{}))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		})
		t.Run(`Missing project from "default_postsubmit_report_config" property`, func(t *ftt.Test) {
			fakeFinditConfig := getMockFinditConfigWithoutAnyProject()
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getProjectConfig(ctx, fakeFinditConfig, "chromium", config)
			assert.Loosely(t, config, should.Resemble(&api.GetProjectDefaultConfigResponse{}))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		})
		t.Run(`Valid "FinditConfig" entity`, func(t *ftt.Test) {
			fakeFinditConfig := getMockFinditConfig()
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getProjectConfig(ctx, fakeFinditConfig, "chromium", config)
			assert.Loosely(t, config, should.Resemble(&api.GetProjectDefaultConfigResponse{
				GitilesHost:    "chromium.googlesource.com",
				GitilesProject: "chromium/src",
				GitilesRef:     "refs/heads/main",
			}))
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestGetBuilderOptions(t *testing.T) {
	ftt.Run(`Should get builder configurations`, t, func(t *ftt.Test) {
		client := Client{}
		ctx := context.Background()

		t.Run(`Invalid "CodeCoverageSettings" JSON`, func(t *ftt.Test) {
			mockFinditConfig := &entities.FinditConfig{
				CodeCoverageSettings: []byte(""),
			}
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getBuilderOptions(ctx, "chromium", "chromium.googlesource.com",
				"chromium", mockFinditConfig, config)
			assert.Loosely(t, config, should.Resemble(&api.GetProjectDefaultConfigResponse{}))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		})

		t.Run(`Missing "postsubmit_platform_info_map" property`, func(t *ftt.Test) {
			mockFinditConfig := &entities.FinditConfig{
				CodeCoverageSettings: []byte("{}"),
			}
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getBuilderOptions(ctx, "chromium", "chromium.googlesource.com",
				"chromium", mockFinditConfig, config)
			assert.Loosely(t, config, should.Resemble(&api.GetProjectDefaultConfigResponse{}))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		})

		t.Run(`FinditConfig has platform options`, func(t *ftt.Test) {
			postsubmitReports := getMockPostsubmitReport()
			mockDataClient := mocks.NewIDataClient(t)
			mockDataClient.On(
				"Query",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"PostsubmitReport",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
					for _, rep := range postsubmitReports {
						if queryFilters[2].Value == rep.Bucket && queryFilters[3].Value == rep.Builder {
							res := reflect.ValueOf(result).Elem()
							res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
							return nil
						}
					}
					return nil
				},
			)
			client.coverageV1DsClient = mockDataClient

			finditConfig := getMockFinditConfig()
			config := &api.GetProjectDefaultConfigResponse{}
			err := client.getBuilderOptions(ctx, "chromium", "chromium.googlesource.com",
				"chromium/src", finditConfig, config)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, config.BuilderConfig, should.HaveLength(1))
		})
	})

}

func TestGetModifiedBuilder(t *testing.T) {
	t.Parallel()
	ftt.Run(`Should be able to modify builder based on field unitTestsOnly`, t, func(t *ftt.Test) {
		client := Client{}
		t.Run(`Field unitTestsOnly is set to true`, func(t *ftt.Test) {
			unitTestsOnly := true
			modifiedBuilder := client.getModifedBuilder("builder", &unitTestsOnly)
			assert.Loosely(t, modifiedBuilder, should.Equal("builder_unit"))
		})
		t.Run(`Field unitTestsOnly is set to false`, func(t *ftt.Test) {
			unitTestsOnly := false
			modifiedBuilder := client.getModifedBuilder("builder", &unitTestsOnly)
			assert.Loosely(t, modifiedBuilder, should.Equal("builder"))
		})
		t.Run(`Field unitTestsOnly is not provided`, func(t *ftt.Test) {
			modifiedBuilder := client.getModifedBuilder("builder", nil)
			assert.Loosely(t, modifiedBuilder, should.Equal("builder"))
		})
	})
}

func TestGetProjectDefaultConfig(t *testing.T) {
	t.Parallel()
	ftt.Run(`Should get project's default configuration`, t, func(t *ftt.Test) {
		client := Client{}
		ctx := context.Background()

		finditConfigRoot := getMockFinditConfigRoot()
		finditConfig := getMockFinditConfig()
		postsubmitReports := getMockPostsubmitReport()

		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"FinditConfigRoot",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				res := reflect.ValueOf(result).Elem()
				res.Set(reflect.Append(res, reflect.ValueOf(finditConfigRoot).Elem()))
				return nil
			},
		)

		mockDataClient.On(
			"Get",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"FinditConfig",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(ctx context.Context, result any, dataType string, key any, options ...any) error {
				res := reflect.ValueOf(result).Elem()
				res.Set(reflect.ValueOf(finditConfig).Elem())
				return nil
			},
		)

		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"PostsubmitReport",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				for _, rep := range postsubmitReports {
					if queryFilters[2].Value == rep.Bucket && queryFilters[3].Value == rep.Builder {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
						return nil
					}
				}
				return nil
			},
		)
		client.coverageV1DsClient = mockDataClient

		req := api.GetProjectDefaultConfigRequest{
			LuciProject: "chromium",
		}
		res, err := client.GetProjectDefaultConfig(ctx, &req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, res.GitilesHost, should.Equal("chromium.googlesource.com"))
		assert.Loosely(t, res.GitilesProject, should.Equal("chromium/src"))
		assert.Loosely(t, res.GitilesRef, should.Equal("refs/heads/main"))
		assert.Loosely(t, res.BuilderConfig, should.HaveLength(1))
	})
}

func TestGetCoverageSummary(t *testing.T) {
	t.Parallel()

	ftt.Run(`Should get summary data`, t, func(t *ftt.Test) {
		client := Client{}
		ctx := context.Background()

		summaryData := getMockSummaryData()

		mockDataClient := mocks.NewIDataClient(t)
		client.coverageV1DsClient = mockDataClient

		commonMock := func() {
			mockDataClient.On(
				"Get",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"SummaryCoverageData",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(ctx context.Context, result any, dataType string, key any, options ...any) error {
					if key.(string) != summaryData.Key.Name {
						return ErrEntityNotFound
					}

					res := reflect.ValueOf(result).Elem()
					res.Set(reflect.ValueOf(summaryData).Elem())
					return nil
				},
			)
		}

		req := api.GetCoverageSummaryRequest{
			GitilesHost:     "chromium.googlesource.com",
			GitilesProject:  "chromium/src",
			GitilesRef:      "refs/heads/main",
			GitilesRevision: "03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a",
			Path:            "//",
			UnitTestsOnly:   false,
			Bucket:          "ci",
			Builder:         "linux-code-coverage",
		}
		t.Run(`with valid params`, func(t *ftt.Test) {
			commonMock()
			res, err := client.GetCoverageSummary(ctx, &req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res.Summary, should.NotBeEmpty)
		})
		t.Run(`with no matching entity in datastore`, func(t *ftt.Test) {
			commonMock()
			req.Bucket = "random"
			res, err := client.GetCoverageSummary(ctx, &req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
			assert.Loosely(t, res, should.BeNil)
		})
		t.Run(`with malformed data`, func(t *ftt.Test) {
			commonMock()
			summaryData.Data, _ = compressString("{")
			mockDataClient.On(
				"Get",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"SummaryCoverageData",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(ctx context.Context, result any, dataType string, key any, options ...any) error {
					if key.(string) != summaryData.Key.Name {
						return ErrEntityNotFound
					}

					res := reflect.ValueOf(result).Elem()
					res.Set(reflect.ValueOf(summaryData).Elem())
					return nil
				},
			)

			res, err := client.GetCoverageSummary(ctx, &req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
			assert.Loosely(t, res, should.BeNil)
		})
	})
}

func TestGetCoverageSummaryForComponents(t *testing.T) {
	t.Parallel()
	ftt.Run(`Should get summary data by components`, t, func(t *ftt.Test) {
		client := Client{}
		ctx := context.Background()

		summaryData := getMockSummaryDataByComponent()

		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Get",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"SummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(ctx context.Context, result any, dataType string, key any, options ...any) error {
				for _, sum := range summaryData {
					if key.(string) == sum.Key.Name {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.ValueOf(sum).Elem())
						return nil
					}
				}
				return nil
			},
		)
		client.coverageV1DsClient = mockDataClient

		req := api.GetCoverageSummaryRequest{
			GitilesHost:     "chromium.googlesource.com",
			GitilesProject:  "chromium/src",
			GitilesRef:      "refs/heads/main",
			GitilesRevision: "03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a",
			Components:      []string{"C1", "C2>C3"},
			UnitTestsOnly:   false,
			Bucket:          "ci",
			Builder:         "linux-code-coverage",
		}

		res, err := client.GetCoverageSummary(ctx, &req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, res, should.NotBeNil)
		assert.Loosely(t, res.Summary, should.HaveLength(2))
	})
}

func TestGetCoverageReportsForLastYear(t *testing.T) {
	t.Parallel()
	client := Client{}
	ctx := context.Background()

	ftt.Run("Should return reports", t, func(t *ftt.Test) {
		postsubmitReports := getMockPostsubmitReport()
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"PostsubmitReport",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				for _, rep := range postsubmitReports {
					if queryFilters[2].Value == rep.Bucket && queryFilters[3].Value == rep.Builder {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
						return nil
					}
				}
				return nil
			},
		)
		client.coverageV1DsClient = mockDataClient

		reports, err := client.getCoverageReportsForLastYear(ctx, "ci", "linux-code-coverage")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, reports, should.HaveLength(1))
		expectedReports := []entities.PostsubmitReport{
			{
				GitilesCommitProject:    "chromium/src",
				GitilesCommitServerHost: "chromium.googlesource.com",
				Bucket:                  "ci",
				Builder:                 "linux-code-coverage",
				GitilesCommitRevision:   "12345",
			},
		}
		assert.Loosely(t, reports, should.Resemble(expectedReports))
	})

	ftt.Run("Should error out with no matching index message", t, func(t *ftt.Test) {
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"PostsubmitReport",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				return fmt.Errorf("PostsubmitReport: %s", "No matching indexes found")
			},
		)
		client.coverageV1DsClient = mockDataClient

		reports, err := client.getCoverageReportsForLastYear(ctx, "ci", "linux-code-coverage")
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		assert.Loosely(t, reports, should.BeNil)
	})
}

func TestGetIncCoverageReportsForLastYear(t *testing.T) {
	t.Parallel()
	client := Client{}
	ctx := context.Background()

	ftt.Run("Should return incremental coverage numbers per day for the path", t, func(t *ftt.Test) {
		reports := getMockCQSummaryReport()
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"CQSummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				for _, rep := range reports {
					matchesPath := queryFilters[0].Value == rep.Path
					matchesIsUnitTests := queryFilters[1].Value == rep.IsUnitTest
					t := queryFilters[2].Value.(time.Time)
					matchesTime := t.Before(rep.Timestamp)
					if matchesPath && matchesIsUnitTests && matchesTime {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
					}
				}
				return nil
			},
		)
		client.coverageV2DsClient = mockDataClient

		data, err := client.getIncCoverageReportsForLastYear(ctx, "//a/b/", false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, data, should.HaveLength(2))
		expectedData := []entities.CQSummaryCoverageData{
			*reports[0],
			*reports[1],
		}
		assert.Loosely(t, data, should.Resemble(expectedData))
	})

	ftt.Run("Should error out with no matching index message", t, func(t *ftt.Test) {
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"CQSummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				return fmt.Errorf("CQSummaryCoverageData: %s", "No matching indexes found")
			},
		)
		client.coverageV2DsClient = mockDataClient

		reports, err := client.getIncCoverageReportsForLastYear(ctx, "//a/b/", false)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		assert.Loosely(t, reports, should.BeNil)
	})
}

func TestGetCoverageNumbersForPath(t *testing.T) {
	t.Parallel()
	client := Client{}
	ctx := context.Background()

	ftt.Run("Should return coverage numbers per day for the path", t, func(t *ftt.Test) {
		summaryData := getMockSummaryData()
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Get",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"SummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(ctx context.Context, result any, dataType string, key any, options ...any) error {
				if key.(string) != summaryData.Key.Name {
					return ErrEntityNotFound
				}

				res := reflect.ValueOf(result).Elem()
				res.Set(reflect.ValueOf(summaryData).Elem())
				return nil
			},
		)
		client.coverageV1DsClient = mockDataClient

		reports := []entities.PostsubmitReport{*getMockPostsubmitReport()[0]}
		ts := time.Date(2009, 11, 17, 20, 34, 58, 0, time.UTC)
		reports[0].GitilesCommitRevision = "03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a"
		reports[0].CommitTimestamp = ts

		data := client.getCoverageNumbersForPath(ctx, reports, "//", "ci", "linux-code-coverage")
		assert.Loosely(t, data, should.HaveLength(1))
		expectedData := []CoveragePerDate{
			{
				date:    "2009-11-17",
				covered: 123,
				total:   300,
			},
		}
		assert.Loosely(t, data, should.Resemble(expectedData))
	})
}

func TestGetCoverageNumbersForComponent(t *testing.T) {
	t.Parallel()
	client := Client{}
	ctx := context.Background()

	ftt.Run("Should return coverage numbers per day for the component", t, func(t *ftt.Test) {
		summaryData := getMockSummaryDataByComponent()
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Get",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"SummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(ctx context.Context, result any, dataType string, key any, options ...any) error {
				for _, sum := range summaryData {
					if key.(string) == sum.Key.Name {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.ValueOf(sum).Elem())
						return nil
					}
				}
				return nil
			},
		)
		client.coverageV1DsClient = mockDataClient

		reports := []entities.PostsubmitReport{*getMockPostsubmitReport()[0]}
		ts := time.Date(2009, 11, 17, 20, 34, 58, 0, time.UTC)
		reports[0].GitilesCommitRevision = "03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a"
		reports[0].CommitTimestamp = ts

		data := client.getCoverageNumbersForComponent(ctx, reports, "C1", "ci", "linux-code-coverage")
		assert.Loosely(t, data, should.HaveLength(1))
		expectedData := []CoveragePerDate{
			{
				date:    "2009-11-17",
				covered: 59,
				total:   200,
			},
		}
		assert.Loosely(t, data, should.Resemble(expectedData))
	})
}

func TestGetAbsoluteCoverageDataOneYear(t *testing.T) {
	t.Skip("crbug.com/379569289: test data was 2023/11/17, so fail after 2024/11/17")
	t.Parallel()
	ctx := context.Background()
	client := Client{
		coverageV1DsClient: mocks.NewIDataClient(t),
	}

	ftt.Run("Should pass", t, func(t *ftt.Test) {
		postsubmitReports := getMockPostsubmitReport()
		summaryData := getMockSummaryData()
		summaryDataByComp := getMockSummaryDataByComponent()
		mockDataClient := mocks.NewIDataClient(t)
		client.coverageV1DsClient = mockDataClient

		commonMock := func() {
			mockDataClient.On(
				"Query",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"PostsubmitReport",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
					for _, rep := range postsubmitReports {
						if queryFilters[2].Value == rep.Bucket &&
							queryFilters[3].Value == rep.Builder &&
							rep.CommitTimestamp.After(queryFilters[6].Value.(time.Time)) {
							res := reflect.ValueOf(result).Elem()
							res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
							return nil
						}
					}
					return nil
				},
			)

			mockDataClient.On(
				"Get",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"SummaryCoverageData",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(ctx context.Context, result any, dataType string, key any, options ...any) error {
					if strings.Contains(key.(string), "dirs") {
						if key.(string) != summaryData.Key.Name {
							return ErrEntityNotFound
						}

						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.ValueOf(summaryData).Elem())
						return nil
					} else {
						for _, sum := range summaryDataByComp {
							if key.(string) == sum.Key.Name {
								res := reflect.ValueOf(result).Elem()
								res.Set(reflect.ValueOf(sum).Elem())
								return nil
							}
						}
						return nil
					}
				},
			)
		}

		t.Run("Valid", func(t *ftt.Test) {
			commonMock()
			req := &api.GetAbsoluteCoverageDataOneYearRequest{
				Paths:         []string{"//"},
				Components:    []string{"C1", "C2>C3"},
				UnitTestsOnly: false,
				Bucket:        "ci",
				Builder:       "linux-code-coverage",
			}
			res, err := client.GetAbsoluteCoverageDataOneYear(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(res.Reports), should.BeGreaterThan(0))
			expectedRes := &api.GetAbsoluteCoverageDataOneYearResponse{
				Reports: []*api.AbsoluteCoverage{{Date: "2023-11-17", LinesCovered: 182, TotalLines: 500}},
			}
			assert.Loosely(t, res, should.Resemble(expectedRes))
		})
		t.Run("No components with some paths", func(t *ftt.Test) {
			commonMock()
			req := &api.GetAbsoluteCoverageDataOneYearRequest{
				Paths:         []string{"//"},
				Components:    []string{},
				UnitTestsOnly: false,
				Bucket:        "ci",
				Builder:       "linux-code-coverage",
			}
			res, err := client.GetAbsoluteCoverageDataOneYear(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(res.Reports), should.BeGreaterThan(0))
			expectedRes := &api.GetAbsoluteCoverageDataOneYearResponse{
				Reports: []*api.AbsoluteCoverage{{Date: "2023-11-17", LinesCovered: 123, TotalLines: 300}},
			}
			assert.Loosely(t, res, should.Resemble(expectedRes))
		})
		t.Run("No paths with some components", func(t *ftt.Test) {
			commonMock()
			req := &api.GetAbsoluteCoverageDataOneYearRequest{
				Paths:         []string{},
				Components:    []string{"C1", "C2>C3"},
				UnitTestsOnly: false,
				Bucket:        "ci",
				Builder:       "linux-code-coverage",
			}
			res, err := client.GetAbsoluteCoverageDataOneYear(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(res.Reports), should.BeGreaterThan(0))
			expectedRes := &api.GetAbsoluteCoverageDataOneYearResponse{
				Reports: []*api.AbsoluteCoverage{{Date: "2023-11-17", LinesCovered: 59, TotalLines: 200}},
			}
			assert.Loosely(t, res, should.Resemble(expectedRes))
		})
	})

	ftt.Run("Should fail", t, func(t *ftt.Test) {
		t.Run("PostsubmitReport fetch error", func(t *ftt.Test) {
			mockDataClient := mocks.NewIDataClient(t)
			mockDataClient.On(
				"Query",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"PostsubmitReport",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
					return fmt.Errorf("PostsubmitReport: %s", "entity not found")
				},
			)
			client.coverageV1DsClient = mockDataClient
			req := &api.GetAbsoluteCoverageDataOneYearRequest{
				Paths:         []string{"//"},
				Components:    []string{},
				UnitTestsOnly: false,
				Bucket:        "ci",
				Builder:       "linux-code-coverage",
			}
			res, err := client.GetAbsoluteCoverageDataOneYear(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
			assert.Loosely(t, res, should.BeNil)
		})
	})
}

func TestGetIncrementalCoverageDataOneYear(t *testing.T) {
	t.Parallel()

	ftt.Run("Should pass", t, func(t *ftt.Test) {
		ctx := context.Background()
		client := Client{
			coverageV1DsClient: mocks.NewIDataClient(t),
		}

		reports := getMockCQSummaryReport()
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"CQSummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				for _, rep := range reports {
					matchesPath := queryFilters[0].Value == rep.Path
					matchesIsUnitTests := queryFilters[1].Value == rep.IsUnitTest
					t := queryFilters[2].Value.(time.Time)
					matchesTime := t.Before(rep.Timestamp)
					if matchesPath && matchesIsUnitTests && matchesTime {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
					}
				}
				return nil
			},
		)
		client.coverageV2DsClient = mockDataClient

		req := &api.GetIncrementalCoverageDataOneYearRequest{
			Paths:         []string{"//a/b/"},
			UnitTestsOnly: false,
		}
		res, err := client.GetIncrementalCoverageDataOneYear(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(res.Reports), should.BeGreaterThan(0))
		expectedRes := &api.GetIncrementalCoverageDataOneYearResponse{
			Reports: []*api.IncrementalCoverage{
				{Date: reports[0].Timestamp.Format(time.DateOnly), FileChangesCovered: 7, TotalFileChanges: 10},
				{Date: reports[1].Timestamp.Format(time.DateOnly), FileChangesCovered: 9, TotalFileChanges: 10},
			},
		}
		sort.Slice(res.Reports, func(i, j int) bool {
			d1, _ := time.Parse(time.DateOnly, res.Reports[i].Date)
			d2, _ := time.Parse(time.DateOnly, res.Reports[j].Date)

			return d1.Before(d2)
		})

		sort.Slice(expectedRes.Reports, func(i, j int) bool {
			d1, _ := time.Parse(time.DateOnly, expectedRes.Reports[i].Date)
			d2, _ := time.Parse(time.DateOnly, expectedRes.Reports[j].Date)

			return d1.Before(d2)
		})
		assert.Loosely(t, res, should.Resemble(expectedRes))
	})

	ftt.Run("Should fail", t, func(t *ftt.Test) {
		ctx := context.Background()
		client := Client{
			coverageV1DsClient: mocks.NewIDataClient(t),
		}
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"CQSummaryCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				return fmt.Errorf("CQSummaryCoverageData: %s", "entity not found")
			},
		)
		client.coverageV2DsClient = mockDataClient
		req := &api.GetIncrementalCoverageDataOneYearRequest{
			Paths:         []string{"//a/b/"},
			UnitTestsOnly: false,
		}
		res, err := client.GetIncrementalCoverageDataOneYear(ctx, req)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err, should.Resemble(ErrInternalServerError))
		assert.Loosely(t, res, should.BeNil)
	})
}

func TestAggregateCoverageReports(t *testing.T) {
	t.Parallel()
	client := Client{}

	ftt.Run("Should aggregate coverage numbers into the supplied map", t, func(t *ftt.Test) {
		t.Run("When data is complete", func(t *ftt.Test) {
			existingMap := make(map[string]map[string]int64)
			existingMap["2023-11-20"] = map[string]int64{
				"covered": 123,
				"total":   200,
			}
			existingMap["2023-11-21"] = map[string]int64{
				"covered": 150,
				"total":   200,
			}

			data := []CoveragePerDate{
				{date: "2023-11-20", covered: 10, total: 20},
				{date: "2023-11-21", covered: 20, total: 30},
			}
			existingMap = client.aggregateCoverageReports(existingMap, data)

			assert.Loosely(t, existingMap["2023-11-20"], should.Resemble(map[string]int64{"covered": 133, "total": 220}))
			assert.Loosely(t, existingMap["2023-11-21"], should.Resemble(map[string]int64{"covered": 170, "total": 230}))
		})

		t.Run("When data has missing dates", func(t *ftt.Test) {
			existingMap := make(map[string]map[string]int64)
			existingMap["2023-11-20"] = map[string]int64{
				"covered": 123,
				"total":   200,
			}
			existingMap["2023-11-21"] = map[string]int64{
				"covered": 150,
				"total":   200,
			}

			data := []CoveragePerDate{
				{date: "2023-11-20", covered: 10, total: 20},
			}
			existingMap = client.aggregateCoverageReports(existingMap, data)

			assert.Loosely(t, existingMap["2023-11-20"], should.Resemble(map[string]int64{"covered": 133, "total": 220}))
			assert.Loosely(t, existingMap["2023-11-21"], should.Resemble(map[string]int64{"covered": 150, "total": 200}))
		})
	})
}

func TestAggregateIncrementalCoverageReports(t *testing.T) {
	t.Parallel()
	client := Client{}

	ftt.Run("Should aggregate inc coverage numbers", t, func(t *ftt.Test) {
		data := getMockCQSummaryReport()
		data = append(data, &entities.CQSummaryCoverageData{
			Timestamp:         data[0].Timestamp,
			Change:            4042362,
			Patchset:          3,
			IsUnitTest:        false,
			Path:              "//a/b/",
			DataType:          "dirs",
			FilesCovered:      12,
			TotalFilesChanged: 13,
		},
		)
		m := client.aggregateIncrementalCoverageReports(data)

		assert.Loosely(t, m[data[0].Timestamp.Format(time.DateOnly)], should.Resemble(map[string]int64{"covered": 19, "total": 23}))
		assert.Loosely(t, m[data[1].Timestamp.Format(time.DateOnly)], should.Resemble(map[string]int64{"covered": 9, "total": 10}))
		assert.Loosely(t, m[data[2].Timestamp.Format(time.DateOnly)], should.Resemble(map[string]int64{"covered": 5, "total": 10}))
	})
}
