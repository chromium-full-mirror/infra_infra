// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/cft/common/adb"
	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/common"
	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/device"
	"go.chromium.org/infra/cros/cmd/common_lib/secretmanager"
)

const (
	tradefedDir          = "/tradefed"
	tradefedAospBinary   = "tradefed.sh"
	tradefedGoogleBinary = "tradefed_runner.sh"
	tradefedGlobalLogs   = "tradefed_global_log_*.txt"
	crossDeviceErr       = "invalid cross-device link"
	gtsAccountKeyFile    = "/tmp/test/gts-arc.json"
	gtsKeySecretName     = "gts-arc"
	gtsKeyProjectNumber  = 501735538094
)

// List of xTS & non-xTS test suites supported by this driver.
// Not all suites are supported for each Tradefed type.
var knownSuites = []string{"cts", "dts", "gts", "vts", "sts", "apts", "general", "custom"}
var nonXtsSuites = []string{"general"}

var testType = "cts"
var tradefedType = "aosp"

type TradefedDriver struct {
	logger *log.Logger
}

func NewTradefedDriver(logger *log.Logger) *TradefedDriver {
	return &TradefedDriver{
		logger: logger,
	}
}
func (td *TradefedDriver) Name() string {
	return "tradefed"
}

func detectTradefedType() {
	if _, err := os.Stat(filepath.Join(tradefedDir, "google-tradefed.jar")); err == nil {
		tradefedType = "google"
	} else {
		tradefedType = "aosp"
	}
}

func isAospTradefed() bool {
	return tradefedType == "aosp"
}

func getTradefedBinary() string {
	if isAospTradefed() {
		return fmt.Sprintf("%s-tradefed", testType)
	} else {
		return tradefedGoogleBinary
	}
}

func isTestTypeSupported(test string) bool {
	for _, prefix := range knownSuites {
		if strings.HasPrefix(test, prefix) {
			return true
		}
	}
	return false
}

func isNonXtsTest(testType string) bool {
	for _, suite := range nonXtsSuites {
		if testType == suite {
			return true
		}
	}
	return false
}

func detectTestType(tests []*api.TestCaseMetadata) string {
	for _, test := range tests {
		testName := test.GetTestCase().GetName()
		// Check for test name prefix, i.e. "xts.TestModule"
		if strings.Index(testName, ".") > 1 && isTestTypeSupported(testName) {
			return testName[:strings.Index(testName, ".")]
		}
		// Check for suite tags, i.e. "suite:xts"
		for _, tag := range test.GetTestCase().GetTags() {
			tagVal := tag.GetValue()
			if strings.HasPrefix(tagVal, "suite:") && isTestTypeSupported(tagVal[6:]) {
				return tagVal[6:]
			}
		}
	}

	// Return default test suite if no other suite detected.
	return "cts"
}

func runTradefedTest(ctx context.Context, logger *log.Logger, tests []*api.TestCaseMetadata,
	serials []string, resultsPath string, metadata *api.ExecutionMetadata, board string, args map[string][]string,
	model string, servo *labapi.Servo) error {

	for _, s := range serials {
		// TODO(b/393175524): Switch back to SetupAdb() after we understand the
		// regression or if this doesn't help.
		if err := adb.RetrySetupAdb(logger, s, 15*time.Second); err != nil {
			return fmt.Errorf("setupAdb failed for %s", s)
		}
		// Force the disablement of the test_harness setting to resolve bootloops.
		_, err := adb.AdbCmd([]string{"-s", adb.FmtAddr(s), "root"}, logger)

		if err != nil {
			logger.Println("Failed to establish ADB root post test")
		}
		err = adb.RetrySetupAdb(logger, s, 15*time.Second)
		if err != nil {
			logger.Println("Failed to recoonec to ADB post test")
		}
		_, _ = adb.AdbShellCmd([]string{"echo", "demo", ">", "/sys/power/wake_lock"}, s, logger)

		_, _ = adb.AdbShellCmd([]string{"cat", "/sys/power/wake_lock"}, s, logger)
	}

	exit := make(chan struct{})
	defer func() {
		exit <- struct{}{}
		for _, s := range serials {

			// Force the disablement of the test_harness setting to resolve bootloops.
			_, err := adb.AdbCmd([]string{"-s", adb.FmtAddr(s), "root"}, logger)

			if err != nil {
				logger.Println("Failed to establish ADB root post test")
			}
			err = adb.RetrySetupAdb(logger, s, 15*time.Second)
			if err != nil {
				logger.Println("Failed to recoonec to ADB post test")
			}

			_, _ = adb.AdbShellCmd([]string{"setprop", "persist.sys.test_harness", "0"}, s, logger)

			if err := adb.TeardownAdb(logger, s); err != nil {
				logger.Printf("Failed to tear down adb connection to %s: %s", s, err)
			}
		}
	}()

	prepareEnvironment(ctx, logger)

	var cmd *exec.Cmd

	// Using Google TradeFed console for CTS and DTS tests.
	baseArgs := []string{"run", "commandAndExit"}
	if isNonXtsTest(testType) {
		baseArgs = append(baseArgs, BuildNonXtsTestCommand(logger, testType, tests,
			serials, metadata, board, args, model, servo)...)
	} else if testType == "custom" {
		baseArgs = append(baseArgs, BuildCustomTestCommand(logger, testType, tests,
			serials, metadata, board, args, model, servo)...)
	} else {
		baseArgs = append(baseArgs, BuildXtsTestCommand(logger, testType, tests,
			serials, metadata, board, args, model, servo)...)
	}
	cmd = exec.Command(getTradefedBinary(), baseArgs...)

	logger.Println("Running TF: ", cmd.String())

	adb.KeepAdbAlive(logger, serials, exit)
	return launchAndRead(cmd, logger)
}

// RunTests drives a test framework to execute tests.
func (td *TradefedDriver) RunTests(ctx context.Context, resultsDir string, req *api.CrosTestRequest, tlwAddr string, tests []*api.TestCaseMetadata) (*api.CrosTestResponse, error) {
	allRspn := &api.CrosTestResponse{}

	serials, err := device.DerviceSerials(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call DerviceSerials: %w", err)
	}

	detectTradefedType()
	td.logger.Println("Detected Tradefed type: ", tradefedType)

	testType = detectTestType(tests)
	td.logger.Println("Detected test type: ", testType)

	executionMD := &api.ExecutionMetadata{}
	if len(req.GetTestSuites()) > 0 {
		executionMD = req.GetTestSuites()[0].GetExecutionMetadata()
	}

	chromeOS := req.GetPrimary().GetDut().GetChromeos()
	err = runTradefedTest(ctx, td.logger, tests, serials, resultsDir, executionMD,
		chromeOS.GetDutModel().GetBuildTarget(),
		getArgs(req),
		chromeOS.GetDutModel().GetModelName(),
		chromeOS.GetServo())

	var results *api.CrosTestResponse
	var artifacts []string
	if err != nil {
		// Error in test setup, DUT initialization or starting Tradefed command.
		// In all these cases, we report each request test as "Failed" with
		// an appropriate error message.
		results = buildErrorResult(td.logger, testType, req, err)
	} else {
		results, artifacts = buildTradefedResult(td.logger, testType, req)
	}
	if results.GetTestCaseResults() != nil {
		allRspn.TestCaseResults = append(allRspn.TestCaseResults, results.TestCaseResults...)
		allRspn.GivenTestResults = append(allRspn.GivenTestResults, results.GivenTestResults...)
	} else {
		td.logger.Println("No results to report: ", results)
	}

	// Adding all global TradeFed logs to the list of artifacts.
	artifacts = append(artifacts, filepath.Join(os.Getenv("GLOBAL_LOG_PATH"), tradefedGlobalLogs))

	td.logger.Println("Collecting result artifacts to:", resultsDir)
	for _, artifact := range artifacts {
		if len(artifact) > 0 {
			td.moveArtifacts(resultsDir, artifact)
		}
	}

	return allRspn, nil
}

// getArgs extracts arguments from the test request.
// It supports multiple values for the same flag (non-unique keys).
func getArgs(req *api.CrosTestRequest) map[string][]string {
	args := make(map[string][]string)

	suites := req.GetTestSuites()
	// Check if there are any test suites.
	if len(suites) > 0 {
		// Assuming the metadata is the same for all test suites.
		metadata := suites[0].GetExecutionMetadata()
		if metadata != nil {
			rawArgs := metadata.GetArgs()
			for _, rawArg := range rawArgs {
				flag := rawArg.GetFlag()
				value := rawArg.GetValue()

				// Only process if both flag and value are non-empty.
				if flag != "" && value != "" {
					args[flag] = append(args[flag], value)
				}
			}
		}
	}
	return args
}

// Move artifact files to resultsDir, deleting the source files. Supports glob patterns.
// Doesn't remove source directory, only files/dirs inside.
func (td *TradefedDriver) moveArtifacts(resultsDir string, artifacts string) {
	matches, err := filepath.Glob(artifacts)
	if err != nil {
		td.logger.Printf("Failed to match %q: %v", artifacts, err)
		return
	}

	for _, match := range matches {
		td.logger.Printf("Moving result artifact: %q to: %q", match, resultsDir)
		if err := moveSingleArtifact(resultsDir, match); err != nil {
			td.logger.Printf("Failed to move %q to: %q, error: %v", match, resultsDir, err)
		}
	}
}

func moveSingleArtifact(resultsDir string, artifactPath string) error {
	destPath := filepath.Join(resultsDir, filepath.Base(artifactPath))
	err := os.Rename(artifactPath, destPath)
	if err == nil {
		return nil
	}

	// Copy-paste file contents in case of cross-device link error
	if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err.Error() == crossDeviceErr {
		sourceFile, err := os.Open(artifactPath)
		if err != nil {
			return err
		}
		defer sourceFile.Close()

		destinationFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer destinationFile.Close()

		if _, err = io.Copy(destinationFile, sourceFile); err != nil {
			return err
		}

		return os.Remove(artifactPath)
	}

	// Return the original error if it's not a cross-device link error
	return err
}

func launchAndRead(cmd *exec.Cmd, logger *log.Logger) error {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("StderrPipe failed")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("StdoutPipe failed")
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run Tradefed: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		common.TestScanner(stderr, logger, "Tradefed")
	}()

	go func() {
		defer wg.Done()
		common.TestScanner(stdout, logger, "Tradefed")
	}()

	wg.Wait()
	return nil
}

func moduleNameFromID(id string) string {
	sections := strings.Split(id, " ")
	if len(sections) > 0 {
		return sections[0]
	}
	return id
}

func prepareEnvironment(ctx context.Context, logger *log.Logger) {
	if !isAospTradefed() && testType == "gts" {
		// Internal runs of GTS might require service account key.
		if _, err := os.Stat(gtsAccountKeyFile); err != nil && os.IsNotExist(err) {
			// If not already present, fetch key "secret" and write it to disk.
			keyData, err := secretmanager.GetSecretBytes(ctx, gtsKeySecretName, gtsKeyProjectNumber, 1)
			if err != nil {
				// Do not return error - try to run without the key.
				logger.Printf("Unable to fetch GTS service account key: %s\n", err)
			} else {
				err = os.WriteFile(gtsAccountKeyFile, keyData, 0666)
				if err != nil {
					logger.Printf("Unable to write GTS service account key to disk: %s\n", err)
				} else {
					logger.Printf("GTS service account key written to file: %s\n", gtsAccountKeyFile)
				}
			}
		} else {
			logger.Printf("Reusing GTS service account key file: %s\n", gtsAccountKeyFile)
		}
	}
}
