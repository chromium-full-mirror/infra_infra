// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cron

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/appengine/chrome-test-health/datastorage"
	"go.chromium.org/infra/appengine/chrome-test-health/datastorage/mocks"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage/entities"
)

func getMockPresubmitData() []*entities.PresubmitCoverageData {
	return []*entities.PresubmitCoverageData{
		{
			ServerHost:      "chromium-review.googlesource.com",
			Change:          1,
			Patchset:        2,
			UpdateTimestamp: time.Now().Add(-time.Hour * 6),
			IncrementalPercentages: []entities.Cov{
				{
					Path:         "//dir1/dir2/file1.cc",
					CoveredLines: 0,
					TotalLines:   4,
				},
				{
					Path:         "//dir1/dir3/file2.cc",
					CoveredLines: 11,
					TotalLines:   11,
				},
			},
			IncrementalPercentagesUnit: []entities.Cov{
				{
					Path:         "//dir1/dir2/file1.cc",
					CoveredLines: 0,
					TotalLines:   4,
				},
				{
					Path:         "//dir1/dir3/file2.cc",
					CoveredLines: 11,
					TotalLines:   11,
				},
			},
		},
		{
			ServerHost:      "chromium-review.googlesource.com",
			UpdateTimestamp: time.Now().Add(-time.Hour * 25),
			Change:          1,
			Patchset:        1,
			IncrementalPercentages: []entities.Cov{
				{
					Path:         "//dir1/dir2/file1.cc",
					CoveredLines: 0,
					TotalLines:   4,
				},
				{
					Path:         "//dir1/dir3/file2.cc",
					CoveredLines: 11,
					TotalLines:   11,
				},
			},
			IncrementalPercentagesUnit: []entities.Cov{
				{
					Path:         "//dir1/dir2/file1.cc",
					CoveredLines: 0,
					TotalLines:   4,
				},
				{
					Path:         "//dir1/dir3/file2.cc",
					CoveredLines: 11,
					TotalLines:   11,
				},
			},
		},
	}
}

func TestUpdatePresubmitData(t *testing.T) {
	t.Parallel()
	client := CronClient{}
	ctx := context.Background()

	ftt.Run("Update presubmit data", t, func(t *ftt.Test) {
		t.Run("Should pass", func(t *ftt.Test) {
			reports := getMockPresubmitData()
			mockDataClient := mocks.NewIDataClient(t)
			mockDataClient.On(
				"Query",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				"PresubmitCoverageData",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
					for _, rep := range reports {
						matchesHost := queryFilters[0].Value == rep.ServerHost
						t := queryFilters[1].Value.(time.Time)
						matchesTime := t.Before(rep.UpdateTimestamp)
						if matchesHost && matchesTime {
							res := reflect.ValueOf(result).Elem()
							res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
							return nil
						}
					}
					return nil
				},
			)
			mockDataClient.On(
				"BatchPut",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, entities any, keys any) error {
					return nil
				},
			)
			client.coverageV1DsClient = mockDataClient
			client.coverageV2DsClient = mockDataClient

			err := client.UpdatePresubmitData(ctx)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Should fail", func(t *ftt.Test) {
			t.Run("Cannot fetch presubmit data", func(t *ftt.Test) {
				mockDataClient := mocks.NewIDataClient(t)
				mockDataClient.On(
					"Query",
					mock.AnythingOfType("backgroundCtx"),
					mock.Anything,
					"PresubmitCoverageData",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(
					func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
						return fmt.Errorf("PresubmitCoverageData: No matching index")
					},
				)
				client.coverageV1DsClient = mockDataClient
				client.coverageV2DsClient = mockDataClient

				err := client.UpdatePresubmitData(ctx)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.Resemble(coverage.ErrInternalServerError))
			})

			t.Run("Cannot store processed data", func(t *ftt.Test) {
				reports := getMockPresubmitData()
				mockDataClient := mocks.NewIDataClient(t)
				mockDataClient.On(
					"Query",
					mock.AnythingOfType("backgroundCtx"),
					mock.Anything,
					"PresubmitCoverageData",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(
					func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
						for _, rep := range reports {
							matchesHost := queryFilters[0].Value == rep.ServerHost
							t := queryFilters[1].Value.(time.Time)
							matchesTime := t.Before(rep.UpdateTimestamp)
							if matchesHost && matchesTime {
								res := reflect.ValueOf(result).Elem()
								res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
								return nil
							}
						}
						return nil
					},
				)
				mockDataClient.On(
					"BatchPut",
					mock.AnythingOfType("backgroundCtx"),
					mock.Anything,
					mock.Anything,
				).Return(
					func(c context.Context, entities any, keys any) error {
						return fmt.Errorf("PresubmitCoverageData: Error storing the entity")
					},
				)
				client.coverageV1DsClient = mockDataClient
				client.coverageV2DsClient = mockDataClient

				err := client.UpdatePresubmitData(ctx)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.Resemble(errors.New("PresubmitCoverageData: Error storing the entity")))
			})
		})
	})
}

func TestGetPresubmitReportsForLastYear(t *testing.T) {
	t.Parallel()
	client := CronClient{}
	ctx := context.Background()

	ftt.Run("Should return presubmit data for last day", t, func(t *ftt.Test) {
		reports := getMockPresubmitData()
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"PresubmitCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				for _, rep := range reports {
					matchesHost := queryFilters[0].Value == rep.ServerHost
					t := queryFilters[1].Value.(time.Time)
					matchesTime := t.Before(rep.UpdateTimestamp)
					if matchesHost && matchesTime {
						res := reflect.ValueOf(result).Elem()
						res.Set(reflect.Append(res, reflect.ValueOf(rep).Elem()))
						return nil
					}
				}
				return nil
			},
		)
		client.coverageV1DsClient = mockDataClient

		data, err := client.getPresubmitReportsOneDay(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, data, should.HaveLength(1))
		expectedData := []entities.PresubmitCoverageData{
			*reports[0],
		}
		assert.Loosely(t, data, should.Resemble(expectedData))
	})

	ftt.Run("Should error out with no matching index message", t, func(t *ftt.Test) {
		mockDataClient := mocks.NewIDataClient(t)
		mockDataClient.On(
			"Query",
			mock.AnythingOfType("backgroundCtx"),
			mock.Anything,
			"PresubmitCoverageData",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(
			func(c context.Context, result any, dataType string, queryFilters []datastorage.QueryFilter, order any, limit int, options ...any) error {
				return fmt.Errorf("PresubmitCoverageData: %s", "No matching indexes found")
			},
		)
		client.coverageV1DsClient = mockDataClient

		reports, err := client.getPresubmitReportsOneDay(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err, should.Resemble(coverage.ErrInternalServerError))
		assert.Loosely(t, reports, should.BeNil)
	})
}

func TestSplitSinglePresubmitData(t *testing.T) {
	t.Parallel()
	client := CronClient{}

	ftt.Run("Should split presubmit data if patchset is latest", t, func(t *ftt.Test) {
		reports := getMockPresubmitData()
		maxPatchsetMap := map[int64]int64{1: 2}
		result := client.splitSinglePresubmitData(reports[0], maxPatchsetMap, false)
		expected := map[string]IncrementalCoverageData{
			"//":                   {CoveredFiles: 1, TotalFiles: 2, IsDir: true},
			"//dir1/":              {CoveredFiles: 1, TotalFiles: 2, IsDir: true},
			"//dir1/dir2/":         {CoveredFiles: 0, TotalFiles: 1, IsDir: true},
			"//dir1/dir2/file1.cc": {CoveredFiles: 0, TotalFiles: 1, IsDir: false},
			"//dir1/dir3/":         {CoveredFiles: 1, TotalFiles: 1, IsDir: true},
			"//dir1/dir3/file2.cc": {CoveredFiles: 1, TotalFiles: 1, IsDir: false},
		}
		assert.Loosely(t, result, should.Resemble(expected))
	})

	ftt.Run("Should return nil if patchset is not latest", t, func(t *ftt.Test) {
		reports := getMockPresubmitData()
		maxPatchsetMap := map[int64]int64{1: 2}
		result := client.splitSinglePresubmitData(reports[1], maxPatchsetMap, false)
		assert.Loosely(t, result, should.BeNil)
	})
}

func TestGetMaxPatchsetToChangeMap(t *testing.T) {
	t.Parallel()
	client := CronClient{}

	ftt.Run("Should return map", t, func(t *ftt.Test) {
		t.Run("With no reports", func(t *ftt.Test) {
			reports := []entities.PresubmitCoverageData{}
			have := client.getMaxPatchsetToChangeMap(reports)
			want := map[int64]int64{}
			assert.Loosely(t, have, should.Resemble(want))
		})

		t.Run("With some reports", func(t *ftt.Test) {
			mockRep := getMockPresubmitData()
			rep1 := *mockRep[0]
			rep2 := rep1
			rep2.Change = 2
			rep2.Patchset = 1
			rep3 := rep1
			rep3.Change = 1
			rep3.Patchset = 3
			reports := []entities.PresubmitCoverageData{rep1, rep2, rep3}
			have := client.getMaxPatchsetToChangeMap(reports)
			want := map[int64]int64{1: 3, 2: 1}
			assert.Loosely(t, have, should.Resemble(want))
		})
	})
}

func TestGetDir(t *testing.T) {
	t.Parallel()

	ftt.Run("Should return parent directory", t, func(t *ftt.Test) {
		t.Run("For a directory path", func(t *ftt.Test) {
			parent := getDir("//a/b/")
			assert.Loosely(t, parent, should.Equal("//a/"))

			parent = getDir("//a/")
			assert.Loosely(t, parent, should.Equal("//"))
		})

		t.Run("For a file path", func(t *ftt.Test) {
			parent := getDir("//a/b/c.ext")
			assert.Loosely(t, parent, should.Equal("//a/b/"))
		})

		t.Run("For root path", func(t *ftt.Test) {
			parent := getDir("//")
			assert.Loosely(t, parent, should.Equal("//"))
		})
	})
}

func TestCreateCqSummaryDatat(t *testing.T) {
	t.Parallel()
	client := CronClient{}

	ftt.Run("Create CQ summary coverage data", t, func(t *ftt.Test) {
		t.Run("Should pass", func(t *ftt.Test) {
			mockDataClient := mocks.NewIDataClient(t)
			mockDataClient.On(
				"BatchPut",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, entities any, keys any) error {
					return nil
				},
			)
			client.coverageV2DsClient = mockDataClient

			mockData := map[string]IncrementalCoverageData{
				"//":   {CoveredFiles: 2, TotalFiles: 3, IsDir: true},
				"//a/": {CoveredFiles: 1, TotalFiles: 1, IsDir: true},
				"//b/": {CoveredFiles: 1, TotalFiles: 2, IsDir: true},
			}
			err := client.createCqSummaryData(context.Background(), time.Now(), int64(1234), int64(1), false, mockData)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Should fail", func(t *ftt.Test) {
			mockDataClient := mocks.NewIDataClient(t)
			mockDataClient.On(
				"BatchPut",
				mock.AnythingOfType("backgroundCtx"),
				mock.Anything,
				mock.Anything,
			).Return(
				func(c context.Context, entities any, keys any) error {
					return fmt.Errorf("Datastore: %s", "Error putting entities")
				},
			)
			client.coverageV2DsClient = mockDataClient
			err := client.createCqSummaryData(context.Background(), time.Now(), int64(1234), int64(1), false, map[string]IncrementalCoverageData{})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(errors.New("Datastore: Error putting entities")))
		})
	})
}
