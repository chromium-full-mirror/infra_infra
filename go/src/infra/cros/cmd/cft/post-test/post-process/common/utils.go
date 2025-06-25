// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/artifact"

	"go.chromium.org/infra/cros/cmd/cft/common/errors"
)

const (
	TastTestPrefix  = "tast."
	TautoTestPrefix = "tauto."
	TautoSubDir     = "tauto/"
	TastSubDir      = "tast/"
	TastTestsDir    = "tast/results/tests"
)

// CreateLogFile creates a file and its parent directory for logging purpose.
func CreateLogFile(fullPath string) (*os.File, error) {
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create directory %v: %w", fullPath, err))
	}

	logFullPathName := filepath.Join(fullPath, "log.txt")

	// Log the full output of the command to disk.
	logFile, err := os.Create(logFullPathName)
	if err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create file %v: %w", fullPath, err))
	}
	return logFile, nil
}

// NewLogger creates a logger. Using go default logger for now.
func NewLogger(logFile *os.File) *log.Logger {
	mw := io.MultiWriter(logFile, os.Stderr)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}

// TestLevelFiles gets the specified file for each test result and returns a
// map from test name to the specified file path.
func TestLevelFiles(logger *log.Logger, testResult *artifact.TestResult, fileName string) map[string]string {
	logger.Printf("Started to fetch: %q file for each test", fileName)
	files := map[string]string{}
	for _, testRun := range testResult.GetTestRuns() {
		testCaseResult := testRun.GetTestCaseInfo().GetTestCaseResult()
		if testCaseResult == nil {
			continue
		}

		testName := testCaseResult.GetTestCaseId().GetValue()
		if testName == "" {
			continue
		}

		// Constructs the file path.
		resultDirPath := testCaseResult.GetResultDirPath().GetPath()
		file := ""
		if strings.Contains(resultDirPath, TastSubDir) {
			// For Tast first class test
			file = resultDirPath + "/" + fileName
		} else if strings.Contains(resultDirPath, TautoSubDir) {
			if strings.HasPrefix(testName, TastTestPrefix) {
				// For Tast second class test wrapped by Tauto test suite
				baseTestName, _ := strings.CutPrefix(testName, TastTestPrefix)
				file = resultDirPath + "/" + TastTestsDir + "/" + baseTestName + "/" + fileName
			} else {
				// For Tauto test
				baseTestName, _ := strings.CutPrefix(testName, TautoTestPrefix)
				file = resultDirPath + "/" + baseTestName + "/" + fileName
			}
		}

		files[testName] = file
	}

	logger.Printf("Successfully fetched: %q file for each test: %q", fileName, files)
	return files
}
