// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/common"
)

const (
	planMetadataFlag   = "plan"
	tfGoogleTestRunner = "google/cts/google-cts-launcher-for-aosp"
	tfAospTestRunner   = "tf-aosp-compatibility-config"
	DTS                = "dts"
)

func extractBuildInfo(test *api.TestCase, metadata *api.ExecutionMetadata, board string, plan string, logger *log.Logger) (branch string, target string, build string) {
	branch, target, build = extractTestInfoFromExecutionMetadata(metadata)
	if !(len(branch) > 0 && len(target) > 0 && len(build) > 0) {
		// As no direct way to return error, we continue with what we have and let Tradefed error out if it doesn't have enough information.
		// Print log to make it clear what is missing.
		logger.Println("Missing required info branch/target/build, got: ", branch, "/", target, "/", build)
	}
	logger.Println("Setting xTS build info from build info - branch/target/build: ", branch, "/", target, "/", build)

	return
}

func getTestRunner() string {
	if isAospTradefed() {
		return tfAospTestRunner
	} else {
		return tfGoogleTestRunner
	}
}

func BuildXtsTestCommand(logger *log.Logger, testType string, tests []*api.TestCaseMetadata,
	serials []string, metadata *api.ExecutionMetadata, board string, args map[string][]string, model string,
	servo *labapi.Servo) []string {

	cmd := []string{getTestRunner()}

	plan, err := extractMetadataFlag(metadata, planMetadataFlag)
	if err != nil {
		// By default, plan is the same as test suite type, i.e. "cts", "dts", etc (except for STS).
		if testType == "sts" {
			plan = "sts-dynamic-full"
		} else {
			plan = testType
		}
	}

	// Add test type specific arguments.
	if isAospTradefed() {
		cmd = append(cmd, "--plan", plan, "--log-level-display", "VERBOSE",
			"--test-tag", fmt.Sprintf("cros-%s-test", testType),
			"--use-device-build-info", "--primary-abi-only", "--stage-remote-file",
			"--load-configs-with-include-filters", "--no-skip-staging-artifacts",
			"--result-reporter:use-log-saver", "--result-reporter:no-compress-logs")
	} else {
		cmd = append(cmd, "--config-name", plan,
			"--test-tag", fmt.Sprintf("cros-%s-test", testType),
			"--cts-package-name", fmt.Sprintf("android-%s.zip", testType),
			"--rootdir-var", fmt.Sprintf("%s_ROOT", strings.ToUpper(testType)),
			"--no-set-test-harness", "--jdk-folder-for-subprocess", "/jdk/jdk21/linux-x86")

		if testType == "gts" {
			cmd = append(cmd, "--inject-global-config", "--global-config-filters", "host_options")
		}
	}

	logPath := getGlobalLogPath()

	cmd = append(cmd, "--invocation-timeout", invocationTimeout, "--log-level", "VERBOSE")

	if !isAospTradefed() {

		cmd = append(cmd, "--cts-use-partial-download", "--cts-version", "2",
			"--gdevice-flash:disable",
			"--android-build-api-log-saver:no-remove-staged-files", "--no-use-event-streaming",
			"--google-device-setup:set-global-setting", "verifier_verify_adb_installs=0",
			"--reporter-template", "template/reporters/subprocess-reporter",
			"--template:map", "reporters=template/reporters/xml-reporter.xml",
			"--metricsreporter:metrics-folder", logPath, "--max-run-time", maxRuntime,
			"--google-device-setup:run-command", "am switch-user 10",
		)

		// Adding cts-params with prefix for each one.
		ctsParams := []string{
			"--no-use-device-build-info", "--log-level", "VERBOSE",
			"--max-log-size", "62914560", "--max-tmp-logcat-file", "62914560",
			"--logcat-on-failure", "--screenshot-on-failure", "--include-test-log-tags",
			"--result-reporter:disable-result-posting", "--result-reporter:no-disable",
			"--use-log-saver", "--post-boot-command", `"am switch-user 10"`,
		}
		if testType != "gts" && testType != "vts" && testType != "sts" {
			ctsParams = append(ctsParams, "--property-check:no-throw-error")
		}
		if testType == "sts" {
			ctsParams = append(ctsParams, "--ghidra-preparer:disable")
		}
		for _, param := range ctsParams {
			cmd = append(cmd, "--cts-params", param)
		}

		// Adding a list of artifacts to skip downloading by the CTS runner.
		for _, artifact := range skipDownloadArtifacts {
			cmd = append(cmd, "--skip-download", artifact)
		}

		cmd = append(cmd, buildResultReportingArgs(logger, metadata, args, board, model)...)
	}

	if retryFailedRuns {
		cmd = append(cmd, "--max-testcase-run-count", "3", "--retry-isolation-grade", "FULLY_ISOLATED",
			"--retry-strategy", "RETRY_ANY_FAILURE", "--module-preparation-retry")
	} else {
		cmd = append(cmd, "--max-testcase-run-count", "1", "--retry-strategy", "NO_RETRY")
	}

	var buildInfoReported = false
	var invocationInfoReported = false
	var branch, target, build string
	branch, target, build = extractBuildInfoFromExecutionMetadata(metadata)
	if len(branch) > 0 && len(target) > 0 && len(build) > 0 {
		// If provided, use these to set branch, target and build. this will make internal
		// TF ants plugin to work correctly with ATP created invocation.
		cmd = append(cmd, "--branch", branch, "--build-flavor", target,
			"--build-id", build)
		invocationInfoReported = true
		logger.Println("Setting build info from execution metadata - branch/target/build: ", branch, "/", target, "/", build)
	}

	for _, t := range tests {
		testName := formatTestName(t.GetTestCase().GetId().GetValue())
		ctsBranch, ctsTarget, ctsBuild := extractBuildInfo(t.TestCase, metadata, board, plan, logger)
		if len(ctsBranch) > 0 && len(ctsTarget) > 0 && len(ctsBuild) > 0 && !buildInfoReported {
			if !invocationInfoReported {
				// if no build info is updated yet, then use test metadata build info to update them.
				cmd = append(cmd, "--branch", ctsBranch, "--build-flavor", ctsTarget,
					"--build-id", ctsBuild)
				invocationInfoReported = true
				branch = ctsBranch
				target = ctsTarget
				build = ctsBuild
			}

			if !isAospTradefed() {
				cmd = append(cmd, "--cts-branch", ctsBranch, "--cts-build-flavor", ctsTarget,
					"--cts-build-id", ctsBuild)
			}
			buildInfoReported = true
		}
		logger.Println("Running provided test: ", testName)

		if isAospTradefed() {
			cmd = append(cmd, "--include-filter", testName)
		} else {
			// Format the name in a quote, incase there is a space in it (ie a class execution)
			cmd = append(cmd, "--cts-params", "--compatibility:include-filter", "--cts-params", testName)
		}
	}

	// Iterate though all values of all "cts-params" args.
	values, ok := args["cts-params"]
	if ok {
		for _, value := range values {
			for _, val := range strings.Split(value, ",") {
				cmd = append(cmd, "--cts-params", val)
			}
		}
	}

	// Add devices
	for _, s := range serials {
		cmd = append(cmd, "-s", s)
	}

	if isAospTradefed() {
		buildId, err := strconv.Atoi(build)
		if err != nil {
			logger.Println("Unable to parse build id: ", build)
		}

		mountPoint, err := common.FetchXtsSuite(testType, branch, target, buildId)
		if err != nil {
			logger.Printf("Unable to fetch tests for suite: %s, branch: %s, target: %s, build: %s, %s\n", testType, branch, target, build, err)
			return nil
		} else {
			logger.Printf("Fetched tests for suite: %s, to mount point: %s\n", testType, mountPoint)
		}
	}

	if servo != nil && servo.ServodAddress != nil && servo.ServodAddress.Address != "" && servo.ServodAddress.Port != 0 {
		cmd = append(cmd,
			"--invocation-data", fmt.Sprintf("servo.host=%s", servo.ServodAddress.Address),
			"--invocation-data", fmt.Sprintf("servo.port=%d", servo.ServodAddress.Port),
		)
	}

	return cmd
}
