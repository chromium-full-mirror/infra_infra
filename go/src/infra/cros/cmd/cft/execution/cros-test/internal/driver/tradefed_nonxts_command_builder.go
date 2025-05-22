// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

const (
	nonXtsTestRunner = "google/template/lab-base"

	generalConfigOnlyZip    = "general-tests_configs.zip"
	generalTestsZipTemplate = "'.*general-tests.zip'"
)

var templateMap = []string{
	"lab-preparers=google/template/preparers/lab",
	"test=suite/base-suite",
	"reporters=google/template/reporters/lab-luci",
}

func BuildNonXtsTestCommand(logger *log.Logger, testType string, tests []*api.TestCaseMetadata,
	serials []string, metadata *api.ExecutionMetadata, board string, args map[string][]string, model string,
	servo *labapi.Servo) []string {

	cmd := []string{nonXtsTestRunner}

	logPath := getGlobalLogPath()

	cmd = append(cmd, "--test-tag", fmt.Sprintf("cros-%s-test", testType),
		"--auto-collect", "HOSTLOG_ON_FAILURE", "--auto-collect", "LOGCAT_ON_FAILURE",
		"--invocation-timeout", invocationTimeout, "--log-level", "VERBOSE",
		"--invocation-data", "local-test-runner=atest", "--invocation-data", "use_artifact_v3_api=true",
		"--gdevice-flash:disable", "--no-intra-module-sharding", "--load-configs-with-include-filters",
		"--periodic-upload", "--primary-abi-only", "--android-build-api-log-saver:no-remove-staged-files",
		"--remote-cache-instance-name", "projects/android-test-rbe/instances/default_instance",
		"--use-sandbox", "--sandbox:sandbox-use-test-discovery", "--screenshot-on-failure",
		"--google-device-setup:run-command", "am switch-user 10", "--no-set-test-harness",
		"--no-skip-staging-artifacts", "--upload-cached-module-results",
		"--test-config-only-zip", generalConfigOnlyZip,
		"--test-zip-file-filter", generalTestsZipTemplate, "--no-bugreport-on-invocation-ended",
		"--google-device-setup:set-global-setting", "verifier_verify_adb_installs=0",
		"--luci-result-reporter:log-output-dir", logPath,
		"--enable-module-dynamic-download",
	)

	// Adding a list of artifacts to skip downloading by the CTS runner.
	for _, artifact := range skipDownloadArtifacts {
		cmd = append(cmd, "--skip-download", artifact)
	}

	for _, template := range templateMap {
		cmd = append(cmd, "--template:map", template)
	}

	cmd = append(cmd, buildResultReportingArgs(logger, metadata, args, board, model)...)

	if retryFailedRuns {
		cmd = append(cmd, "--max-testcase-run-count", "3", "--retry-isolation-grade", "FULLY_ISOLATED",
			"--retry-strategy", "RETRY_ANY_FAILURE", "--module-preparation-retry")
	} else {
		cmd = append(cmd, "--max-testcase-run-count", "1", "--retry-strategy", "NO_RETRY")
	}

	var buildInfoReported = false
	var invocationInfoReported = false
	var branch, target, build string
	// To support ATP runs, the priority is given to the "extra" metadata
	// which is pointing to the location of test and Tradefed ZIP packages.
	branch, target, build = extractTestInfoFromExecutionMetadata(metadata)
	if !(len(branch) > 0 && len(target) > 0 && len(build) > 0) {
		branch, target, build = extractBuildInfoFromExecutionMetadata(metadata)
	}
	if len(branch) > 0 && len(target) > 0 && len(build) > 0 {
		// If provided, use these to set branch, target and build. this will make internal
		// TF ants plugin to work correctly with ATP created invocation.
		cmd = append(cmd, "--branch", branch, "--build-flavor", target, "--build-id", build)
		invocationInfoReported = true
		logger.Println("Setting build info from execution metadata - branch/target/build: ", branch, "/", target, "/", build)
	}

	for _, t := range tests {
		if !buildInfoReported {
			tiBranch, tiTarget, tiBuild := extractBuildInfoFromTest(t.TestCase, metadata, board)
			if len(tiBranch) > 0 && len(tiTarget) > 0 && len(tiBuild) > 0 {
				logger.Println("Setting build info from test metadata - branch/target/build: ", board, "/", target, "/", build)
				if !invocationInfoReported {
					// if no build info is updated yet, then use test metadata build info to update them.
					cmd = append(cmd, "--branch", tiBranch, "--build-flavor", tiTarget,
						"--build-id", tiBuild)
					invocationInfoReported = true
				}
				buildInfoReported = true
			}
		}
		testName := formatTestName(t.GetTestCase().GetId().GetValue())
		logger.Println("Running provided test: ", testName)
		cmd = append(cmd, "--include-filter", testName)
	}

	// Add devices
	for _, s := range serials {
		cmd = append(cmd, "-s", s)
	}

	if servo != nil && servo.ServodAddress != nil && servo.ServodAddress.Address != "" && servo.ServodAddress.Port != 0 {
		cmd = append(cmd,
			"--invocation-data", fmt.Sprintf("servo.host=%s", servo.ServodAddress.Address),
			"--invocation-data", fmt.Sprintf("servo.port=%d", servo.ServodAddress.Port),
		)
	}

	return cmd
}
