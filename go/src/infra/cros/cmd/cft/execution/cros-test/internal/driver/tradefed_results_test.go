// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/localonly"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

var (
	TradefedTestResultFile              = filepath.Join("tradefed_test_data/test_result.xml")
	TradefedTestResultFileModulesNoTest = filepath.Join("tradefed_test_data/test_result_modules_no_test.xml")
	TradefedTestResultModuleError       = filepath.Join("tradefed_test_data/test_result_module_error.xml")
	TradefedLuciTestResultFile          = filepath.Join("tradefed_test_data/LUCIResult_.json")

	FileSelectFileByPattern1 = filepath.Join("/tmp/test-SelectFileByPattern-file1.txt")
	FileSelectFileByPattern2 = filepath.Join("/tmp/test-SelectFileByPattern-file2.txt")
	FileSelectFileByPattern3 = filepath.Join("/tmp/test-SelectFileByPattern-file3.txt")
	FileSelectGlobPattern    = filepath.Join("/tmp/test-Se*-file*.txt")
	FileSelectBadPattern     = filepath.Join("/tmp/mdfvblsakerbakl*")
)

type filePathAndTime struct {
	path    string
	modTime time.Time
}

func createFileWithModTime(t *testing.T, filePath string, mod time.Time) {
	if _, err := os.Create(filePath); err != nil {
		t.Errorf("Can't create temp test file: %s, error: %v", filePath, err)
		return
	}
	if err := os.Chtimes(filePath, mod, mod); err != nil {
		t.Errorf("Can't modify temp test file: %s, error: %v", filePath, err)
	}
}

func TestSelectFileByPattern(t *testing.T) {
	localonly.Because(t, "b/397939428")

	baseTime := time.Now()

	ftt.Run("Select file by pattern", t, func(t *ftt.Test) {
		for _, tc := range []struct {
			name     string
			files    []filePathAndTime
			pattern  string
			selected string
		}{
			{
				name: "Single file by full name",
				files: []filePathAndTime{
					{path: FileSelectFileByPattern1, modTime: baseTime},
				},
				pattern:  FileSelectFileByPattern1,
				selected: FileSelectFileByPattern1,
			},
			{
				name: "Multiple files by pattern",
				files: []filePathAndTime{
					{path: FileSelectFileByPattern1, modTime: baseTime.Add(-time.Minute * 30)},
					{path: FileSelectFileByPattern2, modTime: baseTime},
					{path: FileSelectFileByPattern3, modTime: baseTime.Add(-time.Minute * 10)},
				},
				pattern:  FileSelectGlobPattern,
				selected: FileSelectFileByPattern2,
			},
			{
				name: "No matching files by pattern",
				files: []filePathAndTime{
					{path: FileSelectFileByPattern1, modTime: baseTime},
				},
				pattern:  FileSelectBadPattern,
				selected: "",
			},
		} {
			ftt.Run(fmt.Sprintf("%s, pattern: %s", tc.name, tc.pattern), t.T, func(t *ftt.Test) {
				for _, file := range tc.files {
					createFileWithModTime(t.T, file.path, file.modTime)
					defer os.Remove(file.path)
				}
				got, err := selectFileByPattern(tc.pattern)

				if len(tc.selected) > 0 {
					assert.Loosely(t, err, should.BeNil)
				} else {
					assert.Loosely(t, err, should.NotBeNil)
				}
				assert.Loosely(t, got, should.Equal(tc.selected))
			})
		}
	})
}

func getTestCaseResults(prefix string) []*api.TestCaseResult {
	failureMessage := "java.lang.AssertionError\r\njava.lang.AssertionError\n\tat org.junit.Assert.fail(Assert.java:86)\n"
	startTimestamp := int64(1724450705582)
	startTime := timestamppb.New(time.Unix(startTimestamp/1000, startTimestamp%1000*1000000))
	numOfTests := 2
	duration := &durationpb.Duration{Seconds: int64(2254 / 1000 / numOfTests)}
	testTags := []*api.TestCase_Tag{{Value: "abi:x86_64"}}
	testHarness := &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{Tradefed: &api.TestHarness_Tradefed{}}}
	testClass := "android.jvmti.cts.JvmtiHostTest1976"
	testFullName := prefix + testClass

	return []*api.TestCaseResult{
		{
			TestCaseId:  &api.TestCase_Id{Value: prefix + testClass + "#testJvmti"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testJvmti"}, Name: extractTestCaseName(testFullName + "#testJvmti")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: prefix + testClass + "#testAssumptionFail"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			Errors:      []*api.TestCaseResult_Error{{Message: failureMessage}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testAssumptionFail"}, Name: extractTestCaseName(testFullName + "#testAssumptionFail")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "#testIncomplate"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Crash_{Crash: &api.TestCaseResult_Crash{}},
			Errors:      []*api.TestCaseResult_Error{{Message: failureMessage}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testIncomplate"}, Name: extractTestCaseName(testFullName + "#testIncomplate")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "#testIgnored"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}},
			Errors:      []*api.TestCaseResult_Error{{Message: ""}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testIgnored"}, Name: extractTestCaseName(testFullName + "#testIgnored")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "#testUnknown"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
			Errors:      []*api.TestCaseResult_Error{{Message: ""}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testUnknown"}, Name: extractTestCaseName(testFullName + "#testUnknown")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: prefix + "android.mediav2.cts.CodecDecoderSurfaceTest#testFlushNative"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
			Errors:      []*api.TestCaseResult_Error{{Message: failureMessage}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: prefix + "android.mediav2.cts.CodecDecoderSurfaceTest#testFlushNative"}, Name: extractTestCaseName(prefix + "android.mediav2.cts.CodecDecoderSurfaceTest#testFlushNative")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
	}
}

func getGeneralTestCaseResults(prefix string) []*api.TestCaseResult {
	startTimestamp := int64(1736207832737)
	startTime := timestamppb.New(time.Unix(startTimestamp/1000, startTimestamp%1000*1000000))
	numOfTests := 4
	duration := &durationpb.Duration{Seconds: int64(18 / numOfTests)}
	testTags := []*api.TestCase_Tag{{Value: "abi:x86_64"}}
	testHarness := &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{Tradefed: &api.TestHarness_Tradefed{}}}
	testClass := "com.android.chrome.desktop.integration.ChromeWindowTests"
	testFullName := prefix + testClass

	return []*api.TestCaseResult{
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "#testSingleWindow"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testSingleWindow"}, Name: extractTestCaseName(testFullName + "#testSingleWindow")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "#testSingleWindow"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testSingleWindow"}, Name: extractTestCaseName(testFullName + "#testSingleWindow")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "#testMultiWindowsLimitFail"},
			Tags:        testTags,
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
			Errors:      []*api.TestCaseResult_Error{{Message: "null: java.lang.AssertionError: windows expected: 5, found: 4"}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "#testMultiWindowsLimitFail"}, Name: extractTestCaseName(testFullName + "#testMultiWindowsLimitFail")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
		{
			TestCaseId:  &api.TestCase_Id{Value: testFullName + "2#testMultiWindowsLimit2"},
			Tags:        []*api.TestCase_Tag{{Value: "abi:arm64-v8a"}}, // Different test ABI that should override module ABI.
			TestHarness: testHarness,
			Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			StartTime:   startTime,
			Duration:    duration,
			TestCaseMetadata: &api.TestCaseMetadata{
				TestCase:     &api.TestCase{Id: &api.TestCase_Id{Value: testFullName + "2#testMultiWindowsLimit2"}, Name: extractTestCaseName(testFullName + "2#testMultiWindowsLimit2")},
				TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
			},
		},
	}
}

func TestParseCompatibilityXmlResults(t *testing.T) {
	t.Parallel()

	ftt.Run("Build TradeFed results from XML", t, func(t *ftt.Test) {
		logger := log.New(io.Discard, "", 0)
		testType := "cts"
		want := &api.CrosTestResponse{
			TestCaseResults: getTestCaseResults("tradefed.cts.CtsJvmtiRunTest1976HostTestCases#"),
			GivenTestResults: []*api.CrosTestResponse_GivenTestResult{
				{
					ParentTest:           "tradefed.cts.CtsJvmtiRunTest1976HostTestCases",
					ChildTestCaseResults: getTestCaseResults(""),
				},
			},
		}

		R := Result{}
		err := parseResultFile(logger, TradefedTestResultFile, &R)
		assert.Loosely(t, err, should.BeNil)

		allTestCases, givenTestCases := generateTestCaseResult(logger, testType, R, nil)

		assert.Loosely(t, allTestCases, should.Match(want.TestCaseResults))
		assert.Loosely(t, givenTestCases, should.Match(want.GivenTestResults))
	})

	ftt.Run("Build TradeFed results from empty XML", t, func(t *ftt.Test) {
		logger := log.New(io.Discard, "", 0)
		R := Result{}
		testType := "cts"
		moduleName := fmt.Sprintf("tradefed.%s.CtsJvmtiRunTest1976HostTestCases", testType)
		passVerdict := &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}

		err := parseResultFile(logger, TradefedTestResultFileModulesNoTest, &R)
		assert.Loosely(t, err, should.BeNil)

		allTestCases, givenTestCases := generateTestCaseResult(logger, testType, R, nil)

		assert.Loosely(t, allTestCases[0].TestCaseId.Value, should.Equal(moduleName))
		assert.Loosely(t, allTestCases[0].Verdict, should.Match(passVerdict))
		assert.Loosely(t, givenTestCases[0].GetParentTest(), should.Equal(moduleName))
		assert.Loosely(t, givenTestCases[0].ChildTestCaseResults[0].Verdict, should.Match(passVerdict))
	})

	ftt.Run("Build TradeFed results from XML with module error", t, func(t *ftt.Test) {
		logger := log.New(io.Discard, "", 0)
		R := Result{}
		testType := "cts"
		moduleName := fmt.Sprintf("tradefed.%s.CtsWrapWrapNoDebugTestCases", testType)
		failVerdict := &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}

		err := parseResultFile(logger, TradefedTestResultModuleError, &R)
		assert.Loosely(t, err, should.BeNil)

		allTestCases, givenTestCases := generateTestCaseResult(logger, testType, R, nil)

		assert.Loosely(t, allTestCases[0].TestCaseId.Value, should.Equal(moduleName))
		assert.Loosely(t, allTestCases[0].Verdict, should.Match(failVerdict))
		assert.Loosely(t, givenTestCases[0].GetParentTest(), should.Equal(moduleName))
		assert.Loosely(t, givenTestCases[0].ChildTestCaseResults[0].Verdict, should.Match(failVerdict))
	})

	ftt.Run("Build TradeFed results from LUCI JSON", t, func(t *ftt.Test) {
		logger := log.New(io.Discard, "", 0)
		moduleName := "tradefed.general.DesktopChromeTestCases"

		want := &api.CrosTestResponse{
			TestCaseResults: getGeneralTestCaseResults(moduleName + "#"),
			GivenTestResults: []*api.CrosTestResponse_GivenTestResult{
				{
					ParentTest:           moduleName,
					ChildTestCaseResults: getGeneralTestCaseResults(""),
				},
			},
		}

		TR := LuciJSONResult{}
		err := parseResultFile(logger, TradefedLuciTestResultFile, &TR)
		assert.Loosely(t, err, should.BeNil)

		R, err2 := convertToResult(logger, TR)
		assert.Loosely(t, err2, should.BeNil)

		allTestCases, givenTestCases := generateTestCaseResult(logger, "general", R, nil)

		assert.Loosely(t, allTestCases, should.Match(want.TestCaseResults))
		assert.Loosely(t, givenTestCases, should.Match(want.GivenTestResults))
	})

}

func TestBuildTcResult(t *testing.T) {
	t.Parallel()

	ftt.Run("Build test results for different statuses", t, func(t *ftt.Test) {
		startTime, _ := time.Parse(time.RFC3339, "2024-01-18T00:12:34Z")
		endTime, _ := time.Parse(time.RFC3339, "2024-01-18T00:13:34Z")
		duration := endTime.Sub(startTime)
		failureMessage := "Failed for some reason"
		testName := "tradefed.cts.CtsSampleTestCase"
		abi := "x86_64"

		testCaseID := &api.TestCase_Id{Value: testName}
		testTags := []*api.TestCase_Tag{{Value: fmt.Sprintf("abi:%s", abi)}}
		testHarness := &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{Tradefed: &api.TestHarness_Tradefed{}}}
		wantStartTime := timestamppb.New(startTime)
		wantDuration := &durationpb.Duration{Seconds: int64(duration.Seconds())}
		wantTestCaseMetadata := &api.TestCaseMetadata{
			TestCase:     &api.TestCase{Id: testCaseID, Name: strings.TrimPrefix(testName, "tradefed.")},
			TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
		}

		for _, tc := range []struct {
			name         string
			abi          string
			errorMessage string
			status       string
			want         *api.TestCaseResult
		}{
			{
				name:         "Pass test",
				abi:          abi,
				errorMessage: "",
				status:       "PASSED",
				want: &api.TestCaseResult{
					TestCaseId:       testCaseID,
					Tags:             testTags,
					TestHarness:      testHarness,
					Verdict:          &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
					StartTime:        wantStartTime,
					Duration:         wantDuration,
					TestCaseMetadata: wantTestCaseMetadata,
				},
			},
			{
				name:         "Assumption failure test",
				abi:          abi,
				errorMessage: failureMessage,
				status:       "ASSUMPTION_FAILURE",
				want: &api.TestCaseResult{
					TestCaseId:       testCaseID,
					Tags:             testTags,
					TestHarness:      testHarness,
					Verdict:          &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
					Errors:           []*api.TestCaseResult_Error{{Message: failureMessage}},
					StartTime:        wantStartTime,
					Duration:         wantDuration,
					TestCaseMetadata: wantTestCaseMetadata,
				},
			},
			{
				name:         "Fail test",
				abi:          abi,
				errorMessage: failureMessage,
				status:       "FAILED",
				want: &api.TestCaseResult{
					TestCaseId:       testCaseID,
					Tags:             testTags,
					TestHarness:      testHarness,
					Verdict:          &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
					Errors:           []*api.TestCaseResult_Error{{Message: failureMessage}},
					StartTime:        wantStartTime,
					Duration:         wantDuration,
					TestCaseMetadata: wantTestCaseMetadata,
				},
			},
			{
				name:         "Incomplete test",
				abi:          abi,
				errorMessage: failureMessage,
				status:       "INCOMPLETE",
				want: &api.TestCaseResult{
					TestCaseId:       testCaseID,
					Tags:             testTags,
					TestHarness:      testHarness,
					Verdict:          &api.TestCaseResult_Crash_{Crash: &api.TestCaseResult_Crash{}},
					Errors:           []*api.TestCaseResult_Error{{Message: failureMessage}},
					StartTime:        wantStartTime,
					Duration:         wantDuration,
					TestCaseMetadata: wantTestCaseMetadata,
				},
			},
			{
				name:         "Skipped test",
				abi:          abi,
				errorMessage: failureMessage,
				status:       "SKIPPED",
				want: &api.TestCaseResult{
					TestCaseId:       testCaseID,
					Tags:             testTags,
					TestHarness:      testHarness,
					Verdict:          &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}},
					Errors:           []*api.TestCaseResult_Error{{Message: failureMessage}},
					StartTime:        wantStartTime,
					Duration:         wantDuration,
					TestCaseMetadata: wantTestCaseMetadata,
				},
			},
			{
				name:         "Ignore test",
				abi:          abi,
				errorMessage: failureMessage,
				status:       "IGNORED",
				want: &api.TestCaseResult{
					TestCaseId:       testCaseID,
					Tags:             testTags,
					TestHarness:      testHarness,
					Verdict:          &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}},
					Errors:           []*api.TestCaseResult_Error{{Message: failureMessage}},
					StartTime:        wantStartTime,
					Duration:         wantDuration,
					TestCaseMetadata: wantTestCaseMetadata,
				},
			},
		} {
			ftt.Run(tc.name, t.T, func(t *ftt.Test) {
				got := buildTcResult(testName, tc.abi, tc.status, startTime, int64(duration.Seconds()), tc.errorMessage)
				assert.Loosely(t, got, should.Match(tc.want))
			})
		}
	})
}
