// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tricium/api/v1"
)

const (
	noTestNoBug1 string = ""
	noTestNoBug2 string = "TEST=\nBUG\n"
	noTestNoBug3 string = " \nTeSt= \n \n BuG=  "
	noTestNoBug4 string = "\n.\n"
	badBug1      string = "BUG=b:"
	badBug2      string = "BUG=chromium:\n"
	badBug3      string = "BUG=chromium:asdf\n"
	good1        string = "a\nb\nTEST=did stuff\nBUG=b:123456\nc\nd"
	good2        string = "a\nb\nTEST= did more stuff...\nBUG= chromium:123456,\nc\nd"
)

func TestCommitcheck(t *testing.T) {

	ftt.Run("No comments to add when TEST= and BUG= found and formatted correctly", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		emptyResults := &tricium.Data_Results{}

		checkForTest(good1, results)
		assert.Loosely(t, results, should.Resemble(emptyResults))
		checkForTest(good2, results)
		assert.Loosely(t, results, should.Resemble(emptyResults))
		checkForBug(good1, results)
		assert.Loosely(t, results, should.Resemble(emptyResults))
		checkForBug(good2, results)
		assert.Loosely(t, results, should.Resemble(emptyResults))
	})

	ftt.Run("Leave comment when no TEST= or empty TEST= found", t, func(t *ftt.Test) {
		var comment = tricium.Data_Comment{
			Message:   "No TEST= or empty TEST= found in commit message.",
			Category:  "CommitCheck/NoTestFound",
			StartLine: 0,
		}
		expectedResults := &tricium.Data_Results{}
		expectedResults.Comments = append(expectedResults.Comments, &comment)

		noTestStrings := []string{noTestNoBug1, noTestNoBug2, noTestNoBug3, noTestNoBug4}
		for _, s := range noTestStrings {
			results := &tricium.Data_Results{}
			checkForTest(s, results)
			assert.Loosely(t, results, should.Resemble(expectedResults))
		}
	})

	ftt.Run("Leave comment when no BUG= found", t, func(t *ftt.Test) {
		var comment = tricium.Data_Comment{
			Message:   "No BUG= found in commit message.",
			Category:  "CommitCheck/NoBugFound",
			StartLine: 0,
		}
		expectedResults := &tricium.Data_Results{}
		expectedResults.Comments = append(expectedResults.Comments, &comment)

		noBugStrings := []string{noTestNoBug1, noTestNoBug2, noTestNoBug3, noTestNoBug4}
		for _, s := range noBugStrings {
			results := &tricium.Data_Results{}
			checkForBug(s, results)
			assert.Loosely(t, results, should.Resemble(expectedResults))
		}
	})

	ftt.Run("Leave comment when BUG= has incorrect format", t, func(t *ftt.Test) {
		var comment = tricium.Data_Comment{
			Message:   "No valid bug found. Use format b:123 or chromium:123.",
			Category:  "CommitCheck/InvalidBugDescription",
			StartLine: int32(1),
		}
		expectedResults := &tricium.Data_Results{}
		expectedResults.Comments = append(expectedResults.Comments, &comment)

		badBugStrings := []string{badBug1, badBug2, badBug3}
		for _, s := range badBugStrings {
			results := &tricium.Data_Results{}
			checkForBug(s, results)
			assert.Loosely(t, results, should.Resemble(expectedResults))
		}
	})
}
