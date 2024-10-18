// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

const (
	emptyPatch = "empty_diff.patch"
	inputDir   = "testdata/src"
	enumsPath  = "testdata/src/enums/enums.xml"
)

func analyzeHistogramTestFileAll(t testing.TB, filePath, patch, prevDir string) ([]*tricium.Data_Comment, stringset.Set, stringset.Set) {
	// now mocks the current time for testing.
	now = func() time.Time { return time.Date(2019, time.September, 18, 0, 0, 0, 0, time.UTC) }
	// getMilestoneDate is a function that mocks getting the milestone date from server.
	getMilestoneDate = func(milestone int) (time.Time, error) {
		var date time.Time
		var err error
		switch milestone {
		// Use 50 to simulate if server responds with error.
		case 50:
			err = errors.New("Bad milestone request")
		case 60:
			date, _ = time.Parse(dateMilestoneFormat, "2018-08-25T00:00:00")
		case 77:
			date, _ = time.Parse(dateMilestoneFormat, "2019-08-25T00:00:00")
		case 79:
			date, _ = time.Parse(dateMilestoneFormat, "2019-10-17T00:00:00")
		case 83:
			date, _ = time.Parse(dateMilestoneFormat, "2020-04-23T00:00:00")
		case 87:
			date, _ = time.Parse(dateMilestoneFormat, "2020-10-22T00:00:00")
		case 88:
			date, _ = time.Parse(dateMilestoneFormat, "2020-11-12T00:00:00")
		case 101:
			date, _ = time.Parse(dateMilestoneFormat, "2022-08-11T00:00:00")
		default:
			t.Errorf("Invalid milestone date in test. Please add your own case")
		}
		return date, err
	}
	filesChanged, err := getDiffsPerFile([]string{filePath}, patch)
	if err != nil {
		t.Errorf("Failed to get diffs per file for %s: %v", filePath, err)
	}
	if patch == filepath.Join(inputDir, emptyPatch) {
		// Assumes all test files are less than 100 lines in length.
		// This is necessary to ensure all lines in the test file are analyzed.
		filesChanged.addedLines[filePath] = makeRange(1, 100)
		filesChanged.removedLines[filePath] = makeRange(1, 100)
	}
	singletonEnums := getSingleElementEnums(enumsPath)
	inputPath := filepath.Join(inputDir, filePath)
	f := openFileOrDie(inputPath)
	defer closeFileOrDie(f)
	return analyzeHistogramFile(f, filePath, prevDir, filesChanged, singletonEnums)
}

func analyzeHistogramTestFile(t testing.TB, filePath, patch, prevDir string) []*tricium.Data_Comment {
	comments, _, _ := analyzeHistogramTestFileAll(t, filePath, patch, prevDir)
	return comments
}

func analyzeHistogramSuffixesTestFile(t testing.TB, filePath, patch string) []*tricium.Data_Comment {
	filesChanged, err := getDiffsPerFile([]string{filePath}, patch)
	if err != nil {
		t.Errorf("Failed to get diffs per file for %s: %v", filePath, err)
	}
	inputPath := filepath.Join(inputDir, filePath)
	f := openFileOrDie(inputPath)
	defer closeFileOrDie(f)
	return analyzeHistogramSuffixesFile(f, filePath, filesChanged)
}

func TestHistogramsCheck(t *testing.T) {
	// An empty histogram set. Used as an empty removed histogram set or an empty obsoleted histogram set.
	emptyHistogramSet := make(stringset.Set)

	patchPath := filepath.Join(inputDir, emptyPatch)
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Errorf("Failed to create empty patch file %s: %v", patchPath, err)
		return
	}
	patchFile.Close()
	defer os.Remove(patchPath)

	// ENUM tests

	ftt.Run("Analyze XML file with no errors: single element enum with baseline", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "enums/enum_tests/single_element_baseline.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with no errors: multi element enum no baseline", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "enums/enum_tests/multi_element_no_baseline.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with error: single element enum with no baseline", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "enums/enum_tests/single_element_no_baseline.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Enums",
				Message:              singleElementEnumWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            42,
				EndChar:              66,
				Path:                 "enums/enum_tests/single_element_no_baseline.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// EXPIRY tests

	ftt.Run("Analyze XML file with no errors: good expiry date", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/good_date.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with no errors: good expiry date, expiry on new line", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/expiry_new_line.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with no expiry", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/no_expiry.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              noExpiryError,
				StartLine:            3,
				EndLine:              3,
				Path:                 "expiry/no_expiry.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry of never", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/never_expiry_with_comment.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              neverExpiryInfo,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              77,
				Path:                 "expiry/never_expiry_with_comment.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry of never and no comment", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/never_expiry_no_comment.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              neverExpiryError,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              77,
				Path:                 "expiry/never_expiry_no_comment.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry of never and no comment, expiry on new line", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/never_expiry_new_line.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              neverExpiryError,
				StartLine:            4,
				EndLine:              4,
				StartChar:            16,
				EndChar:              37,
				Path:                 "expiry/never_expiry_new_line.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in over one year", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/over_year_expiry.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              farExpiryWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              82,
				Path:                 "expiry/over_year_expiry.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in past", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/past_expiry.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              pastExpiryWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              82,
				Path:                 "expiry/past_expiry.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with badly formatted expiry", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/unformatted_expiry.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              badExpiryError,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              82,
				Path:                 "expiry/unformatted_expiry.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with reviving an already expired date", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/good_date.xml", "prevdata/tricium_date_data_discontinuity.patch", "prevdata/src")
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              dataDiscontinuityWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              82,
				Path:                 "expiry/good_date.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// EXPIRY MILESTONE tests

	ftt.Run("Analyze XML file with no errors: good milestone expiry", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/good_milestone.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Simulate failure in fetching milestone data from server", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/milestone_fetch_failed.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              milestoneFailure,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              75,
				Path:                 "expiry/milestone/milestone_fetch_failed.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in over one year: milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/over_year_milestone.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              farExpiryWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              75,
				Path:                 "expiry/milestone/over_year_milestone.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in over one year: 3-number milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/over_year_milestone_3.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              farExpiryWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              76,
				Path:                 "expiry/milestone/over_year_milestone_3.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in past: milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/past_milestone.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              pastExpiryWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              75,
				Path:                 "expiry/milestone/past_milestone.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with badly formatted expiry: similar to milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/unformatted_milestone.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              badExpiryError,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              76,
				Path:                 "expiry/milestone/unformatted_milestone.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with reviving an already expired milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/good_milestone.xml", "prevdata/tricium_milestone_data_discontinuity.patch", "prevdata/src")
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Expiry",
				Message:              dataDiscontinuityWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            56,
				EndChar:              75,
				Path:                 "expiry/milestone/good_milestone.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// OWNER tests

	ftt.Run("Analyze XML file with no errors: both owners individuals", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/good_individuals.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with no errors: owner in <variants>", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/variants_one_owner.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with error: only one owner", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/one_owner.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Owners",
				Message:              oneOwnerError,
				StartLine:            4,
				EndLine:              4,
				Path:                 "owners/one_owner.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with error: no owners", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/no_owners.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Owners",
				Message:              oneOwnerError,
				StartLine:            3,
				EndLine:              3,
				Path:                 "owners/no_owners.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is team", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_team_owner.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Owners",
				Message:              firstOwnerTeamError,
				StartLine:            4,
				EndLine:              5,
				Path:                 "owners/first_team_owner.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is OWNERS file", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_owner_file.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Owners",
				Message:              firstOwnerTeamError,
				StartLine:            4,
				EndLine:              5,
				Path:                 "owners/first_owner_file.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is team, only one owner", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_team_one_owner.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Owners",
				Message:              oneOwnerTeamError,
				StartLine:            4,
				EndLine:              4,
				Path:                 "owners/first_team_one_owner.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is OWNERS file, only one owner", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_file_one_owner.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Owners",
				Message:              oneOwnerTeamError,
				StartLine:            4,
				EndLine:              4,
				Path:                 "owners/first_file_one_owner.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// UNITS tests

	ftt.Run("Analyze XML file no errors, units of microseconds, all users", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_all_users.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file no errors, units of microseconds, high-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_high_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file no errors, units of microseconds, low-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_low_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with error: units of microseconds, bad summary", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_bad_summary.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Units",
				Message:              unitsHighResolutionWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            42,
				EndChar:              62,
				Path:                 "units/microseconds_bad_summary.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file no errors, units of us, all users", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_all_users.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file no errors, units of us, high-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_high_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file no errors, units of us, low-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_low_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with error: units of us, bad summary", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_bad_summary.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Units",
				Message:              unitsHighResolutionWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            42,
				EndChar:              52,
				Path:                 "units/us_bad_summary.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file no errors, units of usec, all users", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_all_users.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file no errors, units of usec, high-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_high_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file no errors, units of usec, low-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_low_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with error: units of usec, bad summary", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_bad_summary.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Units",
				Message:              unitsHighResolutionWarning,
				StartLine:            3,
				EndLine:              3,
				StartChar:            42,
				EndChar:              54,
				Path:                 "units/usec_bad_summary.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// HISTOGRAM_SUFFIXES_LIST tests
	ftt.Run("Analyze histogram suffixes file, update an existing <histogram_suffixes>", t, func(t *ftt.Test) {
		results := analyzeHistogramSuffixesTestFile(t, "suffixes/histogram_suffixes_list.xml", "prevdata/add_new_suffix_diff.patch")
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Suffixes",
				Message:              SuffixesDeprecationWarning,
				StartLine:            6,
				EndLine:              6,
				Path:                 "suffixes/histogram_suffixes_list.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// ADDED AND REMOVED HISTOGRAM tests

	ftt.Run("Analyze XML file with no histogram added or removed: only owner line deleted", t, func(t *ftt.Test) {
		comments, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t, "rm/remove_owner_line.xml",
			"prevdata/tricium_owner_line_diff.patch", "prevdata/src")
		assert.Loosely(t, comments, should.BeNil)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with no histogram added or removed: only attribute changed", t, func(t *ftt.Test) {
		comments, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/change_attribute.xml", "prevdata/tricium_attribute_diff.patch", "prevdata/src")
		assert.Loosely(t, comments, should.BeNil)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with histogram(s) removed", t, func(t *ftt.Test) {
		comments, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/remove_histogram.xml", "prevdata/tricium_generated_diff.patch", "prevdata/src")
		assert.Loosely(t, comments, should.BeNil)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.Resemble(stringset.NewFromSlice([]string{"Test.Histogram2"}...)))
	})

	ftt.Run("Analyze XML file with patterned histogram(s) removed", t, func(t *ftt.Test) {
		comments, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/remove_patterned_histogram.xml", "prevdata/tricium_remove_patterned_histogram_diff.patch", "prevdata/src")
		assert.Loosely(t, comments, should.BeNil)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.Resemble(stringset.NewFromSlice(
			[]string{"TestDragon.Histogram2.Bulbasaur", "TestDragon.Histogram2.Charizard",
				"TestFlying.Histogram2.Bulbasaur", "TestFlying.Histogram2.Charizard"}...)))
	})

	ftt.Run("Analyze XML file with <variants> modified to remove a variant", t, func(t *ftt.Test) {
		comments, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/modify_variants.xml", "prevdata/tricium_modify_variants_diff.patch", "prevdata/src")
		assert.Loosely(t, comments, should.BeNil)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.Resemble(stringset.NewFromSlice(
			[]string{"TestDragon.Histogram2.Charizard", "TestFlying.Histogram2.Charizard"}...)))
	})

	ftt.Run("Analyze XML file with histogram(s) added and removed", t, func(t *ftt.Test) {
		comments, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/add_remove_histogram.xml", "prevdata/tricium_patterned_histogram_diff.patch", "prevdata/src")
		assert.Loosely(t, comments, should.BeNil)
		assert.Loosely(t, addedHistograms, should.Resemble(stringset.NewFromSlice([]string{"Test.Histogram99"}...)))
		assert.Loosely(t, removedHistograms, should.Resemble(stringset.NewFromSlice([]string{"Test.Histogram2"}...)))
	})

	// COMMIT MESSAGE tests

	ftt.Run("Analyze commit message with no error: no histogram removed and no obsoletion tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, emptyHistogramSet, false)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze commit message with no error: histograms removed and a CL-level obsoletion tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...), true)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze commit message with no error: histograms removed and histogram specific obsoletion tags added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...),
			stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...), false)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze commit message with no error: histograms removed and a CL-level and a histogram specific obsoletion tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...),
			stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...), true)
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze commit message with a CL-level obsoletion message tag added but no histogram removed", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, emptyHistogramSet, true)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category: category + "/Obsolete",
				Message:  globalObsoletionMessageError,
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram specific obsoletion message tag added but no histogram removed", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...), emptyHistogramSet, false)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category: category + "/Obsolete",
				Message:  fmt.Sprintf(obsoletionMessageError, "Test.Histogram"),
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram removed without an obsoletion message tag", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, stringset.NewFromSlice([]string{"Test.Histogram"}...), false)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category: category + "/Obsolete",
				Message:  fmt.Sprintf(removedHistogramInfo, "Test.Histogram"),
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram specific obsoletion message tag added with a typo", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...),
			stringset.NewFromSlice([]string{"Test.Histogram2"}...), false)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category: category + "/Obsolete",
				Message:  fmt.Sprintf(obsoletionMessageError, "Test.Histogram"),
			},
			{
				Category: category + "/Obsolete",
				Message:  fmt.Sprintf(removedHistogramInfo, "Test.Histogram2"),
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram specific obsoletion message tag added with a typo "+
		"and a CL-level obsoletion message tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...),
			stringset.NewFromSlice([]string{"Test.Histogram2", "Test.Histogram3"}...), true)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category: category + "/Obsolete",
				Message:  fmt.Sprintf(obsoletionMessageError, "Test.Histogram"),
			},
		}))
	})

	// ADDED NAMESPACE tests

	ftt.Run("Analyze XML file with no error: added histogram with same namespace", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/same_namespace.xml", "prevdata/tricium_same_namespace.patch", "prevdata/src")
		assert.Loosely(t, results, should.BeNil)
	})

	ftt.Run("Analyze XML file with warning: added namespace", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/add_namespace.xml", "prevdata/tricium_namespace_diff.patch", "prevdata/src")
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Namespace",
				Message:              fmt.Sprintf(addedNamespaceWarning, "Test2"),
				Path:                 "namespace/add_namespace.xml",
				StartLine:            8,
				EndLine:              8,
				ShowOnUnchangedLines: true,
			},
		}))
	})

	// DEPRECATED NAMESPACE tests

	ftt.Run("Analyze XML file with added histogram with a deprecated namespace>", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/add_deprecated_namespace.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Namespace",
				Message:              osxNamespaceDeprecationError,
				StartLine:            3,
				EndLine:              4,
				Path:                 "namespace/add_deprecated_namespace.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

	ftt.Run("Analyze XML file with a change in a histogram with a deprecated namespace>", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/change_deprecated_namespace.xml", "prevdata/change_deprecated_namespace.patch", "prevdata/src")
		assert.Loosely(t, results, should.Resemble([]*tricium.Data_Comment{
			{
				Category:             category + "/Namespace",
				Message:              osxNamespaceDeprecationError,
				StartLine:            3,
				EndLine:              4,
				Path:                 "namespace/change_deprecated_namespace.xml",
				ShowOnUnchangedLines: true,
			},
		}))
	})

}

func makeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}
