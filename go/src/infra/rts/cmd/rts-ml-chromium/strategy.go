// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"math"
	"path"
	"regexp"
	"strings"
	"sync"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/errors/errtag"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/rts"
	"go.chromium.org/infra/rts/filegraph/git"
	"go.chromium.org/infra/rts/internal/chromium"
	"go.chromium.org/infra/rts/presubmit/eval"
	evalpb "go.chromium.org/infra/rts/presubmit/eval/proto"
)

// mustAlwaysRunTest returns true if the test file must never be skipped.
func mustAlwaysRunTest(testFile string) bool {
	switch {
	// Always run all third-party tests (never skip them),
	// except //third_party/blink which is actually first party.
	case strings.Contains(testFile, "/third_party/") && !strings.HasPrefix(testFile, "//third_party/blink/"):
		return true

	case testFile == "//third_party/blink/web_tests/wpt_internal/webgpu/cts.html":
		// Most of cts.html commits are auto-generated.
		// https://source.chromium.org/chromium/chromium/src/+/HEAD:third_party/blink/web_tests/wpt_internal/webgpu/cts.html;l=5;bpv=1;bpt=0
		// cts.html does not have meaningful edges in the file graph.
		return true

	default:
		return false
	}
}

var (
	// requireAllTests is a list of patterns of files that require running all
	// tests.
	requireAllTests = []string{
		// A CL changes the way tests run or their configurations.
		"//testing/.+",

		// The full list of modified files is not available, and the
		// graph does not include DEPSed file changes anyway.
		"//DEPS",

		// MB CLs change the way tests run or their configurations.
		"//tools/mb/.+",
	}
	requireAllTestsRegexp = regexp.MustCompile(fmt.Sprintf("^(%s)$", strings.Join(requireAllTests, "|")))

	disableRTS = errtag.Make("skip RTS", true)
)

// selectIndividualTests calls skipFile for individual tests that should be
// skipped additionally using the provided ML model to infer features.
// May return an error annotated with disableRTS tag and the message explaining
// why RTS was disabled.
func (r *selectRun) selectTests(ctx context.Context, skipTest func(string, string) error) (err error) {
	// Disable RTS if the number of files is unusual.
	if len(r.ChangedFiles) < chromium.MinChangedFiles || len(r.ChangedFiles) > chromium.MaxChangedFiles {
		return errors.Reason(
			"%d files were changed, which is outside of [%d, %d] range",
			len(r.ChangedFiles),
			chromium.MinChangedFiles,
			chromium.MaxChangedFiles,
		).Tag(disableRTS).Err()
	}

	// Check if any of the changed files requires all tests.
	if !r.IgnoreExceptions {
		for f := range r.ChangedFiles {
			if requireAllTestsRegexp.MatchString(f) {
				return errors.Reason(
					"%q was changed, which matches regexp %s",
					f,
					requireAllTests,
				).Tag(disableRTS).Err()
			}
		}
	}

	r.testToExamples = map[testFilterTarget]*mlExample{}
	err = r.addGitDistanceFeatures(ctx)
	if err != nil {
		return err
	}

	err = r.addFileDistanceFeatures()
	if err != nil {
		return err
	}

	err = r.addStabilityFeatures()
	if err != nil {
		return err
	}

	var allRows []*mlExample
	var testFilterTargets []testFilterTarget
	for filterTarget, example := range r.testToExamples {
		allRows = append(allRows, example)
		testFilterTargets = append(testFilterTargets, filterTarget)
	}

	predictions, err := fileInferMlModel(ctx, allRows, r.ModelDir)

	for i := range predictions {
		if predictions[i] > r.Strategy.MaxDistance {
			err = skipTest(testFilterTargets[i].testSuite, testFilterTargets[i].testName)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (r *selectRun) addStabilityFeatures() error {
	for testName, testStability := range r.stability {
		filterTarget := testFilterTarget{testSuite: testStability.TestSuite, testName: testName}
		if _, ok := r.testToExamples[filterTarget]; ok {
			r.testToExamples[filterTarget].SixMonthFailCount = testStability.Stability.SixMonthFailCount
			r.testToExamples[filterTarget].SixMonthRunCount = testStability.Stability.SixMonthRunCount
			r.testToExamples[filterTarget].OneMonthFailCount = testStability.Stability.OneMonthFailCount
			r.testToExamples[filterTarget].OneMonthRunCount = testStability.Stability.OneMonthRunCount
			r.testToExamples[filterTarget].OneWeekFailCount = testStability.Stability.OneWeekFailCount
			r.testToExamples[filterTarget].OneWeekRunCount = testStability.Stability.OneWeekRunCount
		} else {
			r.testToExamples[filterTarget] = &mlExample{
				SixMonthFailCount: testStability.Stability.SixMonthFailCount,
				SixMonthRunCount:  testStability.Stability.SixMonthRunCount,
				OneMonthFailCount: testStability.Stability.OneMonthFailCount,
				OneMonthRunCount:  testStability.Stability.OneMonthRunCount,
				OneWeekFailCount:  testStability.Stability.OneWeekFailCount,
				OneWeekRunCount:   testStability.Stability.OneWeekRunCount,
			}
		}
	}
	return nil
}

func (r *selectRun) addGitDistanceFeatures(ctx context.Context) error {
	r.Strategy.EdgeReader = &git.EdgeReader{
		ChangeLogDistanceFactor:     1,
		FileStructureDistanceFactor: 0,
	}
	missingFileCount := 0
	r.Strategy.RunQuery(r.ChangedFiles.ToSlice(), func(fileName string, af rts.Affectedness) bool {
		file, ok := r.TestFiles[fileName]
		if !ok {
			missingFileCount++
			return true
		}

		for _, testName := range file.TestNames {
			for _, testTarget := range file.TestTargets {
				filterTarget := testFilterTarget{testSuite: testTarget, testName: testName}
				if _, ok := r.testToExamples[filterTarget]; ok {
					r.testToExamples[filterTarget].UseGitDistance = true
					r.testToExamples[filterTarget].GitDistance = af.Distance
				} else {
					r.testToExamples[filterTarget] = &mlExample{
						UseGitDistance: true,
						GitDistance:    af.Distance,
					}
				}
			}
		}

		return true
	})
	logging.Warningf(ctx, "files not found: %d", missingFileCount)
	return nil
}

func (r *selectRun) addFileDistanceFeatures() error {
	r.Strategy.EdgeReader = &git.EdgeReader{
		ChangeLogDistanceFactor:     0,
		FileStructureDistanceFactor: 1,
	}

	r.Strategy.RunQuery(r.ChangedFiles.ToSlice(), func(fileName string, af rts.Affectedness) bool {
		file, ok := r.TestFiles[fileName]
		if !ok {
			// We don't have test file info for the test, skip it
			return true
		}

		for _, testName := range file.TestNames {
			for _, testTarget := range file.TestTargets {
				filterTarget := testFilterTarget{testSuite: testTarget, testName: testName}
				// Set the distances for all rows that use this file
				if _, ok := r.testToExamples[filterTarget]; ok {
					r.testToExamples[filterTarget].UseFileDistance = true
					r.testToExamples[filterTarget].FileDistance = af.Distance
				} else {
					r.testToExamples[filterTarget] = &mlExample{
						UseFileDistance: true,
						FileDistance:    af.Distance,
					}
				}
			}
		}
		// This file too close to skip it.
		return true
	})
	return nil
}

func (r *createModelRun) evalStrategy() eval.Strategy {
	onTestNotFound := func(ctx context.Context, tv *evalpb.TestVariant) {
		if strings.Contains(path.Base(tv.FileName), "autogen") {
			// This file is autogenerated.
			return
		}
		logging.Warningf(ctx, "test file not found: %s", tv.FileName)
	}
	gitStrategy := &git.SelectionStrategy{
		Graph: r.fg,
		EdgeReader: &git.EdgeReader{
			ChangeLogDistanceFactor:     1,
			FileStructureDistanceFactor: 0,
		},
		OnTestNotFound: onTestNotFound,
	}
	fileStrategy := &git.SelectionStrategy{
		Graph: r.fg,
		EdgeReader: &git.EdgeReader{
			ChangeLogDistanceFactor:     0,
			FileStructureDistanceFactor: 1,
		},
		OnTestNotFound: onTestNotFound,
	}
	var mu sync.Mutex

	return func(ctx context.Context, in eval.Input, out *eval.Output) error {
		for _, f := range in.ChangedFiles {
			switch {
			case f.Repo != "https://chromium.googlesource.com/chromium/src":
				return errors.Reason("unexpected repo %q", f.Repo).Err()
			case requireAllTestsRegexp.MatchString(f.Path):
				return nil
			}
		}

		// Get the git based distance
		if err := gitStrategy.SelectEval(ctx, in, out); err != nil {
			return err
		}
		gitDistances := make([]float64, len(in.TestVariants))
		for i := 0; i < len(gitDistances); i++ {
			gitDistances[i] = out.TestVariantAffectedness[i].Distance
		}

		// Get the file based distance
		if err := fileStrategy.SelectEval(ctx, in, out); err != nil {
			return err
		}
		fileDistances := make([]float64, len(in.TestVariants))
		for i := 0; i < len(fileDistances); i++ {
			fileDistances[i] = out.TestVariantAffectedness[i].Distance
		}

		// Create the examples to be inferred, using the appropriate day
		var examples = make([]*mlExample, len(in.TestVariants))
		for i := range in.TestVariants {

			example, ok := r.stabilityMap[stabilityMapKey{testID: in.TestVariants[i].Id, date: in.Timestamp}]
			if !ok {
				example = &mlExample{}
				mu.Lock()
				r.missingStabilities[in.TestVariants[i].Id] = struct{}{}
				mu.Unlock()
			}
			example.GitDistance = gitDistances[i]
			example.UseGitDistance = gitDistances[i] != 0.0 && !math.IsInf(gitDistances[i], 0)
			example.FileDistance = fileDistances[i]
			example.UseFileDistance = fileDistances[i] != 0.0 && !math.IsInf(fileDistances[i], 0)
			examples[i] = example
		}

		predictions, err := fileInferMlModel(ctx, examples, r.modelDir)

		if err != nil {
			return err
		}

		for i := range out.TestVariantAffectedness {
			out.TestVariantAffectedness[i] = rts.Affectedness{Distance: predictions[i]}
		}

		// No matter what filegraph said, never skip certain tests.
		for i, tv := range in.TestVariants {
			if mustAlwaysRunTest(tv.FileName) {
				out.TestVariantAffectedness[i] = rts.Affectedness{Distance: 0}
			}
		}
		return nil
	}
}
