// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

const (
	tfStageLogsPath      = "/tmp/stage-android-build-api/stub"
	tfStageResultPattern = "subprocess-test_result.xml_*.xml.gz"
	tfStageLogPattern    = "passed_tests_*.txt"
	tfAospResultPath     = "/tmp/xts_results/latest/test_result.xml"
	tfLuciResultPattern  = "LUCIResult_*.json"
	incompleteError      = "Module is missing from results or skipped by exclude filter"

	RawStatusKeyName = "raw_status"
)

//--------------------------------------------------------------
// Data structures for parsing compatibility reporter XML data.
//--------------------------------------------------------------

type TradeFedTestFailure struct {
	Message    string `xml:"message,attr"`
	StackTrace string `xml:"StackTrace"`
}

type TradeFedTest struct {
	Result  string `xml:"result,attr"`
	Abi     string
	Name    string              `xml:"name,attr"`
	Failure TradeFedTestFailure `xml:"Failure"`
}

type TradefedTestCase struct {
	Name  string         `xml:"name,attr"`
	Tests []TradeFedTest `xml:"Test"`
}

type Reason struct {
	Message   string `xml:"message,attr"`
	ErrorName string `xml:"error_name,attr"`
	ErrorCode string `xml:"error_code,attr"`
}

type Module struct {
	Name       string             `xml:"name,attr"`
	Abi        string             `xml:"abi,attr"`
	Runtime    string             `xml:"runtime,attr"`
	Done       string             `xml:"done,attr"`
	Pass       string             `xml:"pass,attr"`
	TotalTests string             `xml:"total_tests,attr"`
	TestCases  []TradefedTestCase `xml:"TestCase"`
	Reason     Reason             `xml:"Reason"`
}

type Result struct {
	// Start time and end time in milliseconds.
	Start    string   `xml:"start,attr"`
	End      string   `xml:"end,attr"`
	BuildNum string   `xml:"suite_build_number,attr"`
	Modules  []Module `xml:"Module"`
}

//--------------------------------------------------------------
// Data structures for parsing LUCI reporter JSON data.
//--------------------------------------------------------------

type LuciJSONResult struct {
	Tr []struct {
		TestID        string  `json:"testId"`
		Name          string  `json:"result_name"`
		Status        string  `json:"status"`
		Expected      bool    `json:"expected"`
		FailureReason string  `json:"failureReason"`
		Duration      float32 `json:"duration"`
		Tags          []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"tags"`
	} `json:"tr"`
}

func selectMostRecentFile(files []string) string {
	var recentFile string
	var recentTime int64 = 0
	for _, file := range files {
		fi, err := os.Stat(file)
		if err == nil {
			fileTime := fi.ModTime().Unix()
			if fileTime > recentTime {
				recentTime = fileTime
				recentFile = file
			}
		}
	}
	return recentFile
}

func selectFileByPattern(pattern string) (string, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files found matching pattern: %s", pattern)
	}

	if len(files) == 1 {
		return files[0], nil
	}

	recentFile := selectMostRecentFile(files)
	if len(recentFile) > 0 {
		return recentFile, nil
	} else {
		return "", fmt.Errorf("no files found matching pattern: %s", pattern)
	}
}

// Unmarshal XML or JSON result file into a given struct.
func parseResultFile(logger *log.Logger, resultPath string, v any) error {
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		logger.Println("Failed to read Tradefed result file: ", err)
		return err
	}
	if strings.HasSuffix(resultPath, ".json") {
		err = json.Unmarshal(resultData, v)
	} else {
		// Use XML parser for all other result files.
		err = xml.Unmarshal(resultData, v)
	}
	if err != nil {
		logger.Println("Failed to parse Tradefed result file: ", err)
		return err
	}
	return nil
}

func uncompressGzip(gzFile string, outFileName string) error {
	gz, err := os.Open(gzFile)
	if err != nil {
		return fmt.Errorf("error opening GZIP file: %s, error: %w", gzFile, err)
	}
	uncompressedStream, err := gzip.NewReader(gz)
	if err != nil {
		return fmt.Errorf("error extracting GZIP file: %s, error: %w", gzFile, err)
	}
	// Make sure target path exists.
	targetDir := filepath.Dir(outFileName)
	cmd := exec.Command("mkdir", "-p", targetDir)
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("error creating target directory: %s, error: %w", targetDir, err)
	}
	outFile, err := os.Create(outFileName)
	if err != nil {
		return fmt.Errorf("error creating output file: %s, error: %w", gzFile, err)
	}
	defer outFile.Close()
	if _, err = io.Copy(outFile, uncompressedStream); err != nil {
		return fmt.Errorf("error extracting file: %s, error: %w", gzFile, err)
	}
	return nil
}

func parseCompatibilityXMLResults(logger *log.Logger, testType string, req *api.CrosTestRequest) ([]*api.TestCaseResult, []*api.CrosTestResponse_GivenTestResult, []string, error) {
	var logsDir []string
	allTestCases := []*api.TestCaseResult{}

	xmlResultFile, err := selectFileByPattern(tfAospResultPath)
	if err != nil {
		// Fallback to stub location for XML result file.
		xmlResultFile, err = selectFileByPattern(filepath.Join(tfStageLogsPath, "*", "*", tfStageResultPattern))
		if err != nil {
			return allTestCases, nil, logsDir, fmt.Errorf("failed to locate result file: %w", err)
		} else {
			standardResultFileName := filepath.Join(filepath.Dir(xmlResultFile), filepath.Base(tfAospResultPath))
			// Unzip stub location XML result file to the standard file name.
			// This is required by CTS Archiver.
			logger.Printf("Extracting stub XML result file: %s to: %s",
				xmlResultFile, standardResultFileName)

			if err = uncompressGzip(xmlResultFile, standardResultFileName); err != nil {
				return allTestCases, nil, logsDir, fmt.Errorf("failed to prepare result file: %w", err)
			} else {
				xmlResultFile = standardResultFileName
			}
		}
	}

	logsDir = append(logsDir, filepath.Join(filepath.Dir(xmlResultFile), "*"))

	logger.Println("Parsing results from: ", xmlResultFile)
	R := Result{}
	if err = parseResultFile(logger, xmlResultFile, &R); err != nil {
		return allTestCases, nil, logsDir, err
	}
	allTestCases, givenTestCases := generateTestCaseResult(logger, testType, R, req)

	return allTestCases, givenTestCases, logsDir, nil
}

func parseLuciResultName(resultName string) (abi string, moduleName string, endtime int64, err error) {
	// Parse ABI, Module name and test time from LUCI result name string.
	// Format of LUCI result name: "<ABI> <module>##<datetime>"

	// First, extract the ABI
	parts1 := strings.SplitN(resultName, " ", 2)
	if len(parts1) != 2 {
		return "", "", 0, fmt.Errorf("could not split ABI from result name: %s", resultName)
	}
	abi = parts1[0]

	// Then, split the remaining part
	parts2 := strings.SplitN(parts1[1], "##", 2)
	if len(parts2) != 2 {
		return "", "", 0, fmt.Errorf("could not parse result name: %s", resultName)
	}
	moduleName = parts2[0]

	// Parse the datetime and convert to timestamp (i.e. "2025-01-06T23:57:37.155198962Z" -> 1736168257)
	t, err := time.Parse(time.RFC3339Nano, parts2[1])
	if err != nil {
		fmt.Println(err)
	}
	endtime = t.UnixMilli()

	return
}

func parseTestName(testName string) (string, string, error) {
	testNameParts := strings.SplitN(testName, "#", 2)
	if len(testNameParts) != 2 {
		return "", "", fmt.Errorf("could not parse test name: %s", testName)
	}

	return testNameParts[0], testNameParts[1], nil
}

func isPassedTest(status string) bool {
	statusUp := strings.ToUpper(status)
	return strings.HasPrefix(statusUp, "PASS") || strings.HasPrefix(statusUp, "SKIP")
}

func convertToResult(logger *log.Logger, luciResult LuciJSONResult) (Result, error) {
	moduleMap := make(map[string]*Module)
	var endTime int64
	var maxDuration int64

	for _, trResult := range luciResult.Tr {
		abi, moduleName, moduleEndTime, err := parseLuciResultName(trResult.Name)
		if err != nil {
			return Result{}, err
		}
		testCaseName, testName, err := parseTestName(trResult.TestID)
		if err != nil {
			return Result{}, err
		}

		status := trResult.Status
		for _, tag := range trResult.Tags {
			if tag.Key == RawStatusKeyName {
				status = tag.Value
			}
		}

		if _, ok := moduleMap[moduleName]; !ok {
			moduleMap[moduleName] = &Module{
				Name:    moduleName,
				Abi:     abi,
				Runtime: strconv.FormatInt(int64(trResult.Duration*1000), 10),
				Done:    "true",
			}
		}

		module := moduleMap[moduleName]
		var testCase *TradefedTestCase
		for i := range module.TestCases {
			if module.TestCases[i].Name == testCaseName {
				testCase = &module.TestCases[i]
				break
			}
		}
		if testCase == nil {
			module.TestCases = append(module.TestCases, TradefedTestCase{Name: testCaseName})
			testCase = &module.TestCases[len(module.TestCases)-1]
		}

		tfTest := TradeFedTest{
			Name:   testName,
			Abi:    abi,
			Result: status,
		}
		if len(trResult.FailureReason) > 0 && !trResult.Expected {
			tfTest.Failure = TradeFedTestFailure{Message: trResult.FailureReason}
		}
		testCase.Tests = append(testCase.Tests, tfTest)

		// Keep track of latest endTime and max test runtime duration.
		currModuleRuntime, err := strconv.Atoi(module.Runtime)
		if err == nil {
			currModuleRuntime = 1
		}
		moduleDuration := int64(trResult.Duration * 1000)
		if moduleDuration > maxDuration {
			maxDuration = moduleDuration
		}
		if moduleDuration > int64(currModuleRuntime) {
			module.Runtime = strconv.FormatInt(moduleDuration, 10)
		}

		if moduleEndTime > endTime {
			endTime = moduleEndTime
		}
	}

	var modules []Module
	for _, module := range moduleMap {
		passCount := 0
		totalCount := 0
		for _, testCase := range module.TestCases {
			for _, test := range testCase.Tests {
				totalCount++
				if isPassedTest(test.Result) {
					passCount++
				}
			}
		}
		module.Pass = strconv.Itoa(passCount)
		module.TotalTests = strconv.Itoa(totalCount)
		modules = append(modules, *module)
	}

	return Result{
		Start:   strconv.FormatInt(endTime-maxDuration, 10),
		End:     strconv.FormatInt(endTime, 10),
		Modules: modules,
	}, nil
}

func parseLuciJSONResults(logger *log.Logger, testType string, req *api.CrosTestRequest) ([]*api.TestCaseResult, []*api.CrosTestResponse_GivenTestResult, []string, error) {
	var logsDir []string
	allTestCases := []*api.TestCaseResult{}

	resultFilePath := filepath.Join(getGlobalLogPath(), "*", tfLuciResultPattern)
	jsonResultFile, err := selectFileByPattern(resultFilePath)
	if err != nil {
		return allTestCases, nil, logsDir, fmt.Errorf("failed to locate result file: %w", err)
	}

	logsDir = append(logsDir, filepath.Join(filepath.Dir(jsonResultFile), "*"))

	logger.Println("Parsing JSON results from: ", jsonResultFile)
	TR := LuciJSONResult{}
	if err = parseResultFile(logger, jsonResultFile, &TR); err != nil {
		return allTestCases, nil, logsDir, err
	}

	R, err := convertToResult(logger, TR)
	if err != nil {
		return allTestCases, nil, logsDir, err
	}

	allTestCases, givenTestCases := generateTestCaseResult(logger, testType, R, req)

	// Save location of additional (staged) logs.
	stagedLogFile, err := selectFileByPattern(filepath.Join(tfStageLogsPath, "*", "*", tfStageLogPattern))
	if err == nil {
		logsDir = append(logsDir, filepath.Join(filepath.Dir(stagedLogFile), "*"))
	}

	return allTestCases, givenTestCases, logsDir, nil
}

func testResponseNoXMLFound(req *api.CrosTestRequest) *api.CrosTestResponse {
	r := &api.CrosTestResponse{}
	if req == nil || req.GetTestSuites() == nil {
		return r
	}
	for _, suites := range req.GetTestSuites() {
		for _, testCaseIds := range suites.GetTestCaseIds().GetTestCaseIds() {
			testCaseResult := buildTcResult(testCaseIds.GetValue(), "", "INCOMPLETE", time.Now().Add(-1*time.Second), 1, "", "")
			r.TestCaseResults = append(r.TestCaseResults, testCaseResult)
			r.GivenTestResults = append(r.GivenTestResults, &api.CrosTestResponse_GivenTestResult{
				ParentTest:           testCaseIds.GetValue(),
				ChildTestCaseResults: []*api.TestCaseResult{testCaseResult},
			})
		}
	}
	return r
}

func generateTestCaseResult(logger *log.Logger, testType string, R Result, req *api.CrosTestRequest) ([]*api.TestCaseResult, []*api.CrosTestResponse_GivenTestResult) {
	allTestCases := []*api.TestCaseResult{}
	givenTestCases := []*api.CrosTestResponse_GivenTestResult{}
	startTimeMs, _ := strconv.ParseInt(R.Start, 10, 64)
	startTime := time.UnixMilli(startTimeMs)
	logger.Println("Found n module results:", len(R.Modules))
	modulesFromReq := getAllModulesFromReq(req)
	for _, module := range R.Modules {
		moduleName := fmt.Sprintf("tradefed.%s.%s", testType, module.Name)
		logger.Println("Parsing Module: ", module.Name)

		delete(modulesFromReq, moduleName)
		nTests, _ := strconv.Atoi(module.TotalTests)
		runTime, _ := strconv.Atoi(module.Runtime)
		// Make a duration of atleast 1 second so the proto isnt empty.
		testDur := int64(runTime / 1000)
		if testDur == 0 {
			testDur = 1
		}
		if nTests != 0 {
			testDur = testDur / int64(nTests)
		} else {
			// Check and report module-level error.
			if module.Reason.Message != "" {
				allTestCases = append(allTestCases,
					buildTcResult(moduleName, module.Abi, "FAILED", startTime, testDur, module.Reason.Message, ""))
				givenTestCases = append(givenTestCases, &api.CrosTestResponse_GivenTestResult{
					ParentTest: moduleName,
					ChildTestCaseResults: []*api.TestCaseResult{
						buildTcResult(moduleName, module.Abi, "FAILED", startTime, testDur, module.Reason.Message, ""),
					},
				})
			} else {
				// No tests in the module and no errors, so reporting this module as 'pass'.
				allTestCases = append(allTestCases,
					buildTcResult(moduleName, module.Abi, "PASSED", startTime, testDur, "", ""))
				givenTestCases = append(givenTestCases, &api.CrosTestResponse_GivenTestResult{
					ParentTest: moduleName,
					ChildTestCaseResults: []*api.TestCaseResult{
						buildTcResult(moduleName, module.Abi, "PASSED", startTime, testDur, "", ""),
					},
				})
			}
		}

		givenTestCase := &api.CrosTestResponse_GivenTestResult{}
		givenTestCase.ParentTest = moduleName
		childTestCases := []*api.TestCaseResult{}
		for _, testcase := range module.TestCases {
			for _, test := range testcase.Tests {
				fullTestName := fmt.Sprintf("tradefed.%s.%s#%s#%s", testType, module.Name, testcase.Name, test.Name)
				errorMessage := test.Failure.Message
				abi := module.Abi
				if len(test.Abi) > 0 {
					// If test ABI is available, use it instead of the module ABI.
					abi = test.Abi
				}
				allTestCases = append(allTestCases,
					buildTcResult(fullTestName, abi, test.Result, startTime, testDur, errorMessage, test.Failure.StackTrace))
				testName := fmt.Sprintf("%s#%s", testcase.Name, test.Name)
				childTestCases = append(childTestCases,
					buildTcResult(testName, abi, test.Result, startTime, testDur, errorMessage, test.Failure.StackTrace))
			}
		}
		givenTestCase.ChildTestCaseResults = childTestCases
		givenTestCases = append(givenTestCases, givenTestCase)
	}
	for moduleName := range modulesFromReq {
		allTestCases = append(allTestCases,
			buildTcResult(moduleName, "", "SKIPPED", startTime, 1, incompleteError, ""))
		givenTestCases = append(givenTestCases,
			&api.CrosTestResponse_GivenTestResult{
				ParentTest: moduleName,
				ChildTestCaseResults: []*api.TestCaseResult{
					buildTcResult(moduleName, "", "SKIPPED", startTime, 1, incompleteError, ""),
				},
			})
	}
	return allTestCases, givenTestCases
}

func getAllModulesFromReq(req *api.CrosTestRequest) map[string]struct{} {
	modules := make(map[string]struct{})
	if req == nil || req.GetTestSuites() == nil {
		return modules
	}
	for _, suites := range req.GetTestSuites() {
		for _, testCaseIds := range suites.GetTestCaseIds().GetTestCaseIds() {
			// Split on a space, incase the request is module + class.
			// We need to get back to just the module to make a result for the "parent"
			modules[moduleNameFromID(testCaseIds.GetValue())] = struct{}{}
		}
	}
	return modules
}

func buildErrorResult(logger *log.Logger, testType string, req *api.CrosTestRequest, err error) *api.CrosTestResponse {
	f := &api.CrosTestResponse{}
	if req == nil || req.GetTestSuites() == nil {
		return f
	}
	errMsg := fmt.Sprintf("Failed test setup/command: %s", err)

	for _, suites := range req.GetTestSuites() {
		for _, testCaseIds := range suites.GetTestCaseIds().GetTestCaseIds() {
			testCaseResult := buildTcResult(testCaseIds.GetValue(), "", "FAILED", time.Now().Add(-1*time.Second), 1, errMsg, "")
			f.TestCaseResults = append(f.TestCaseResults, testCaseResult)
			f.GivenTestResults = append(f.GivenTestResults, &api.CrosTestResponse_GivenTestResult{
				ParentTest:           testCaseIds.GetValue(),
				ChildTestCaseResults: []*api.TestCaseResult{testCaseResult},
			})
		}
	}

	return f
}

func buildTradefedResult(logger *log.Logger, testType string, req *api.CrosTestRequest) (*api.CrosTestResponse, []string) {
	// List of glob patterns of artifacts that should be saved in test results.
	var logsToSave []string

	logger.Println("Building results...")
	f := &api.CrosTestResponse{}

	var err error
	if testType == "custom" {
		// For custom tests, we need to try parsing all possible outputs.
		f.TestCaseResults, f.GivenTestResults, logsToSave, err = parseLuciJSONResults(logger, testType, req)
		if err != nil {
			f.TestCaseResults, f.GivenTestResults, logsToSave, err = parseCompatibilityXMLResults(logger, testType, req)
		}
	} else if isNonXtsTest(testType) {
		f.TestCaseResults, f.GivenTestResults, logsToSave, err = parseLuciJSONResults(logger, testType, req)
	} else {
		f.TestCaseResults, f.GivenTestResults, logsToSave, err = parseCompatibilityXMLResults(logger, testType, req)
	}
	if err != nil {
		logger.Println("Failed to locate or parse Tradefed result file: ", err)
		return testResponseNoXMLFound(req), logsToSave
	}
	logger.Println("Found n case results:", len(f.TestCaseResults))

	return f, logsToSave
}

func buildTcResult(testName string, abi string, testStatus string, startTime time.Time, duration int64,
	errorMessage string, stackTrace string) *api.TestCaseResult {

	tcResult := new(api.TestCaseResult)

	tcResult.TestHarness = &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{Tradefed: &api.TestHarness_Tradefed{}}}
	tcResult.TestCaseId = &api.TestCase_Id{Value: testName}
	if len(abi) > 0 {
		tcResult.Tags = []*api.TestCase_Tag{{Value: fmt.Sprintf("abi:%s", abi)}}
	}

	tcResult.StartTime = timestamppb.New(startTime)
	// Make a duration of atleast 1 second so the proto isnt empty.
	testDuration := duration
	if testDuration == 0 {
		testDuration = 1
	}
	tcResult.Duration = &durationpb.Duration{Seconds: testDuration}

	testError := &api.TestCaseResult_Error{Message: errorMessage, StackTrace: stackTrace}

	switch testStatus {
	case "PASSED", "pass":
		tcResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
	case "ASSUMPTION_FAILURE":
		tcResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
		tcResult.Errors = []*api.TestCaseResult_Error{testError}
	case "FAILED", "FAILURE", "fail":
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
		tcResult.Errors = []*api.TestCaseResult_Error{testError}
	case "INCOMPLETE":
		tcResult.Verdict = &api.TestCaseResult_Crash_{Crash: &api.TestCaseResult_Crash{}}
		tcResult.Errors = []*api.TestCaseResult_Error{testError}
	case "SKIPPED":
		tcResult.Verdict = &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}}
		tcResult.Errors = []*api.TestCaseResult_Error{testError}
	case "IGNORED":
		tcResult.Verdict = &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}}
		tcResult.Errors = []*api.TestCaseResult_Error{testError}
	default:
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
		tcResult.Errors = []*api.TestCaseResult_Error{testError}
	}

	tcResult.TestCaseMetadata = &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Id:   tcResult.TestCaseId,
			Name: extractTestCaseName(tcResult.TestCaseId.Value),
		},
		TestCaseExec: &api.TestCaseExec{TestHarness: tcResult.TestHarness},
	}

	return tcResult
}
