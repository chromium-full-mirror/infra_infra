// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

func TestGosecWrapper(t *testing.T) {

	ftt.Run("Test min function", t, func(t *ftt.Test) {
		assert.Loosely(t, min(3, 4), should.Match(3))
		assert.Loosely(t, min(-1, 4), should.Match(-1))
		assert.Loosely(t, min(-3, -4), should.Match(-4))
		assert.Loosely(t, min(0, 0), should.BeZero)
	})

	ftt.Run("hashIssue function is implementation of sha256", t, func(t *ftt.Test) {
		issue := Issue{
			Severity:   "HIGH",
			Confidence: "LOW",
			RuleID:     "G101",
			Details:    "test",
			File:       "./test/example.py",
			Code:       "",
			Line:       "10",
		}
		other := issue
		expected := [32]uint8{
			110, 157, 73, 217, 74, 91, 117, 141, 251, 137, 228, 144, 93, 155, 215,
			152, 226, 0, 255, 83, 42, 178, 23, 140, 77, 241, 151, 53, 67, 141, 116, 1}

		assert.Loosely(t, hashIssue(&issue), should.Match(expected))
		// Assert x = y -> f(x) = f(y)
		assert.Loosely(t, hashIssue(&issue), should.Match(hashIssue(&other)))
	})

	ftt.Run("Test postProcess function", t, func(t *ftt.Test) {
		*inputDir = ""
		f1, _ := filepath.Abs("example.go")
		f2, _ := filepath.Abs("example2.go")
		f3, _ := filepath.Abs("example3.go")
		f4, _ := filepath.Abs("example4.go")

		issue1 := Issue{
			Severity:   "HIGH",
			Confidence: "LOW",
			RuleID:     "G101",
			Details:    "",
			File:       f1,
			Code:       "",
			Line:       "1",
		}
		issue1.Hash = hashIssue(&issue1)

		issue2 := Issue{
			Severity:   "HIGH",
			Confidence: "MEDIUM",
			RuleID:     "G101",
			Details:    "",
			File:       f2,
			Code:       "",
			Line:       "1",
		}
		issue2.Hash = hashIssue(&issue2)

		issue3 := Issue{
			Severity:   "HIGH",
			Confidence: "LOW",
			RuleID:     "G101",
			Details:    "",
			File:       f3,
			Code:       "",
			Line:       "2",
		}
		issue3.Hash = hashIssue(&issue3)

		issue4 := Issue{
			Severity:   "HIGH",
			Confidence: "LOW",
			RuleID:     "G103",
			Details:    "",
			File:       f4,
			Code:       "",
			Line:       "1",
		}
		issue4.Hash = hashIssue(&issue4)

		results := []GosecResult{
			{
				Issues: []Issue{
					issue1,
					issue2,
					issue3,
					issue4,
				},
			},
			{
				Issues: []Issue{
					issue1,
					issue2,
					issue3,
					issue4,
				},
			},
			{
				Issues: []Issue{
					issue1,
					issue2,
					issue3,
					issue4,
				},
			},
			{
				Issues: []Issue{
					issue1,
					issue2,
					issue3,
					issue4,
				},
			},
		}

		input := []*tricium.Data_File{
			{
				Path: "example.go",
			},
			{
				Path: "example2.go",
			},
		}

		assert.Loosely(t, len(postProcess(results, input, false)), should.Match(2))
	})

}
