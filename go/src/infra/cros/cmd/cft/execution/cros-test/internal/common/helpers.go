// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common provide command utilities and variables for all components in
// cros-test to use.
package common

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"

	testerrors "go.chromium.org/infra/cros/cmd/cft/common/errors"
	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/device"
)

// TestScanner makes a scanner to read from test streams.
func TestScanner(stream io.Reader, logger *log.Logger, harness string) {
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Println("Failed to read pipe: ", err)
			break
		}
		logger.Printf("[%v] %v", harness, strings.TrimSpace(line))
	}
}

// ExtraArgs upacks the extra arguements from test reqeusts.
func ExtraArgs(req *api.CrosTestRequest) (args []*api.Arg) {
	for _, t := range req.GetTestSuites() {
		args = append(args, t.GetExecutionMetadata().GetArgs()...)
	}
	return args
}

// UnpackMetadata unpacks the Any metadata field into AutotestExecutionMetadata
func UnpackMetadata(req *api.CrosTestRequest) (args []*api.Arg, resultsRootDir string, err error) {
	if req.Metadata == nil {
		return nil, "", nil
	}
	if autotestMetaData, resultsRootDir, err := unpackAutotestMetadata(req); err == nil {
		return autotestMetaData, resultsRootDir, nil
	}
	if tastMetaData, resultsRootDir, err := unpackTastMetadata(req); err == nil {
		return tastMetaData, resultsRootDir, nil
	}
	return unpackUnifiedMetadata(req)
}

// unpackAutotestMetadata unpacks the Any metadata field into AutotestExecutionMetadata
func unpackAutotestMetadata(req *api.CrosTestRequest) (args []*api.Arg, resultRootDir string, err error) {
	if req.Metadata == nil {
		return nil, "", nil
	}
	var m api.AutotestExecutionMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return nil, "", fmt.Errorf("improperly formatted input proto metadata, %w", err)
	}
	for _, a := range m.GetArgs() { // nolint:staticcheck // SA1019
		args = append(args, &api.Arg{Flag: a.GetFlag(), Value: a.GetValue()})
	}
	return args, m.GetResultsSubDir(), nil
}

// unpackTastMetadata unpacks the Any metadata field into TastExecutionMetadata
func unpackTastMetadata(req *api.CrosTestRequest) (args []*api.Arg, resultRootDir string, err error) {
	if req.Metadata == nil {
		return nil, "", nil
	}
	var m api.TastExecutionMetadata // nolint:staticcheck // SA1019
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return nil, "", fmt.Errorf("improperly formatted input proto metadata, %w", err)
	}
	for _, a := range m.GetArgs() {
		args = append(args, &api.Arg{Flag: a.GetFlag(), Value: a.GetValue()})
	}
	return args, "", nil
}

// unpackUnifiedMetadata unpacks the Any metadata field into TastExecutionMetadata
func unpackUnifiedMetadata(req *api.CrosTestRequest) (args []*api.Arg, resultRootDir string, err error) {
	if req.Metadata == nil {
		return nil, "", nil
	}
	var m api.ExecutionMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return nil, "", fmt.Errorf("improperly formatted input proto metadata, %w", err)
	}
	for _, a := range m.GetArgs() {
		args = append(args, &api.Arg{Flag: a.GetFlag(), Value: a.GetValue()})
	}
	return args, "", nil
}

// Companions separates devices to ChromeOS devices and Android devices.
func Companions(devices []*api.CrosTestRequest_Device) (chromeOSCompanions []*device.DutInfo,
	androidCompanions []*device.AndroidInfo, err error) {

	companionCount := 0
	for _, c := range devices {
		if c.Dut.GetChromeos() != nil {
			companionCount++
			info, err := device.FillDUTInfo(c, fmt.Sprintf("cd%d", companionCount))
			if err != nil {
				return nil, nil, testerrors.NewStatusError(testerrors.InvalidArgument,
					fmt.Errorf("cannot get address from companion device: %v", c))
			}
			chromeOSCompanions = append(chromeOSCompanions, info)
			continue
		}
		if c.Dut.GetAndroid() != nil {
			androidInfo := device.FillAndroidInfo(c)
			androidCompanions = append(androidCompanions, androidInfo)
		}
	}
	return chromeOSCompanions, androidCompanions, nil
}

// Fetches the build ID from TestSuite request.
func BuildIDFomSuites(suites []*api.TestSuite) (string, error) {
	for _, suite := range suites {
		for _, arg := range suite.GetExecutionMetadata().GetArgs() {
			if arg.GetFlag() == "build_id" {
				return arg.GetValue(), nil
			}
		}
	}
	return "", fmt.Errorf("no build_id present in suite metadata")
}

// TranslateMoblyResults translate a mobly test result yaml output
// to test results.
func TranslateMoblyResults(moblyYaml []byte, resultsRootDir string) ([]*api.TestCaseResult, error) {
	reader := bytes.NewReader(moblyYaml)
	decoder := yaml.NewDecoder(reader)
	var results []*api.TestCaseResult
	for {
		var node yaml.Node
		err := decoder.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("failed to parse yaml file")
		}
		result, err := parseResult(&node, resultsRootDir)
		if err != nil {
			// Current document does not contain results.
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

func parseResult(node *yaml.Node, resultsRootDir string) (*api.TestCaseResult, error) {
	if node.Kind != yaml.DocumentNode {
		// Results are in document block.
		// Return an error if it is not a document node.
		return nil, errors.New("not an document node")
	}
	for _, c := range node.Content {
		if c.Kind != yaml.MappingNode {
			// Result is in map format.
			continue
		}
		var result map[string]interface{}
		if err := c.Decode(&result); err != nil {
			// Ignore map that we cannot parse.
			continue
		}
		return fillTestCaseResult(result, resultsRootDir)
	}
	return nil, errors.New("document does not contain results")

}

func fillTestCaseResult(raw map[string]interface{}, resultsRootDir string) (*api.TestCaseResult, error) {
	const (
		testClassKey = "Test Class"
		testNameKey  = "Test Name"
		resultKey    = "Result"
		detailKey    = "Details"
		startTimeKey = "Begin Time"
		endTimeKey   = "End Time"
	)
	var name string
	var class string
	var result string
	var detail string
	var startTime time.Time
	var endTime time.Time

	nameValue, ok := raw[testNameKey]
	if !ok {
		return nil, errors.New("map does not have name information")
	}

	name = nameValue.(string)
	if classValue, ok := raw[testClassKey]; ok {
		class = classValue.(string)
	}

	startTimeValue, ok := raw[startTimeKey]
	if !ok || startTimeValue == nil {
		// The map does not have start time information, use current time.
		startTime = time.Now()
	} else {
		startTime = time.UnixMilli(int64(startTimeValue.(int)))
	}

	endTimeValue, ok := raw[endTimeKey]
	if !ok || endTimeValue == nil {
		// The map does not have end time information.
		// Use start time.
		endTime = startTime
	} else {
		endTime = time.UnixMilli(int64(endTimeValue.(int)))
	}

	if resultValue, ok := raw[resultKey]; ok && resultValue != nil {
		result = resultValue.(string)
	}

	if detailValue, ok := raw[detailKey]; ok && detailValue != nil {
		detail = detailValue.(string)
	}
	var testID string
	if class != "" {
		testID = fmt.Sprintf("%s.%s", class, name)
	} else {
		testID = name
	}

	duration := endTime.Sub(startTime)
	protoStartTime := timestamppb.New(startTime)
	protoDuration := durationpb.New(duration)

	r := &api.TestCaseResult{
		TestCaseId: &api.TestCase_Id{Value: testID},
		StartTime:  protoStartTime,
		Duration:   protoDuration,
		ResultDirPath: &_go.StoragePath{
			HostType: _go.StoragePath_LOCAL,
			Path:     resultsRootDir,
		},

		TestHarness: &api.TestHarness{
			TestHarnessType: &api.TestHarness_Mobly_{
				Mobly: &api.TestHarness_Mobly{},
			},
		},
	}

	moblyVerdict := strings.ToUpper(result)
	switch moblyVerdict {
	case "PASS":
		r.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
	case "FAIL", "ERROR":
		r.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
		if detail != "" {
			r.Reason = detail
		} else {
			r.Reason = "Failure reason is not available"
		}
	case "SKIP":
		r.Verdict = &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}}
		if detail != "" {
			r.Reason = detail
		} else {
			r.Reason = "Skip reason is not available"
		}
	}
	return r, nil
}
