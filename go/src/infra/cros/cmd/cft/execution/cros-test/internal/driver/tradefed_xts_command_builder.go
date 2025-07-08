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
	planMetadataFlag          = "plan"
	tfGoogleTestRunner        = "google/cts/google-cts-launcher-for-aosp"
	tfAospTestRunner          = "tf-aosp-compatibility-config"
	DTS                       = "dts"
	defaultCSuitePackagesFile = "/google/data/ro/teams/app-compatibility/test-resources/package-lists/vic-top-100.txt"
)

func getTestRunner(testType string) string {
	if isAospTradefed() {
		return fmt.Sprintf("%s-%s", testType, tfAospTestRunner)
	} else {
		return tfGoogleTestRunner
	}
}

func extractCsuitePackageInfo(logger *log.Logger, metadata *api.ExecutionMetadata) []string {
	cmd := []string{}
	if packages, err := extractMetadataFlag(metadata, "packages"); err == nil {
		for _, pkg := range strings.Split(packages, ",") {
			cmd = append(cmd, "--package="+pkg)
		}
		logger.Println("Got packages: ", cmd)
		return cmd
	}
	if packageFile, err := extractMetadataFlag(metadata, "packages-file"); err == nil {
		logger.Printf("Got package-file: %s", packageFile)
		return []string{"--packages-file=" + packageFile}
	}
	logger.Println("No Csuite packages provided, using default: ", defaultCSuitePackagesFile)
	return []string{"--packages-file=" + defaultCSuitePackagesFile}
}

func BuildXtsTestCommand(logger *log.Logger, testType string, tests []*api.TestCaseMetadata,
	serials []string, metadata *api.ExecutionMetadata, board string, args map[string][]string, model string,
	servo *labapi.Servo, lsnexus *labapi.IpEndpoint) []string {

	cmd := []string{getTestRunner(testType)}

	plan, err := extractMetadataFlag(metadata, planMetadataFlag)
	if err != nil {
		// By default, plan is the same as test suite type, i.e. "cts", "dtd", etc (except for STS and CSuite).
		if testType == "sts" {
			plan = "sts-dynamic-full"
		} else if testType == "csuite" {
			plan = "csuite-app-launch"
		} else if testType == "apts" {
			plan = formatTestName(tests[0].GetTestCase().GetId().GetValue())
			if len(tests) > 1 {
				logger.Println("Only single test is supported when running APTS test suite, running test: ", plan)
			}
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
		if testType == "apts" {
			cmd = append(cmd, "--no-disable-compress", "--no-enable-root")
		}
	}

	logPath := getGlobalLogPath()

	cmd = append(cmd, "--invocation-timeout", invocationTimeout, "--log-level", "VERBOSE")

	if !isAospTradefed() {

		cmd = append(cmd, "--cts-version", "2",
			"--gdevice-flash:disable",
			"--android-build-api-log-saver:no-remove-staged-files", "--no-use-event-streaming",
			"--google-device-setup:set-global-setting", "verifier_verify_adb_installs=0",
			"--reporter-template", "template/reporters/subprocess-reporter",
			"--template:map", "reporters=template/reporters/xml-reporter.xml",
			"--metricsreporter:metrics-folder", logPath, "--max-run-time", maxRuntime,
			"--google-device-setup:run-command", "am switch-user 10",
		)

		if testType != "csuite" {
			cmd = append(cmd, "--cts-use-partial-download")
		}

		// Adding cts-params with prefix for each one.
		ctsParams := []string{
			"--log-level", "VERBOSE",
			"--max-log-size", "62914560", "--max-tmp-logcat-file", "62914560",
			"--logcat-on-failure", "--screenshot-on-failure",
		}
		if testType == "cts" || testType == "dts" {
			ctsParams = append(ctsParams, "--property-check:no-throw-error")
		}
		if testType == "sts" {
			ctsParams = append(ctsParams, "--ghidra-preparer:disable")
		}
		if testType == "csuite" {
			cmd = append(cmd, "--no-throw-if-extra-not-found", "-l", "VERBOSE")
			ctsParams = append(ctsParams, "--compatibility:enable-module-dynamic-download",
				"--dynamic-download-args com.android.csuite.config.AppRemoteFileResolver:uri-template=/google/data/ro/teams/app-compatibility/test-resources/apks/vic-top-apps/cutf64-240621/{package}",
				"--compatibility:test-arg=com.android.tradefed.testtype.HostTest:set-option:collect-app-version:true",
				"--compatibility:test-arg=com.android.tradefed.testtype.HostTest:set-option:screenshot-after-launch:true",
				"--compatibility:test-arg=com.android.tradefed.testtype.HostTest:set-option:app-launch-timeout-ms:20000",
				"--compatibility:test-arg=com.android.tradefed.testtype.HostTest:set-option:save-apk-when:ON_FAIL",
			)
			packages := extractCsuitePackageInfo(logger, metadata)
			ctsParams = append(ctsParams, packages...)
		}
		if testType != "apts" {
			ctsParams = append(ctsParams, "--no-use-device-build-info", "--include-test-log-tags",
				"--result-reporter:disable-result-posting", "--result-reporter:no-disable",
				"--post-boot-command", `"am switch-user 10"`, "--use-log-saver",
			)
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

	if extractRetryConfig(metadata) {
		cmd = append(cmd, "--max-testcase-run-count", "3", "--retry-isolation-grade", "FULLY_ISOLATED",
			"--retry-strategy", "RETRY_ANY_FAILURE", "--module-preparation-retry")
	} else {
		cmd = append(cmd, "--max-testcase-run-count", "1", "--retry-strategy", "NO_RETRY")
	}

	cmd = append(cmd, generateDriverArgsCmds(metadata)...)

	var buildInfoReported = false
	var invocationInfoReported = false
	var branch, target, build string
	if !isAospTradefed() {
		branch, target, build = extractBuildInfoFromExecutionMetadata(metadata)
		if len(branch) > 0 && len(target) > 0 && len(build) > 0 {
			// If provided, use these to set branch, target and build. this will make internal
			// TF ants plugin to work correctly with ATP created invocation.
			cmd = append(cmd, "--branch", branch, "--build-flavor", target,
				"--build-id", build)
			invocationInfoReported = true
			logger.Println("Setting build info from execution metadata - branch/target/build: ", branch, "/", target, "/", build)
		}
	}
	for _, t := range tests {
		testName := formatTestName(t.GetTestCase().GetId().GetValue())
		if !buildInfoReported {
			ctsBranch, ctsTarget, ctsBuild := extractTestInfoFromExecutionMetadata(metadata)
			if len(ctsBranch) > 0 && len(ctsTarget) > 0 && len(ctsBuild) > 0 {
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
					logger.Printf("Setting test info from execution metadata of test: %s, branch/target/build: %s/%s/%s",
						testName, ctsBranch, ctsTarget, ctsBuild)
					cmd = append(cmd, "--cts-branch", ctsBranch, "--cts-build-flavor", ctsTarget,
						"--cts-build-id", ctsBuild)
				}
			} else {
				logger.Println("Missing test info in execution metadata of test: ", testName)
			}
			buildInfoReported = true
		}
		logger.Println("Running provided test: ", testName)

		// For APTS and CSuite, test name is defined in "--config-name" or "--package" parameter.
		if testType != "apts" && testType != "csuite" {
			if isAospTradefed() {
				cmd = append(cmd, "--include-filter", testName)
			} else {
				// Format the name in a quote, in case there is a space in it (ie a class execution)
				cmd = append(cmd, "--cts-params", "--compatibility:include-filter", "--cts-params", testName)
			}
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

	if lsnexus != nil && servo != nil && servo.GetState() != labapi.PeripheralState_BROKEN {
		cmd = append(cmd,
			"--invocation-data", fmt.Sprintf("lsnexus_primary.host=%s", lsnexus.GetAddress()),
			"--invocation-data", fmt.Sprintf("lsnexus_primary.port=%d", lsnexus.GetPort()),
		)
	}

	return cmd
}
