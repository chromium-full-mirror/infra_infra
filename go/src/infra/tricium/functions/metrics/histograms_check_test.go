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
	findingspb "go.chromium.org/luci/common/proto/findings"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const (
	emptyPatch = "empty_diff.patch"
	inputDir   = "testdata/src"
	enumsPath  = "testdata/src/enums/enums.xml"
)

func analyzeHistogramTestFileAll(t testing.TB, filePath, patch, prevDir string) ([]*findingspb.Finding, stringset.Set, stringset.Set) {
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

func analyzeHistogramTestFile(t testing.TB, filePath, patch, prevDir string) []*findingspb.Finding {
	findings, _, _ := analyzeHistogramTestFileAll(t, filePath, patch, prevDir)
	return findings
}

func analyzeHistogramSuffixesTestFile(t testing.TB, filePath, patch string) []*findingspb.Finding {
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
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with no errors: multi element enum no baseline", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "enums/enum_tests/multi_element_no_baseline.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with error: single element enum with no baseline", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "enums/enum_tests/single_element_no_baseline.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       singleElementEnumWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "enums/enum_tests/single_element_no_baseline.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 42,
						EndColumn:   66,
					},
				},
			},
		}))
	})

	// EXPIRY tests

	ftt.Run("Analyze XML file with no errors: good expiry date", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/good_date.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with no errors: good expiry date, expiry on new line", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/expiry_new_line.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with no expiry", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/no_expiry.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       noExpiryError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "expiry/no_expiry.xml",
					Range: &findingspb.Location_Range{
						StartLine: 3,
						EndLine:   3,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry of never", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/never_expiry_with_comment.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       neverExpiryInfo,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_INFO,
				Location: &findingspb.Location{
					FilePath: "expiry/never_expiry_with_comment.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   77,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry of never and no comment", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/never_expiry_no_comment.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       neverExpiryError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "expiry/never_expiry_no_comment.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   77,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry of never and no comment, expiry on new line", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/never_expiry_new_line.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       neverExpiryError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "expiry/never_expiry_new_line.xml",
					Range: &findingspb.Location_Range{
						StartLine:   4,
						EndLine:     4,
						StartColumn: 16,
						EndColumn:   37,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in over one year", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/over_year_expiry.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       farExpiryWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/over_year_expiry.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   82,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in past", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/past_expiry.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       pastExpiryWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/past_expiry.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   82,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with badly formatted expiry", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/unformatted_expiry.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       badExpiryError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "expiry/unformatted_expiry.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   82,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with reviving an already expired date", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/good_date.xml", "prevdata/tricium_date_data_discontinuity.patch", "prevdata/src")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       dataDiscontinuityWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/good_date.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   82,
					},
				},
			},
		}))
	})

	// EXPIRY MILESTONE tests

	ftt.Run("Analyze XML file with no errors: good milestone expiry", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/good_milestone.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Simulate failure in fetching milestone data from server", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/milestone_fetch_failed.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       milestoneFailure,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/milestone/milestone_fetch_failed.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   75,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in over one year: milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/over_year_milestone.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       farExpiryWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/milestone/over_year_milestone.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   75,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in over one year: 3-number milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/over_year_milestone_3.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       farExpiryWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/milestone/over_year_milestone_3.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   76,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with expiry in past: milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/past_milestone.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       pastExpiryWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/milestone/past_milestone.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   75,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with badly formatted expiry: similar to milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/unformatted_milestone.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       badExpiryError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "expiry/milestone/unformatted_milestone.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   76,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with reviving an already expired milestone", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "expiry/milestone/good_milestone.xml", "prevdata/tricium_milestone_data_discontinuity.patch", "prevdata/src")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       dataDiscontinuityWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "expiry/milestone/good_milestone.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 56,
						EndColumn:   75,
					},
				},
			},
		}))
	})

	// OWNER tests

	ftt.Run("Analyze XML file with no errors: both owners individuals", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/good_individuals.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with no errors: owner in <variants>", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/variants_one_owner.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with error: only one owner", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/one_owner.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       oneOwnerError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "owners/one_owner.xml",
					Range: &findingspb.Location_Range{
						StartLine: 4,
						EndLine:   4,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with error: no owners", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/no_owners.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       oneOwnerError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "owners/no_owners.xml",
					Range: &findingspb.Location_Range{
						StartLine: 3,
						EndLine:   3,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is team", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_team_owner.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       firstOwnerTeamError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "owners/first_team_owner.xml",
					Range: &findingspb.Location_Range{
						StartLine: 4,
						EndLine:   5,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is OWNERS file", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_owner_file.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       firstOwnerTeamError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "owners/first_owner_file.xml",
					Range: &findingspb.Location_Range{
						StartLine: 4,
						EndLine:   5,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is team, only one owner", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_team_one_owner.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       oneOwnerTeamError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "owners/first_team_one_owner.xml",
					Range: &findingspb.Location_Range{
						StartLine: 4,
						EndLine:   4,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with error: first owner is OWNERS file, only one owner", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "owners/first_file_one_owner.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       oneOwnerTeamError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "owners/first_file_one_owner.xml",
					Range: &findingspb.Location_Range{
						StartLine: 4,
						EndLine:   4,
					},
				},
			},
		}))
	})

	// UNITS tests

	ftt.Run("Analyze XML file no errors, units of microseconds, all users", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_all_users.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file no errors, units of microseconds, high-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_high_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file no errors, units of microseconds, low-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_low_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with error: units of microseconds, bad summary", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/microseconds_bad_summary.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       unitsHighResolutionWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "units/microseconds_bad_summary.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 42,
						EndColumn:   62,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file no errors, units of us, all users", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_all_users.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file no errors, units of us, high-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_high_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file no errors, units of us, low-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_low_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with error: units of us, bad summary", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/us_bad_summary.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       unitsHighResolutionWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "units/us_bad_summary.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 42,
						EndColumn:   52,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file no errors, units of usec, all users", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_all_users.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file no errors, units of usec, high-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_high_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file no errors, units of usec, low-resolution", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_low_res.xml", patchPath, inputDir)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with error: units of usec, bad summary", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "units/usec_bad_summary.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       unitsHighResolutionWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "units/usec_bad_summary.xml",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 42,
						EndColumn:   54,
					},
				},
			},
		}))
	})

	// HISTOGRAM_SUFFIXES_LIST tests
	ftt.Run("Analyze histogram suffixes file, update an existing <histogram_suffixes>", t, func(t *ftt.Test) {
		results := analyzeHistogramSuffixesTestFile(t, "suffixes/histogram_suffixes_list.xml", "prevdata/add_new_suffix_diff.patch")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       SuffixesDeprecationWarning,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "suffixes/histogram_suffixes_list.xml",
					Range: &findingspb.Location_Range{
						StartLine: 6,
						EndLine:   6,
					},
				},
			},
		}))
	})

	// ADDED AND REMOVED HISTOGRAM tests

	ftt.Run("Analyze XML file with no histogram added or removed: only owner line deleted", t, func(t *ftt.Test) {
		findings, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t, "rm/remove_owner_line.xml",
			"prevdata/tricium_owner_line_diff.patch", "prevdata/src")
		assert.Loosely(t, findings, should.BeEmpty)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with no histogram added or removed: only attribute changed", t, func(t *ftt.Test) {
		findings, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/change_attribute.xml", "prevdata/tricium_attribute_diff.patch", "prevdata/src")
		assert.Loosely(t, findings, should.BeEmpty)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with histogram(s) removed", t, func(t *ftt.Test) {
		findings, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/remove_histogram.xml", "prevdata/tricium_generated_diff.patch", "prevdata/src")
		assert.Loosely(t, findings, should.BeEmpty)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.Match(stringset.NewFromSlice([]string{"Test.Histogram2"}...)))
	})

	ftt.Run("Analyze XML file with patterned histogram(s) removed", t, func(t *ftt.Test) {
		findings, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/remove_patterned_histogram.xml", "prevdata/tricium_remove_patterned_histogram_diff.patch", "prevdata/src")
		assert.Loosely(t, findings, should.BeEmpty)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.Match(stringset.NewFromSlice(
			[]string{"TestDragon.Histogram2.Bulbasaur", "TestDragon.Histogram2.Charizard",
				"TestFlying.Histogram2.Bulbasaur", "TestFlying.Histogram2.Charizard"}...)))
	})

	ftt.Run("Analyze XML file with <variants> modified to remove a variant", t, func(t *ftt.Test) {
		findings, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/modify_variants.xml", "prevdata/tricium_modify_variants_diff.patch", "prevdata/src")
		assert.Loosely(t, findings, should.BeEmpty)
		assert.Loosely(t, addedHistograms, should.BeEmpty)
		assert.Loosely(t, removedHistograms, should.Match(stringset.NewFromSlice(
			[]string{"TestDragon.Histogram2.Charizard", "TestFlying.Histogram2.Charizard"}...)))
	})

	ftt.Run("Analyze XML file with histogram(s) added and removed", t, func(t *ftt.Test) {
		findings, addedHistograms, removedHistograms := analyzeHistogramTestFileAll(t,
			"rm/add_remove_histogram.xml", "prevdata/tricium_patterned_histogram_diff.patch", "prevdata/src")
		assert.Loosely(t, findings, should.BeEmpty)
		assert.Loosely(t, addedHistograms, should.Match(stringset.NewFromSlice([]string{"Test.Histogram99"}...)))
		assert.Loosely(t, removedHistograms, should.Match(stringset.NewFromSlice([]string{"Test.Histogram2"}...)))
	})

	// COMMIT MESSAGE tests

	ftt.Run("Analyze commit message with no error: no histogram removed and no obsoletion tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, emptyHistogramSet, false)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze commit message with no error: histograms removed and a CL-level obsoletion tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...), true)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze commit message with no error: histograms removed and histogram specific obsoletion tags added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...),
			stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...), false)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze commit message with no error: histograms removed and a CL-level and a histogram specific obsoletion tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...),
			stringset.NewFromSlice([]string{"Test.Histogram", "Test.Histogram2"}...), true)
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze commit message with a CL-level obsoletion message tag added but no histogram removed", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, emptyHistogramSet, true)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       globalObsoletionMessageError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "/COMMIT_MSG",
				},
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram specific obsoletion message tag added but no histogram removed", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...), emptyHistogramSet, false)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(obsoletionMessageError, "Test.Histogram"),
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "/COMMIT_MSG",
				},
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram removed without an obsoletion message tag", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(emptyHistogramSet, stringset.NewFromSlice([]string{"Test.Histogram"}...), false)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(removedHistogramInfo, "Test.Histogram"),
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_INFO,
				Location: &findingspb.Location{
					FilePath: "/COMMIT_MSG",
				},
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram specific obsoletion message tag added with a typo", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...),
			stringset.NewFromSlice([]string{"Test.Histogram2"}...), false)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(obsoletionMessageError, "Test.Histogram"),
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "/COMMIT_MSG",
				},
			},
			{
				Category:      category,
				Message:       fmt.Sprintf(removedHistogramInfo, "Test.Histogram2"),
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_INFO,
				Location: &findingspb.Location{
					FilePath: "/COMMIT_MSG",
				},
			},
		}))
	})

	ftt.Run("Analyze commit message with a histogram specific obsoletion message tag added with a typo "+
		"and a CL-level obsoletion message tag added", t, func(t *ftt.Test) {
		results := analyzeCommitMessage(stringset.NewFromSlice([]string{"Test.Histogram"}...),
			stringset.NewFromSlice([]string{"Test.Histogram2", "Test.Histogram3"}...), true)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(obsoletionMessageError, "Test.Histogram"),
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "/COMMIT_MSG",
				},
			},
		}))
	})

	// ADDED NAMESPACE tests

	ftt.Run("Analyze XML file with no error: added histogram with same namespace", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/same_namespace.xml", "prevdata/tricium_same_namespace.patch", "prevdata/src")
		assert.Loosely(t, results, should.BeEmpty)
	})

	ftt.Run("Analyze XML file with warning: added namespace", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/add_namespace.xml", "prevdata/tricium_namespace_diff.patch", "prevdata/src")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       fmt.Sprintf(addedNamespaceWarning, "Test2"),
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "namespace/add_namespace.xml",
					Range: &findingspb.Location_Range{
						StartLine: 8,
						EndLine:   8,
					},
				},
			},
		}))
	})

	// DEPRECATED NAMESPACE tests

	ftt.Run("Analyze XML file with added histogram with a deprecated namespace>", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/add_deprecated_namespace.xml", patchPath, inputDir)
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       osxNamespaceDeprecationError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "namespace/add_deprecated_namespace.xml",
					Range: &findingspb.Location_Range{
						StartLine: 3,
						EndLine:   4,
					},
				},
			},
		}))
	})

	ftt.Run("Analyze XML file with a change in a histogram with a deprecated namespace>", t, func(t *ftt.Test) {
		results := analyzeHistogramTestFile(t, "namespace/change_deprecated_namespace.xml", "prevdata/change_deprecated_namespace.patch", "prevdata/src")
		assert.That(t, results, should.Match([]*findingspb.Finding{
			{
				Category:      category,
				Message:       osxNamespaceDeprecationError,
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "namespace/change_deprecated_namespace.xml",
					Range: &findingspb.Location_Range{
						StartLine: 3,
						EndLine:   4,
					},
				},
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
