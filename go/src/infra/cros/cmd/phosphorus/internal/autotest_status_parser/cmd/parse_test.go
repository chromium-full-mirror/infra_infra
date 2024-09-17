// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package cmd

import (
	"testing"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const verdictPass = skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS
const verdictFail = skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL
const verdictError = skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR
const verdictAbort = skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT
const verdictNoVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT

// parseResultsFile tests

func TestEmptyResultFile(t *testing.T) {
	ftt.Run("When an empty status.log is parsed, there are no results.", t, func(t *ftt.Test) {
		input := ""
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase(nil)
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestPass(t *testing.T) {
	ftt.Run("When status.log contains a GOOD status with a test name, a passing test is returned.", t, func(t *ftt.Test) {
		// Input format: each line consists of strings separated by tab characters,
		// the first three strings in a line represent the test status, test
		// directory and test name, the last string when present represents a
		// comment (see README.md for a detailed description).
		input := `
START	----	Pass
	GOOD	----	----
END GOOD	----	----
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Pass", verdictPass, ""),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestWarn(t *testing.T) {
	ftt.Run("When status.log contains a WARN status, a passing test with a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	----	Warn
	WARN	----	----	This is a warning.
END WARN	----	----
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Warn", verdictPass, "This is a warning.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestFail(t *testing.T) {
	ftt.Run("When status.log contains a FAIL status, a test failure with a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	----	Fail
	FAIL	----	----	Something failed.
END FAIL	----	----
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Fail", verdictFail, "Something failed.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestFailWithStackTrace(t *testing.T) {
	ftt.Run("When status.log contains a FAIL status with a stack trace, a test failure with a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	Fail.suspend	Fail.suspend
	FAIL	Fail.suspend	Fail.suspend	ValueError: max() arg is an empty sequence
  Traceback (most recent call last):
    File "/usr/local/autotest/client/common_lib/test.py", line 816, in _call_test_function
      return func(*args, **dargs)
  ValueError: max() arg is an empty sequence
END FAIL	Fail.suspend	Fail.suspend
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Fail.suspend", verdictFail, "ValueError: max() arg is an empty sequence\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestError(t *testing.T) {
	ftt.Run("When status.log contains an ERROR status, a test failure with a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	----	Error
	ERROR	----	----	An error occured.
END ERROR	----	----
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Error", verdictError, "An error occured.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestTestNA(t *testing.T) {
	ftt.Run("When status.log contains an TEST_NA status, a test failure with a comment is returned (crbug.com/846770).", t, func(t *ftt.Test) {
		input := `
START	----	TestNA
	TEST_NA	----	----	The DUT is missing a necessary gadget.
END TEST_NA	----	----
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("TestNA", verdictNoVerdict, "The DUT is missing a necessary gadget.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestUnrecognizedEventType(t *testing.T) {
	ftt.Run("When status.log contains an unrecognized event, a test failure without a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	----	WeirdStatus
	WEIRD	----	----	Comments about unrecognized status values are ignored.
END WEIRD	----	----
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("WeirdStatus", verdictFail, ""),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestNameless(t *testing.T) {
	ftt.Run("When a test has no name the directory name is used instead.", t, func(t *ftt.Test) {
		input := `
Fall back to dir name for nameless tests.
START	NamelessDir	----
	GOOD	Ignored	Ignored
END GOOD	Ignored	Ignored
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("NamelessDir", verdictPass, ""),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestComments(t *testing.T) {
	ftt.Run("When there are comments, only the ones from specific event lines are surfaced.", t, func(t *ftt.Test) {
		input := `
Comments that don't match the event line format (at least 3 strings separated by tabs) are ignored.
START	----	WithoutComments
	GOOD	----	----	Comments about "GOOD" tests are ignored.
END GOOD	----	----
START	----	WithComments	START comments are ignored.
	INFO	----	----	INFO comments are ignored.
	WARN	----	----	This comment is ignored because there's a string after it.	This is a warning.
Comments can be anywhere.
	WARN	----	----	There's more to say about the warning.
	WARN	----	----	This comment is ignored because there's a tab after it.	
END WARN	----	----	END comments are ignored.
	stderr:
More logging that is ignored.
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("WithoutComments", verdictPass, ""),
			testCase("WithComments", verdictPass, "This is a warning.\nThere's more to say about the warning.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestIgnoredFields(t *testing.T) {
	ftt.Run("When a test name is provided in the START event, directory name is ignored and test name provided in other events is also ignored.", t, func(t *ftt.Test) {
		input := `
START	Ignored	Test-Name
	INFO	Ignored	Ignored	Ignored.
	FAIL	Ignored	Ignored	Not ignored.
END FAIL	Ignored	Ignored

`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Test-Name", verdictFail, "Not ignored.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestIgnoredTestNames(t *testing.T) {
	ftt.Run("When a test name is 'reboot', 'suspend' or both test name and directory name are missing, ignore the test.", t, func(t *ftt.Test) {
		input := `
START	----	----
	FAIL	Ignored	Ignored	Something failed.
END FAIL	Ignored	Ignored

Ignore tests named reboot.
START	----	reboot
	FAIL	Ignored	Ignored	Something failed.
END FAIL	Ignored	Ignored

Ignore tests named suspend.
START	----	suspend
	FAIL	Ignored	Ignored	Something failed.
END FAIL	Ignored	Ignored
`
		got := parseResultsFile(input)

		assert.Loosely(t, got, should.BeNil)
	})
}

func TestNestedTestCases(t *testing.T) {
	ftt.Run("When the test cases are nested, the verdicts are correctly attributed to test cases.",
		t, func(t *ftt.Test) {
			input := `
START	----	NestedTest
	START	----	SubTest1
		WARN	----	----	A subtest warning.
	END WARN	----	----
	START	----	SubTest2
		START	----	SubSubTest
			ERROR	----	----	The outer tests don't care about this error.
		END ERROR	----	----
		GOOD	----	----	Subtest succeeded.
	END GOOD	----	----
	FAIL	----	----	A failure of the outer test.
END FAIL	----	----
`
			got := parseResultsFile(input)

			want := []*skylab_test_runner.Result_Autotest_TestCase{
				testCase("SubTest1", verdictPass, "A subtest warning.\n"),
				testCase("SubSubTest", verdictError, "The outer tests don't care about this error.\n"),
				testCase("SubTest2", verdictPass, ""),
				testCase("NestedTest", verdictFail, "A failure of the outer test.\n"),
			}
			assert.Loosely(t, got, should.Resemble(want))
		})
}

func TestUnfinishedTestCases(t *testing.T) {
	ftt.Run("When test cases don't have an 'END ...' event, declare them aborted.",
		t, func(t *ftt.Test) {
			input := `
START	----	CrashedOuter
	START	----	CrashedMiddle
		START	----	CrashedInner
			WARN	----	----	Something bad is happening.
	AutoservRunError: command execution error
  stderr:
  A stack trace that is ignored.`
			got := parseResultsFile(input)

			want := []*skylab_test_runner.Result_Autotest_TestCase{
				testCase("CrashedInner", verdictAbort, "Something bad is happening.\n"),
				testCase("CrashedMiddle", verdictAbort, ""),
				testCase("CrashedOuter", verdictAbort, ""),
			}
			assert.Loosely(t, got, should.Resemble(want))
		})
}

func TestContradictoryVerdicts(t *testing.T) {
	ftt.Run("When an event string disagrees with the 'END ' string, the ends string wins.",
		t, func(t *ftt.Test) {
			input := `
START	----	ActuallyPasses
	FAIL	----	----	This test actually succeeds.
END GOOD	----	----

START	----	ActuallyFails
	GOOD	----	----	Nothing could go wrong.
END ERROR	----	----
`
			got := parseResultsFile(input)

			want := []*skylab_test_runner.Result_Autotest_TestCase{
				testCase("ActuallyPasses", verdictPass, "This test actually succeeds.\n"),
				testCase("ActuallyFails", verdictError, ""),
			}
			assert.Loosely(t, got, should.Resemble(want))
		})
}

func TestInterruptedTestCase(t *testing.T) {
	ftt.Run("When status.log contains an interrupted test case without an END event line, the test is still returned but declare it aborted.", t, func(t *ftt.Test) {
		input := `
START	----	----
	START	----	Fail
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("Fail", verdictAbort, ""),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

// Exit code file tests

func TestExitedWithoutErrors(t *testing.T) {
	ftt.Run("When exit code is zero, report no exit errors",
		t, func(t *ftt.Test) {
			// Input format: three lines, each containing an integer. The integer on
			// the second line is the exit status code.
			input := "42\n0\n0"

			assert.Loosely(t, exitedWithErrors(input), should.Resemble(false))
		})
}

func TestExitedWithErrors(t *testing.T) {
	ftt.Run("When the server job was aborted, report exit error",
		t, func(t *ftt.Test) {
			// 256 = Exited with status 1.
			input := "42\n256\n0"

			assert.Loosely(t, exitedWithErrors(input), should.Resemble(true))
		})
}

func TestAborted(t *testing.T) {
	ftt.Run("When status.log contains an ABORT status, a test failure with a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	----	ActuallyAbort
	START	----	reboot
		GOOD	----	reboot.start
		ABORT	----	reboot.verify	Host did not return from reboot
	END FAIL	----	reboot	Host did not return from reboot
END FAIL	----	ActuallyAbort
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("ActuallyAbort", verdictAbort, "Host did not return from reboot\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestInnerAbortedStep(t *testing.T) {
	ftt.Run("When status.log contains an inner ABORT step, a test failure with a comment is returned.", t, func(t *ftt.Test) {
		input := `
START	cheets_CTS_P.internal.arm.CtsTheme	cheets_CTS_P.internal.arm.CtsTheme	timestamp=1655978455	localtime=Jun 23 03:00:55
	START	----	reboot	timestamp=1655978465	localtime=Jun 23 03:01:05
		GOOD	----	reboot.start	timestamp=1655978465	localtime=Jun 23 03:01:05
		GOOD	----	reboot.verify	timestamp=1655978495	localtime=Jun 23 03:01:35
	END GOOD	----	reboot	kernel=5.10.123-16746-g919ef5ff27e4	timestamp=1655978495	localtime=Jun 23 03:01:35
	END ABORT	----	----	timestamp=1655979120	localtime=Jun 23 03:12:00	Autotest client terminated unexpectedly: DUT is pingable, SSHable and did NOT restart un-expectedly. We probably lost connectivity during the test.
	ERROR	cheets_CTS_P.internal.arm.CtsTheme	cheets_CTS_P.internal.arm.CtsTheme	timestamp=1655979124	localtime=Jun 23 03:12:04	Failed to login to Chrome
END ERROR	cheets_CTS_P.internal.arm.CtsTheme	cheets_CTS_P.internal.arm.CtsTheme	timestamp=1655979124	localtime=Jun 23 03:12:04
`
		got := parseResultsFile(input)

		want := []*skylab_test_runner.Result_Autotest_TestCase{
			testCase("cheets_CTS_P.internal.arm.CtsTheme", verdictAbort, "Autotest client terminated unexpectedly: DUT is pingable, SSHable and did NOT restart un-expectedly. We probably lost connectivity during the test.\n"),
		}
		assert.Loosely(t, got, should.Resemble(want))
	})
}

func TestFailedToParseExitCode(t *testing.T) {
	ftt.Run("When the exit code is not an integer, report exit error.",
		t, func(t *ftt.Test) {
			input := "42\nnot_an_integer\n0"

			assert.Loosely(t, exitedWithErrors(input), should.Resemble(true))
		})
}

func TestMissingExitCode(t *testing.T) {
	ftt.Run("When the exit code is missing, report exit error.",
		t, func(t *ftt.Test) {
			input := "42"

			assert.Loosely(t, exitedWithErrors(input), should.Resemble(true))
		})
}

func TestEmptyExitStatusFile(t *testing.T) {
	ftt.Run("When the exit status file is empty, report exit error.",
		t, func(t *ftt.Test) {
			input := ""

			assert.Loosely(t, exitedWithErrors(input), should.Resemble(true))
		})
}

func testCase(name string, verdict skylab_test_runner.Result_Autotest_TestCase_Verdict, summary string) *skylab_test_runner.Result_Autotest_TestCase {
	output := skylab_test_runner.Result_Autotest_TestCase{
		Name:                 name,
		Verdict:              verdict,
		HumanReadableSummary: summary,
	}
	return &output
}
