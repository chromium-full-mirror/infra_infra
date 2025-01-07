// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

func TestCheckSpaceMix(t *testing.T) {
	ftt.Run("Finds tab + single space mix", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("test.file", "\t code", 1), should.Match(&tricium.Data_Comment{
			Path:      "test.file",
			Category:  "spacey_space_mix",
			Message:   "Found mix of white space characters",
			StartLine: 1,
			EndLine:   1,
			StartChar: 0,
			EndChar:   1,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "         code",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     6,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with spaces",
				},
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "\tcode",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     6,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with tabs",
				},
			},
		}))
	})

	ftt.Run("Finds tab + multiple space mix", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("test.file", "\t  code", 1), should.Match(&tricium.Data_Comment{
			Path:      "test.file",
			Category:  "spacey_space_mix",
			Message:   "Found mix of white space characters",
			StartLine: 1,
			EndLine:   1,
			StartChar: 0,
			EndChar:   2,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "          code",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     7,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with spaces",
				},
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "\tcode",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     7,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with tabs",
				},
			},
		}))
	})

	ftt.Run("Finds space + tab mix", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("test.file", " \tcode", 1), should.Match(&tricium.Data_Comment{
			Path:      "test.file",
			Category:  "spacey_space_mix",
			Message:   "Found mix of white space characters",
			StartLine: 1,
			EndLine:   1,
			StartChar: 0,
			EndChar:   1,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "         code",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     6,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with spaces",
				},
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "\tcode",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     6,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with tabs",
				},
			},
		}))
	})

	ftt.Run("Finds other whitespace mix", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("test.file", "\t\v\f...", 1), should.Match(&tricium.Data_Comment{
			Path:      "test.file",
			Category:  "spacey_space_mix",
			Message:   "Found mix of white space characters",
			StartLine: 1,
			EndLine:   1,
			StartChar: 0,
			EndChar:   2,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "        ...",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     6,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with spaces",
				},
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "\t...",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     6,
						},
					},
					Description: "Replace all whitespace at the beginning of the line with tabs",
				},
			},
		}))
	})

	ftt.Run("Produces no comment for mid-line space mix", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("test.file", "+ \tcode", 1), should.BeNil)
	})

	ftt.Run("Produces no comment in Makefile", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("Makefile", "\t  some code", 1), should.BeNil)
	})

	ftt.Run("Produces no comment in makefile with extension", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("my.mk", "\t  some code", 1), should.BeNil)
	})

	ftt.Run("Produces no comment in patch file", t, func(t *ftt.Test) {
		assert.Loosely(t, checkSpaceMix("my.patch", " \t\tsome code", 1), should.BeNil)
	})
}

func TestCheckTrailingSpace(t *testing.T) {
	ftt.Run("Finds single trailing space", t, func(t *ftt.Test) {
		assert.Loosely(t, checkTrailingSpace("test.file", "code ", 1), should.Match(&tricium.Data_Comment{
			Path:      "test.file",
			Category:  "spacey_trailing_space",
			Message:   "Found trailing space",
			StartLine: 1,
			EndLine:   1,
			StartChar: 4,
			EndChar:   4,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "code",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     4,
						},
					},
					Description: "Get rid of trailing space",
				},
			},
		}))
	})

	ftt.Run("Finds multiple trailing spaces", t, func(t *ftt.Test) {
		assert.Loosely(t, checkTrailingSpace("test.file", "code  ", 1), should.Match(&tricium.Data_Comment{
			Path:      "test.file",
			Category:  "spacey_trailing_space",
			Message:   "Found trailing space",
			StartLine: 1,
			EndLine:   1,
			StartChar: 4,
			EndChar:   5,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							Replacement: "code",
							Path:        "test.file",
							StartLine:   1,
							EndLine:     1,
							StartChar:   0,
							EndChar:     5,
						},
					},
					Description: "Get rid of trailing space",
				},
			},
		}))
	})

	ftt.Run("Produces no comment in ignored file types", t, func(t *ftt.Test) {
		assert.Loosely(t, checkTrailingSpace("my.patch", " ", 1), should.BeNil)
		assert.Loosely(t, checkTrailingSpace("my.pdf", " ", 1), should.BeNil)
	})
}

func TestCheckTrailingLines(t *testing.T) {
	ftt.Run("Finds trailing lines at the end of a file", t, func(t *ftt.Test) {
		assert.Loosely(t, analyzeFile(bufio.NewScanner(strings.NewReader("some code\nsome more code\n\n\n")),
			"file.path"), should.Match([]*tricium.Data_Comment{
			{
				Category:  "spacey_trailing_lines",
				Message:   "Found empty line(s) at the end of the file",
				Path:      "file.path",
				StartLine: 3,
				EndLine:   5,
			},
		}))
	})
}

func TestMergingSimilarComments(t *testing.T) {
	ftt.Run("Merges multiple similar comments (TrailingSpace) together into one", t, func(t *ftt.Test) {
		inputComments := []*tricium.Data_Comment{
			{
				Path:      "test.file",
				Category:  "spacey_trailing_space",
				Message:   "Found trailing space",
				StartLine: 1,
				EndLine:   1,
				StartChar: 4,
				EndChar:   5,
			},
			{
				Path:      "test.file",
				Category:  "spacey_trailing_space",
				Message:   "Found trailing space",
				StartLine: 2,
				EndLine:   2,
				StartChar: 4,
				EndChar:   5,
			},
			{
				Path:      "test.file",
				Category:  "spacey_trailing_space",
				Message:   "Found trailing space",
				StartLine: 1,
				EndLine:   1,
				StartChar: 3,
				EndChar:   99,
			},
			{
				Path:      "test.file",
				Category:  "spacey_trailing_space",
				Message:   "Found trailing space",
				StartLine: 9,
				EndLine:   9,
				StartChar: 43,
				EndChar:   51,
			},
		}
		expectedComments := []*tricium.Data_Comment{{
			Path:     "test.file",
			Category: "spacey_trailing_space",
			Message:  "Found 4 spacey_trailing_space warnings in this file",
		}}

		organizedComments := organizeCommentsByCategory(inputComments)
		assert.Loosely(t, mergeComments(organizedComments, "test.file"), should.Match(expectedComments))
	})

	ftt.Run("Keeps similar comments separate if their number of occurrences is below set limit", t, func(t *ftt.Test) {
		inputComments := []*tricium.Data_Comment{
			{
				Path:      "test.file",
				Category:  "spacey_trailing_space",
				Message:   "Found trailing space",
				StartLine: 1,
				EndLine:   1,
				StartChar: 4,
				EndChar:   5,
			},
			{
				Path:      "test.file",
				Category:  "spacey_trailing_space",
				Message:   "Found trailing space",
				StartLine: 2,
				EndLine:   2,
				StartChar: 4,
				EndChar:   5,
			},
		}

		organizedComments := organizeCommentsByCategory(inputComments)
		assert.Loosely(t, mergeComments(organizedComments, "test.file"), should.Match(inputComments))
	})
}
