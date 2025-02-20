// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package driver

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// TestNewCrosierDriver ensures the crosier driver is built correctly
func TestNewCrosierDriver(t *testing.T) {
	l := new(log.Logger)

	crosierDriver := NewCrosierDriver(l)

	if crosierDriver.logger != l {
		t.Errorf("Got unexpected logger (%v), want (%v)", crosierDriver.logger, l)
	}
}

// TestGtestName ensures the proper name is associated with the driver
func TestCrosierName(t *testing.T) {
	const expectedName string = "crosier"

	l := new(log.Logger)

	crosierDriver := NewCrosierDriver(l)
	driverName := crosierDriver.Name()
	if diff := cmp.Diff(driverName, expectedName); diff != "" {
		t.Errorf("Got unexpected argument from CrosierDriver.Name (-got +want):\n%s\n%v\n--\n%v\n", diff, driverName, expectedName)
	}
}

// TestBuiltTestCaseResultsPass ensures passing results
// proto generation is correct for Crosier harness.
func TestCrosierBuiltTestCaseResultsPass(t *testing.T) {
	expectedResult := new(api.TestCaseResult)
	tcID := "gtest.fake.test"
	startTime := time.Now()
	reasons := []string{}
	duration := int64(100)
	harness := api.TestHarness{TestHarnessType: &api.TestHarness_Crosier_{Crosier: &api.TestHarness_Crosier{}}}

	result := newExecutionData(startTime, duration, reasons)

	expectedResult.TestCaseId = &api.TestCase_Id{Value: tcID}
	expectedResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
	expectedResult.Reason = strings.Join(reasons, "\n")
	expectedResult.TestHarness = &harness
	expectedResult.StartTime = timestamppb.New(startTime)
	expectedResult.Duration = &durationpb.Duration{Seconds: duration}

	actualResult := buildTestCaseResults(tcID, &harness, result)

	if !proto.Equal(actualResult, expectedResult) {
		t.Errorf("unexpected test results for 'pass' (-got +want):\n%v\n--\n%v\n", actualResult, expectedResult)
	}
}

// TestParseCrosierTestResultString parses a sample test result XML
// and validates that all fields are parsed correctly.
func TestParseCrosierTestResultString(t *testing.T) {
	rawResult := []byte(`<?xml version="1.0" encoding="UTF-8"?>
		<testsuites name="AllTests" tests="1" failures="0" disabled="0" errors="0" time="0.782" timestamp="2024-01-17T23:11:33">
		<testsuite name="StubTest" tests="1" failures="0" disabled="0" errors="0" time="0.782" timestamp="2024-01-17T23:11:33">
			<testcase name="Positive" status="run" time="0.782" timestamp="2024-01-17T23:11:33" classname="StubTest">
				<failure message="Failure reason" type=""></failure>
			</testcase>
		</testsuite>
		</testsuites>`)
	var actualData gtestResult
	if err := parseResultXML(rawResult, &actualData); err != nil {
		t.Errorf("error parsing Crosier result XML: %v", err)
	}

	gtestResult := gtestResult{}
	gtestResult.XMLName.Local = "testsuites"
	gtestResult.TestSuites = append(gtestResult.TestSuites, gtestSuite{})
	gtestResult.TestSuites[0].Name = "StubTest"
	gtestResult.TestSuites[0].Tests = 1
	gtestResult.TestSuites[0].Failures = 0
	gtestResult.TestSuites[0].Time = 0.782
	gtestResult.TestSuites[0].Timestamp = "2024-01-17T23:11:33"
	gtestResult.TestSuites[0].TestCases = append(gtestResult.TestSuites[0].TestCases, gtestCase{})
	gtestResult.TestSuites[0].TestCases[0].Classname = "StubTest"
	gtestResult.TestSuites[0].TestCases[0].Name = "Positive"
	gtestResult.TestSuites[0].TestCases[0].Status = "run"
	gtestResult.TestSuites[0].TestCases[0].Timestamp = "2024-01-17T23:11:33"
	gtestResult.TestSuites[0].TestCases[0].Time = 0.782
	gtestResult.TestSuites[0].TestCases[0].Failures = append(gtestResult.TestSuites[0].TestCases[0].Failures, gtestFailure{})
	gtestResult.TestSuites[0].TestCases[0].Failures[0].Message = "Failure reason"
	gtestResult.Name = "AllTests"
	gtestResult.Tests = 1
	gtestResult.Failures = 0
	gtestResult.Disabled = 0
	gtestResult.Errors = 0
	gtestResult.Time = 0.782
	gtestResult.Timestamp = "2024-01-17T23:11:33"

	if diff := cmp.Diff(actualData, gtestResult); diff != "" {
		t.Errorf("unexpected result for 'reasons' (-got +want):\n%s\n%v\n--\n%v\n", diff, actualData, gtestResult)
	}
}
