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

	"infra/tricium/api/v1"
)

func TestPylintParsingFunctions(t *testing.T) {

	ftt.Run("scanPylintOutput", t, func(t *ftt.Test) {

		t.Run("Parsing empty buffer gives no warnings", func(t *ftt.Test) {
			buf := strings.NewReader("")
			s := bufio.NewScanner(buf)
			assert.Loosely(t, s, should.NotBeNil)

			results := &tricium.Data_Results{}
			scanCpplintOutput(s, results)
			assert.Loosely(t, results.Comments, should.BeEmpty)
		})

		t.Run("Parsing normal cpplint output generates the appropriate comments", func(t *ftt.Test) {
			output := "test.cc:0:  No copyright message found  [legal/copyright] [5]\n" +
				"test.cc:141:  If an else has a brace on one side, it should have it on both  [readability/braces] [5]\n" +
				"test.cc:42:  Add #include <vector> for vector<>  [build/include_what_you_use] [4]\n"

			expected := &tricium.Data_Results{
				Comments: []*tricium.Data_Comment{
					{
						Category: "Cpplint/legal/copyright",
						Message: "No copyright message found (confidence 5/5).\n" +
							"To disable, add: // NOLINT(legal/copyright)",
						Path:      "test.cc",
						StartLine: 0,
					},
					{
						Category: "Cpplint/readability/braces",
						Message: "If an else has a brace on one side, " +
							"it should have it on both (confidence 5/5).\n" +
							"To disable, add: // NOLINT(readability/braces)",
						Path:      "test.cc",
						StartLine: 141,
					},
					{
						Category: "Cpplint/build/include_what_you_use",
						Message: "Add #include <vector> for vector<>\n" +
							"Note: This check is known to produce false positives, " +
							"(e.g. for types used only in function overrides). (confidence 4/5).\n" +
							"To disable, add: // NOLINT(build/include_what_you_use)",
						Path:      "test.cc",
						StartLine: 42,
					},
				},
			}

			results := &tricium.Data_Results{}
			scanCpplintOutput(bufio.NewScanner(strings.NewReader(output)), results)
			assert.Loosely(t, results, should.Match(expected))
		})
	})

	ftt.Run("parsePylintLine", t, func(t *ftt.Test) {

		t.Run("Parsing valid line gives a comment", func(t *ftt.Test) {
			line := "test.cc:148:  This is the helpful explanation  [readability/foo] [4]"
			assert.Loosely(t, parseCpplintLine(line), should.Match(&tricium.Data_Comment{
				Category: "Cpplint/readability/foo",
				Message: "This is the helpful explanation (confidence 4/5).\n" +
					"To disable, add: // NOLINT(readability/foo)",
				Path:      "test.cc",
				StartLine: 148,
			}))
		})

		t.Run("Parsing some other line gives nil", func(t *ftt.Test) {
			assert.Loosely(t, parseCpplintLine("Total errors found: 24"), should.BeNil)
		})

		t.Run("Warnings for include_what_you_use for string are ignored", func(t *ftt.Test) {
			line := "test.cc:148:  Add #include <string> for string  [build/include_what_you_use] [4]"
			assert.Loosely(t, parseCpplintLine(line), should.BeNil)
		})

		t.Run("An extra note is added for include_what_you_use", func(t *ftt.Test) {
			line := "test.cc:148:  Add #include <foo> for foo  [build/include_what_you_use] [4]"
			assert.Loosely(t, parseCpplintLine(line), should.Match(&tricium.Data_Comment{
				Category: "Cpplint/build/include_what_you_use",
				Message: ("Add #include <foo> for foo\n" +
					"Note: This check is known to produce false positives, " +
					"(e.g. for types used only in function overrides). (confidence 4/5).\n" +
					"To disable, add: // NOLINT(build/include_what_you_use)"),
				Path:      "test.cc",
				StartLine: 148,
			}))
		})
	})

	ftt.Run("Command line argument helper functions", t, func(t *ftt.Test) {
		t.Run("Building the verbose argument", func(t *ftt.Test) {
			assert.Loosely(t, verboseArg("5"), should.Equal("5"))
			assert.Loosely(t, verboseArg(""), should.Equal("4"))
		})

		t.Run("Building the filter argument", func(t *ftt.Test) {
			assert.Loosely(t, filterArg("-whitespace/braces"), should.Equal("-whitespace/braces"))
			assert.Loosely(t, filterArg(""), should.Equal("-whitespace"))
		})
	})
}
