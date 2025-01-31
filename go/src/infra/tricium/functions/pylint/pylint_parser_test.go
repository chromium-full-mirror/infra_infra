// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

func TestPylintParsing(t *testing.T) {

	ftt.Run("parsePylintOutput", t, func(t *ftt.Test) {

		t.Run("Parsing empty buffer gives no warnings", func(t *ftt.Test) {
			comments, err := parsePylintOutput([]byte("[]"))
			if err != nil {
				t.Fatal(err)
			}
			assert.Loosely(t, comments, should.BeEmpty)
		})

		t.Run("Parsing normal pylint output generates the appropriate comments", func(t *ftt.Test) {
			output := `
				[
					{
						"type": "convention",
						"line": 6,
						"column": 0,
						"path": "test.py",
						"symbol": "empty-docstring",
						"message": "Empty function docstring"
					},
					{
						"type": "warning",
						"line": 6,
						"column": 15,
						"path": "test.py",
						"symbol": "unused-argument",
						"message": "Unused argument 'y'"
					},
					{
						"type": "warning",
						"line": 6,
						"column": 18,
						"path": "test.py",
						"symbol": "unused-argument",
						"message": "Unused argument 'z'"
					},
					{
						"type": "warning",
						"line": 12,
						"column": 2,
						"path": "test.py",
						"symbol": "unnecessary-pass",
						"message": "Unnecessary pass statement"
					},
					{
						"type": "warning",
						"line": 19,
						"column": 10,
						"path": "test.py",
						"symbol": "undefined-loop-variable",
						"message": "Using possibly undefined loop variable 'a'"
					},
					{
						"type": "warning",
						"line": 18,
						"column": 6,
						"path": "test.py",
						"symbol": "unused-variable",
						"message": "Unused variable 'a'"
					},
					{
						"type": "error",
						"line": 26,
						"column": 0,
						"path": "test.py",
						"symbol": "undefined-variable",
						"message": "Undefined variable 'main'"
					}
				]
			`

			expected := []*tricium.Data_Comment{
				{
					Path: "test.py",
					Message: "Type: convention; Symbol: empty-docstring\n" +
						"Empty function docstring.\n" +
						"To disable, add: # pylint: disable=empty-docstring",
					Category:  "pylint",
					StartLine: 6,
					StartChar: 0,
				},
				{
					Path: "test.py",
					Message: "Type: warning; Symbol: unused-argument\n" +
						"Unused argument 'y'.\n" +
						"To disable, add: # pylint: disable=unused-argument",
					Category:  "pylint",
					StartLine: 6,
					StartChar: 15,
				},
				{
					Path: "test.py",
					Message: "Type: warning; Symbol: unused-argument\n" +
						"Unused argument 'z'.\n" +
						"To disable, add: # pylint: disable=unused-argument",
					Category:  "pylint",
					StartLine: 6,
					StartChar: 18,
				},
				{
					Path: "test.py",
					Message: "Type: warning; Symbol: unnecessary-pass\n" +
						"Unnecessary pass statement.\n" +
						"To disable, add: # pylint: disable=unnecessary-pass",
					Category:  "pylint",
					StartLine: 12,
					StartChar: 2,
				},
				{
					Path: "test.py",
					Message: "Type: warning; Symbol: undefined-loop-variable\n" +
						"Using possibly undefined loop variable 'a'.\n" +
						"To disable, add: # pylint: disable=undefined-loop-variable",
					Category:  "pylint",
					StartLine: 19,
					StartChar: 10,
				},
				{
					Path: "test.py",
					Message: "Type: warning; Symbol: unused-variable\n" +
						"Unused variable 'a'.\n" +
						"To disable, add: # pylint: disable=unused-variable",
					Category:  "pylint",
					StartLine: 18,
					StartChar: 6,
				},
				{
					Path: "test.py",
					Message: "Type: error; Symbol: undefined-variable\n" +
						"Undefined variable 'main'.\n" +
						"This check could give false positives when there are wildcard imports\n" +
						"(from module import *). It is recommended to avoid wildcard imports; see\n" +
						"https://www.python.org/dev/peps/pep-0008/#imports.\n" +
						"To disable, add: # pylint: disable=undefined-variable",
					Category:  "pylint",
					StartLine: 26,
					StartChar: 0,
				},
			}

			comments, err := parsePylintOutput([]byte(output))
			if err != nil {
				t.Fatal(err)
			}
			assert.That(t, comments, should.Match(expected))
		})
	})

	ftt.Run("isFatalPylintError", t, func(t *ftt.Test) {
		t.Run("Returns false for nil error", func(t *ftt.Test) {
			assert.That(t, isFatalPylintError(nil), should.BeFalse)
		})

		t.Run("Returns true for non-exit error", func(t *ftt.Test) {
			assert.That(t, isFatalPylintError(os.ErrNotExist), should.BeTrue)
		})

		t.Run("Returns false for non-fatal exit errors", func(t *ftt.Test) {
			assert.That(t, isFatalPylintError(fakeExitError{2}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{4}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{6}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{8}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{12}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{14}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{16}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{18}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{20}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{22}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{24}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{26}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{28}), should.BeFalse)
			assert.That(t, isFatalPylintError(fakeExitError{30}), should.BeFalse)
		})

		t.Run("Returns true for fatal exit errors", func(t *ftt.Test) {
			assert.That(t, isFatalPylintError(fakeExitError{1}), should.BeTrue)
			assert.That(t, isFatalPylintError(fakeExitError{19}), should.BeTrue)
			assert.That(t, isFatalPylintError(fakeExitError{32}), should.BeTrue)
			assert.That(t, isFatalPylintError(fakeExitError{123}), should.BeTrue)
		})
	})
}

type fakeExitError struct {
	exitCode int
}

func (e fakeExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.exitCode)
}

func (e fakeExitError) ExitCode() int {
	return e.exitCode
}
