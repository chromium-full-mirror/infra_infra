// Copyright 2018 The Chromium Authors
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

func TestSpellCheckerAnalyzeFiles(t *testing.T) {
	// These tests depend on both dictionary.txt and comment_formats.json.
	// TODO(qyearsley): Make the tests not depend on these files.
	cp := loadCommentFormats()
	dict = loadDict()

	ftt.Run("Analyzing simple file with one misspelling generates one comment", t, func(t *ftt.Test) {
		fileContent := "/* coment */"
		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"coment" is a possible misspelling of "comment".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 3,
					EndChar:   9,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "comment",
									StartLine:   1,
									EndLine:     1,
									StartChar:   3,
									EndChar:     9,
								},
							},
						},
					},
				},
			},
		}
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("Analyzing simple file with one misspelling generates one comment", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader("Updat the thing")), "", true, nil, results)
		assert.Loosely(t, results, should.Resemble(&tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "",
					Message:   `"Updat" is a possible misspelling of "Update".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 0,
					EndChar:   5,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "",
									Replacement: "Update",
									StartLine:   1,
									EndLine:     1,
									StartChar:   0,
									EndChar:     5,
								},
							},
						},
					},
				},
			},
		}))
	})

	ftt.Run("Words in all caps are not checked", t, func(t *ftt.Test) {
		fileContent := "/* DONT COMENT */"
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Words in URLs are not checked", t, func(t *ftt.Test) {
		fileContent := "/* See: https://exmaple.com/DontChek/wurds%20heare?x=NdAn43" +
			"And see also: http://exmaple.com/moar-wurds#framgent */"
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Words in TODO/FIXME notes are not flagged as misspellings.", t, func(t *ftt.Test) {
		// Note that just the part in the TODO is not checked; the comment
		// after the TODO is still checked.
		fileContent := "TODO(nams): do someting\nFIXME(zuser): fix me\n"
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.Resemble([]*tricium.Data_Comment{
			{
				Path:      "test.txt",
				Message:   `"someting" is a possible misspelling of "something".`,
				Category:  "SpellChecker",
				StartLine: 1,
				EndLine:   1,
				StartChar: 15,
				EndChar:   23,
				Suggestions: []*tricium.Data_Suggestion{
					{
						Description: "Misspelling fix suggestion",
						Replacements: []*tricium.Data_Replacement{
							{
								Path:        "test.txt",
								Replacement: "something",
								StartLine:   1,
								EndLine:     1,
								StartChar:   15,
								EndChar:     23,
							},
						},
					},
				},
			},
		}))
	})

	ftt.Run("TODO notes are not checcked even if there's no space beforehand.", t, func(t *ftt.Test) {
		fileContent := "//TODO(exmaple): ..."
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Email addresses are not checked.", t, func(t *ftt.Test) {
		fileContent := "nams@chromium.org"
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Email addresses are ignored regardless of prefix part.", t, func(t *ftt.Test) {
		fileContent := "...someting@chromium.org... \n\nTBR=alph@chromium.org\n"
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("URLs are ignored regardless of prefix part.", t, func(t *ftt.Test) {
		fileContent := "URL: (https://exmaple.com/urrl)"
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("TODOs are ignored even if there's no space beforehand", t, func(t *ftt.Test) {
		fileContent := "//TODO(exmaple): ..."
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Very short words are not checked, including in camel case", t, func(t *ftt.Test) {
		fileContent := ("wiAksYuo aa qstAbt ThsAdn tht Ue WihyFo AcnOtFof")
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Very long words with no whitespace are not checked", t, func(t *ftt.Test) {
		fileContent := ("wTichQtCwYkKxWiyhQCAJGBBUXYWYExmapleFTich")
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Words with non-ASCII characters are not split up", t, func(t *ftt.Test) {
		fileContent := "... François ..."
		// Example from crbug.com/996242.
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Words with apostrophes are not split up", t, func(t *ftt.Test) {
		fileContent := "... wasn't ..."
		// Example from crbug.com/996804.
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)

		// The same applies for "smart quote" apostrophes; from crbug.com/1238795.
		fileContent = "... wasn’t ..."
		results = &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Words in the ignore list are ignored.", t, func(t *ftt.Test) {
		fileContent := ("Errorprone referers supercede abbout vertexes;\n" +
			"And names like Donn Sargent, Wen Chang, Beng, Claus Wight, alle alph")
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, len(results.Comments), should.Equal(1))
		assert.Loosely(t, results.Comments[0].Message, should.Equal(`"abbout" is a possible misspelling of "about" or "abbot".`))
	})

	ftt.Run("Words previously in the ignorelist but no longer needed are still not flagged.", t, func(t *ftt.Test) {
		fileContent := ("Short words: bae ba cas dur ect gae iam ith lod mut que te tho tim wan wen yau\n" +
			"And copyable cloneable files' process' subprocess' thru wontfix; Anny amin bengr.\n")
		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results.Comments, should.BeEmpty)
	})

	ftt.Run("Analyzing a .c file with several comments.", t, func(t *ftt.Test) {
		fileContent := "// The misspelling iminent is mapped to three possible fixes.\n" +
			"This is not in a comment so aberation shouldn't be flagged.\n" +
			"// The word wanna has a reason to be disabled, so isn't flagged\n" +
			"/*Here are\ncombinatins of\nlines.\nAnd GAE is ignored.*/\n"

		// "iminent" has three suggested fixes: ["imminent" "immanent" "eminent"]
		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"iminent" is a possible misspelling of "imminent", "immanent", or "eminent".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 19,
					EndChar:   26,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "imminent",
									StartLine:   1,
									EndLine:     1,
									StartChar:   19,
									EndChar:     26,
								},
							},
						},
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "immanent",
									StartLine:   1,
									EndLine:     1,
									StartChar:   19,
									EndChar:     26,
								},
							},
						},
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "eminent",
									StartLine:   1,
									EndLine:     1,
									StartChar:   19,
									EndChar:     26,
								},
							},
						},
					},
				},
				{
					Path:      "test.c",
					Message:   `"combinatins" is a possible misspelling of "combinations".`,
					Category:  "SpellChecker",
					StartLine: 5,
					EndLine:   5,
					StartChar: 0,
					EndChar:   11,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "combinations",
									StartLine:   5,
									EndLine:     5,
									StartChar:   0,
									EndChar:     11,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("One line with both types of comment patterns", t, func(t *ftt.Test) {
		fileContent := "/*beggining//code*//*beccause*/\n"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"beggining" is a possible misspelling of "beginning".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 2,
					EndChar:   11,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "beginning",
									StartLine:   1,
									EndLine:     1,
									StartChar:   2,
									EndChar:     11,
								},
							},
						},
					},
				},
				{
					Path:      "test.c",
					Message:   `"beccause" is a possible misspelling of "because".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 21,
					EndChar:   29,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "because",
									StartLine:   1,
									EndLine:     1,
									StartChar:   21,
									EndChar:     29,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("Block comment across multiple lines", t, func(t *ftt.Test) {
		fileContent := "/*An\nabandonded\ncalcualtion.*/\n"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"abandonded" is a possible misspelling of "abandoned".`,
					Category:  "SpellChecker",
					StartLine: 2,
					EndLine:   2,
					StartChar: 0,
					EndChar:   10,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "abandoned",
									StartLine:   2,
									EndLine:     2,
									StartChar:   0,
									EndChar:     10,
								},
							},
						},
					},
				},
				{
					Path:      "test.c",
					Message:   `"calcualtion" is a possible misspelling of "calculation".`,
					Category:  "SpellChecker",
					StartLine: 3,
					EndLine:   3,
					StartChar: 0,
					EndChar:   11,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "calculation",
									StartLine:   3,
									EndLine:     3,
									StartChar:   0,
									EndChar:     11,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("One line comment with // separating misspelled words", t, func(t *ftt.Test) {
		fileContent := "//Doccument//divertion, docrines\ndoas\n"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"Doccument" is a possible misspelling of "Document".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 2,
					EndChar:   11,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "Document",
									StartLine:   1,
									EndLine:     1,
									StartChar:   2,
									EndChar:     11,
								},
							},
						},
					},
				},
				{
					Path:      "test.c",
					Message:   `"divertion" is a possible misspelling of "diversion".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 13,
					EndChar:   22,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "diversion",
									StartLine:   1,
									EndLine:     1,
									StartChar:   13,
									EndChar:     22,
								},
							},
						},
					},
				},
				{
					Path:      "test.c",
					Message:   `"docrines" is a possible misspelling of "doctrines".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 24,
					EndChar:   32,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "doctrines",
									StartLine:   1,
									EndLine:     1,
									StartChar:   24,
									EndChar:     32,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("All words in a text file are analyzed", t, func(t *ftt.Test) {
		fileContent := "Familes\nfaund\nnormal\n"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.txt",
					Message:   `"Familes" is a possible misspelling of "Families".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 0,
					EndChar:   7,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.txt",
									Replacement: "Families",
									StartLine:   1,
									EndLine:     1,
									StartChar:   0,
									EndChar:     7,
								},
							},
						},
					},
				},
				{
					Path:      "test.txt",
					Message:   `"faund" is a possible misspelling of "found" or "fund".`,
					Category:  "SpellChecker",
					StartLine: 2,
					EndLine:   2,
					StartChar: 0,
					EndChar:   5,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.txt",
									Replacement: "found",
									StartLine:   2,
									EndLine:     2,
									StartChar:   0,
									EndChar:     5,
								},
							},
						},
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.txt",
									Replacement: "fund",
									StartLine:   2,
									EndLine:     2,
									StartChar:   0,
									EndChar:     5,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("Analyzing HTML file generates appropriate comments", t, func(t *ftt.Test) {
		fileContent := "<!DOCTYPE html>\n<html>\n<head>\n<!--coment-->\n</head>\n</html>\n"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.html",
					Message:   `"coment" is a possible misspelling of "comment".`,
					Category:  "SpellChecker",
					StartLine: 4,
					EndLine:   4,
					StartChar: 4,
					EndChar:   10,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.html",
									Replacement: "comment",
									StartLine:   4,
									EndLine:     4,
									StartChar:   4,
									EndChar:     10,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.html", false, cp[".html"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("Same misspelling multiple times in one line", t, func(t *ftt.Test) {
		fileContent := "//twpo twpo"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"twpo" is a possible misspelling of "two".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 2,
					EndChar:   6,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "two",
									StartLine:   1,
									EndLine:     1,
									StartChar:   2,
									EndChar:     6,
								},
							},
						},
					},
				},
				{
					Path:      "test.c",
					Message:   `"twpo" is a possible misspelling of "two".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 7,
					EndChar:   11,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "two",
									StartLine:   1,
									EndLine:     1,
									StartChar:   7,
									EndChar:     11,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))
	})

	ftt.Run("Words joined in camelcase should be checked individually", t, func(t *ftt.Test) {
		fileContent := "//camelCaseExmaple comment"

		expected := &tricium.Data_Results{
			Comments: []*tricium.Data_Comment{
				{
					Path:      "test.c",
					Message:   `"Exmaple" is a possible misspelling of "Example".`,
					Category:  "SpellChecker",
					StartLine: 1,
					EndLine:   1,
					StartChar: 11,
					EndChar:   18,
					Suggestions: []*tricium.Data_Suggestion{
						{
							Description: "Misspelling fix suggestion",
							Replacements: []*tricium.Data_Replacement{
								{
									Path:        "test.c",
									Replacement: "Example",
									StartLine:   1,
									EndLine:     1,
									StartChar:   11,
									EndChar:     18,
								},
							},
						},
					},
				},
			},
		}

		results := &tricium.Data_Results{}
		analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.c", false, cp[".c"], results)
		assert.Loosely(t, results, should.Resemble(expected))

		t.Run("unless the entire word is in the ignore list", func(t *ftt.Test) {
			fileContent := "token WontFix is OK"

			results := &tricium.Data_Results{}
			analyzeFile(bufio.NewScanner(strings.NewReader(fileContent)), "test.txt", true, cp[".txt"], results)
			assert.Loosely(t, results.Comments, should.BeEmpty)
		})
	})
}

func TestGettingCommentFormat(t *testing.T) {
	cp := loadCommentFormats()

	ftt.Run("The appropriate comment formats are determined from the file extensions", t, func(t *ftt.Test) {
		assert.Loosely(t, cp[".py"], should.Resemble(&commentFormat{
			LineStart:  "#",
			BlockStart: `"""`,
			BlockEnd:   `"""`,
		}))

		assert.Loosely(t, cp[".c"], should.Resemble(&commentFormat{
			LineStart:  "//",
			BlockStart: `/*`,
			BlockEnd:   `*/`,
		}))

		assert.Loosely(t, cp[".html"], should.Resemble(&commentFormat{
			BlockStart: `<!--`,
			BlockEnd:   `-->`,
		}))
	})
}

func TestCommentCaseMatching(t *testing.T) {
	ftt.Run("matchCase converts to title-case if target appears to be title case", t, func(t *ftt.Test) {
		assert.Loosely(t, matchCase("myword", "Myword"), should.Equal("Myword"))
		assert.Loosely(t, matchCase("myword", "TarGet"), should.Equal("Myword"))
	})

	ftt.Run("matchCase doesn't convert case if the target has irregular case", t, func(t *ftt.Test) {
		assert.Loosely(t, matchCase("myword", "tArGeT"), should.Equal("myword"))
	})

	ftt.Run("words with apostrophes are capitalized as expected", t, func(t *ftt.Test) {
		assert.Loosely(t, matchCase("don't", "Target"), should.Equal("Don't"))
	})
}

func TestStripLastParagraph(t *testing.T) {
	ftt.Run("stripFooterParagraph removes last paragraph with footers", t, func(t *ftt.Test) {
		assert.Loosely(t,
			stripFooterParagraph("Summary\n\ntext\n\nReviewed-by: Personn Naime\n\n"),
			should.Equal("Summary\n\ntext"))
	})

	ftt.Run("stripFooterParagraph doesn't remove single-paragraph message", t, func(t *ftt.Test) {
		assert.Loosely(t,
			stripFooterParagraph("Summary\ntext\n"),
			should.Equal("Summary\ntext\n"))
	})

	ftt.Run("stripFooterParagraph handles non-empty 'blank lines'", t, func(t *ftt.Test) {
		assert.Loosely(t,
			stripFooterParagraph("Summary\n \nChange-Id: I12342\nBug: 1234\n"),
			should.Equal("Summary"))
	})
}
