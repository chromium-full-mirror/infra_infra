// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package commands

import (
	"context"
	"log"
	"os"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"

	"go.chromium.org/infra/cros/cmd/cft/common/files"
	"go.chromium.org/infra/cros/cmd/cft/post-test/post-process/common"
)

const (
	FixturesSuffix  = "fixtures"
	MaxReadDirDepth = 100
	ServoFileName   = "servo_info.json"
	TastServoDir    = "servoHook/"
	TautoServoDir   = "sysinfo"
	TestsPrefix     = "tests/"
)

// ServoInfo retrieves servo info from servo file.
func ServoInfo(logger *log.Logger, testResult *artifact.TestResult) *api.GetServoInfoResponse {
	if testResult == nil {
		return &api.GetServoInfoResponse{}
	}

	servoFilePath := ServoFilePath(logger, testResult)
	ctx := context.Background()
	servoInfoFile, _ := ReadServoInfo(ctx, logger, servoFilePath)
	return &api.GetServoInfoResponse{ServoInfo: servoInfoFile}

}

// TastServoFilePath retrieves path of servo file for tast tests. Tast test
// generates multiple servo files, but we only need to find one servo file
// since the contents of all servo files will be the same.
func TastServoFilePath(logger *log.Logger, path string) string {
	f, err := os.Open(path)
	if err != nil {
		logger.Printf("opening file %q: %s", path, err)
		return ""
	}

	files, _ := f.Readdirnames(MaxReadDirDepth)
	for _, fileName := range files {
		servoFilePath := path + "/" + fileName + "/" + TastServoDir + ServoFileName
		_, err := os.Open(servoFilePath)
		if err == nil {
			logger.Printf("Found servo file: %q", servoFilePath)
			return servoFilePath
		}
	}
	return ""
}

// ServoFilePath retrieves path of servo file.
func ServoFilePath(logger *log.Logger, testResult *artifact.TestResult) string {
	filePath := ""
	for _, testRun := range testResult.GetTestRuns() {
		testCaseResult := testRun.GetTestCaseInfo().GetTestCaseResult()
		if testCaseResult == nil {
			continue
		}

		testName := testCaseResult.GetTestCaseMetadata().GetTestCase().GetName()
		if testName == "" {
			continue
		}

		// Constructs the file path.
		resultDirPath := testCaseResult.GetResultDirPath().GetPath()

		if strings.Contains(resultDirPath, common.TastSubDir) {
			// Tast test
			// Example:
			// resultDirPath = cros-test-fd051ff7/cros-test/results/tast/tests/firmware.H1Console
			// testName = firmware.H1Console
			// baseFixturesPath = cros-test-fd051ff7/cros-test/results/tast/fixtures
			// filePath = cros-test-fd051ff7/cros-test/results/tast/fixtures/tastRootRemoteFixture_cros_20241120003832/servoHook/servo_info.json
			suffix := TestsPrefix + testName
			basePath, _ := strings.CutSuffix(resultDirPath, suffix)
			baseFixturesPath := basePath + FixturesSuffix
			filePath = TastServoFilePath(logger, baseFixturesPath)
		} else if strings.Contains(resultDirPath, common.TautoSubDir) {
			// For Tauto test and Tast second class test wrapped by Tauto test suite
			// Example:
			// resultDirPath = cros-test-b2c6a741/cros-test/results/tauto/results-1-tast.generic-servo
			// filePath = cros-test-b2c6a741/cros-test/results/tauto/results-1-tast.generic-servo/sysinfo/servo_info.json
			filePath = resultDirPath + "/" + TautoServoDir + "/" + ServoFileName
		}
		return filePath
	}
	return ""
}

// ReadServoInfo converts the servo info JSON file into Servo info proto.
func ReadServoInfo(ctx context.Context, logger *log.Logger, servoFilePath string) (*anypb.Any, error) {
	servoInfo := &artifact.BuildMetadata_ServoInfo{}
	err := files.ReadProtoJSONFile(ctx, servoFilePath, servoInfo)
	if err != nil {
		logger.Printf("Failed to read servo info from file: %q with err: %v", servoFilePath, err)
		return anypb.New(&artifact.BuildMetadata_ServoInfo{})
	}

	return anypb.New(servoInfo)
}
