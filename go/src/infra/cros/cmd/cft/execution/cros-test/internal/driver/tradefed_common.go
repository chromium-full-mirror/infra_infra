// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
)

const (
	invocationTimeout    = "18h"
	maxRuntime           = "18h"
	retryFailedRuns      = true
	resultsUpload        = true
	defaultGlobalLogPath = "/tmp"
	GitMainALDev         = "git_main-al-dev"
	GitMain              = "git_main"
	tfprefix             = "tradefed."
)

var skipDownloadArtifacts = []string{"TESTS_ZIP", "DEVICE_IMAGE", "BASEBAND"}

// TODO: find better way to discover or pass in board's architecture.
var armBoards = []string{"corsola"}

func getArchFromBoard(targetBoard string) string {
	arch := "x86"
	for _, board := range armBoards {
		if strings.Contains(strings.ToLower(targetBoard), board) {
			arch = "arm"
		}
	}
	return arch
}

func getGlobalLogPath() string {
	logPath := os.Getenv("GLOBAL_LOG_PATH")
	if len(logPath) <= 0 {
		logPath = defaultGlobalLogPath
		os.Setenv("GLOBAL_LOG_PATH", logPath)
	}
	return logPath
}

func extractBuildInfoFromExecutionMetadata(metadata *api.ExecutionMetadata) (branch string, target string, build string) {
	if branchFlag, err := extractMetadataFlag(metadata, "branch"); err == nil {
		branch = branchFlag
	}
	if targetFlag, err := extractMetadataFlag(metadata, "build_flavor"); err == nil {
		target = targetFlag
	}
	if buildIDFlag, err := extractMetadataFlag(metadata, "build_id"); err == nil {
		build = buildIDFlag
	}

	return
}

func extractTestInfoFromExecutionMetadata(metadata *api.ExecutionMetadata) (branch string, target string, build string) {
	if branchFlag, err := extractMetadataFlag(metadata, "extra_branch"); err == nil {
		branch = branchFlag
	}
	if targetFlag, err := extractMetadataFlag(metadata, "extra_build_flavor"); err == nil {
		target = targetFlag
	}
	if buildIDFlag, err := extractMetadataFlag(metadata, "extra_build"); err == nil {
		build = buildIDFlag
	}

	return
}

func extractBuildInfoFromTest(test *api.TestCase, metadata *api.ExecutionMetadata, board string) (branch string, target string, build string) {
	for _, t := range test.Tags {
		if strings.HasPrefix(t.Value, "target_build=") {
			tags := strings.Split(strings.TrimPrefix(t.Value, "target_build="), ",")
			var targetCandidate, buildCandidate, abi string
			for _, tag := range tags {
				if strings.HasPrefix(tag, "target:") {
					targetCandidate = strings.TrimPrefix(tag, "target:")
				}
				if strings.HasPrefix(tag, "build_id:") {
					buildCandidate = strings.TrimPrefix(tag, "build_id:")
				}
				if strings.HasPrefix(tag, "abi:") {
					abi = strings.TrimPrefix(tag, "abi:")
				}
			}
			if abi != "" && strings.Contains(abi, getArchFromBoard(board)) {
				build = buildCandidate
				target = targetCandidate
			}
		}
		if strings.HasPrefix(t.Value, "branch:") {
			branch = strings.TrimPrefix(t.Value, "branch:")
		}
		if strings.HasPrefix(t.Value, "target:") && strings.Contains(t.Value, getArchFromBoard(board)) && target == "" {
			target = strings.TrimPrefix(t.Value, "target:")
		}
		if strings.HasPrefix(t.Value, "build_id:") && build == "" {
			if getArchFromBoard(board) == "arm" && branch == "git_main" {
				build = "12681648"
			} else {
				build = strings.TrimPrefix(t.Value, "build_id:")
			}
		}
	}
	return
}

func formatTestName(fqTestName string) string {
	testName := fqTestName
	testName = strings.TrimPrefix(testName, tfprefix)

	// Trim test suite prefix for all known suites.
	for _, suite := range knownSuites {
		testName = strings.TrimPrefix(testName, suite+".")
	}
	return fmt.Sprintf("\"%s\"", testName)
}

func extractMetadataFlag(metadata *api.ExecutionMetadata, flagName string) (string, error) {
	if metadata != nil && len(metadata.Args) > 0 {
		for _, arg := range metadata.Args {
			if arg.Flag == flagName {
				return arg.Value, nil
			}
		}
	}
	return "", fmt.Errorf("flag %s for found in test metadata", flagName)
}

func buildResultReportingArgs(logger *log.Logger, metadata *api.ExecutionMetadata, args map[string]string, board, model string) []string {
	cmd := []string{}

	uploadToAnts := true
	if skipAntsUploadFlag, err := extractMetadataFlag(metadata, "skip_ants_upload"); err == nil {
		logger.Printf("Skip upload to AnTS: %s", skipAntsUploadFlag)
		if b, err := strconv.ParseBool(skipAntsUploadFlag); err == nil && b {
			uploadToAnts = false
		}
	}

	if resultsUpload && uploadToAnts {
		cmd = append(cmd, "--ants-result-reporter:create-local-invocation",
			"--ants-result-reporter:no-enable-sponge",
			"--ants-result-reporter:save-metric-file")

		// Extract AnTS invocation id and WorkUnit ID from execution metadata.
		if AnTsInvID, err := extractMetadataFlag(metadata, "ants_invocation_id"); err == nil { // nolint:stylecheck // ST1003
			cmd = append(cmd, "--invocation-data", fmt.Sprintf("invocation_id=%s", AnTsInvID))
			logger.Printf("Got invocation id: %s", AnTsInvID)
		}
		if AnTsWorkUnitID, err := extractMetadataFlag(metadata, "ants_work_unit_id"); err == nil { // nolint:stylecheck // ST1003
			cmd = append(cmd, "--invocation-data", fmt.Sprintf("work_unit_id=%s", AnTsWorkUnitID))
			logger.Printf("Got work unit id: %s", AnTsWorkUnitID)
		}
		if atpName, err := extractMetadataFlag(metadata, "atp_test_name"); err == nil {
			cmd = append(cmd, "--invocation-data", fmt.Sprintf("atp_test_name=%s", atpName))
			logger.Printf("Got ATP test name: %s", atpName)
		}
		value, ok := args["crystalball_ingest"]
		if ok {
			if strings.ToLower(value) == "true" {
				cmd = append(cmd, "--invocation-data", "invocation-property=crystalball_ingest:yes")
				cmd = append(cmd, "--invocation-data", fmt.Sprintf("invocation-property=run_target:%s_%s", board, model))
			}
		}

		// Also allow the flag to be passed in directly through the args.
		for k, v := range args {
			if k == "invocation-data" {
				cmd = append(cmd, fmt.Sprintf("--%s", k), v)
				cmd = append(cmd, "--invocation-data", fmt.Sprintf("invocation-property=run_target:%s_%s", board, model))

			}
		}
		extraBranch, _ := extractMetadataFlag(metadata, "extra_branch")
		extraTarget, _ := extractMetadataFlag(metadata, "extra_target")
		extraBuild, _ := extractMetadataFlag(metadata, "extra_build")
		if extraBranch != "" && extraTarget != "" && extraBuild != "" {
			extraBuilds := fmt.Sprintf(`extra_builds=[{"build_id": "%s",`+
				`"build_target": "%s", "build_flavor": "%s", "build_platform": "linux",`+
				`"build_type": null, "branch": "%s", "build_alias": null, "attempt_id":`+
				`"latest", "build_os": "linux"}]`, extraBuild, extraTarget,
				extraTarget, extraBranch)
			cmd = append(cmd, "--invocation-data", extraBuilds)
		}
	}

	if envName, err := extractMetadataFlag(metadata, "android-build-environment"); err == nil {
		cmd = append(cmd, "--invocation-data", fmt.Sprintf("android-build-environment=%s", envName))
		logger.Printf("Got Android build environment: %s", envName)
	} else {
		// Use production build env as default, if not specified otherwise.
		cmd = append(cmd, "--invocation-data", "android-build-environment=prod")
	}

	return cmd
}

func extractTestCaseName(testName string) string {
	testCaseName := testName
	if strings.HasPrefix(testCaseName, tfprefix) {
		testCaseName = strings.TrimPrefix(testCaseName, tfprefix)
		// Also trimming the suite name.
		testCaseName = strings.TrimLeft(testCaseName, ".")
	}
	return testCaseName
}
