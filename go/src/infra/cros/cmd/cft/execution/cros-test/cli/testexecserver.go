// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli implements the executionservice server
package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"
	statuserrors "go.chromium.org/chromiumos/test/execution/errors"
	"go.chromium.org/chromiumos/test/util/finder"

	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/driver"
)

// driverToTestsMapping builds a map between test and its driver.
func driverToTestsMapping(logger *log.Logger, mdList []*api.TestCaseMetadata) (map[driver.Driver][]*api.TestCaseMetadata, error) {
	tastDriver := driver.NewTastDriver(logger)
	tautoDriver := driver.NewTautoDriver(logger)
	gtestDriver := driver.NewGtestDriver(logger)
	crosierDriver := driver.NewCrosierDriver(logger)
	moblyDriver := driver.NewMoblyDriver(logger)
	tradefedDriver := driver.NewTradefedDriver(logger)

	driverToTests := make(map[driver.Driver][]*api.TestCaseMetadata)
	for _, md := range mdList {
		if md.TestCase == nil {
			return nil, statuserrors.NewStatusError(statuserrors.InvalidArgument,
				fmt.Errorf("missing test case information %v", md))
		}
		if md.TestCaseExec == nil || md.TestCaseExec.TestHarness == nil {
			return nil, statuserrors.NewStatusError(statuserrors.InvalidArgument,
				fmt.Errorf("test case %v does not have test harness information", md.TestCase.Name))
		}

		if md.TestCaseExec.TestHarness.GetTast() != nil {
			driverToTests[tastDriver] = append(driverToTests[tastDriver], md)
		} else if md.TestCaseExec.TestHarness.GetTauto() != nil {
			driverToTests[tautoDriver] = append(driverToTests[tautoDriver], md)
		} else if md.TestCaseExec.TestHarness.GetGtest() != nil {
			driverToTests[gtestDriver] = append(driverToTests[gtestDriver], md)
		} else if md.TestCaseExec.TestHarness.GetCrosier() != nil {
			driverToTests[crosierDriver] = append(driverToTests[crosierDriver], md)
		} else if md.TestCaseExec.TestHarness.GetMobly() != nil {
			driverToTests[moblyDriver] = append(driverToTests[moblyDriver], md)
		} else if md.TestCaseExec.TestHarness.GetTradefed() != nil {
			driverToTests[tradefedDriver] = append(driverToTests[tradefedDriver], md)
		} else {
			return nil, statuserrors.NewStatusError(statuserrors.InvalidArgument,
				errors.New("manual harness has not been supported"))
		}
	}
	return driverToTests, nil
}

// runTests runs the requested tests.
func runTests(ctx context.Context, logger *log.Logger, resultRootDir, tlwAddr string, metadataList *api.TestCaseMetadataList, req *api.CrosTestRequest) (*api.CrosTestResponse, error) {
	matchedMdList, err := finder.MatchedTestsForSuites(metadataList.Values, req.TestSuites)
	if err != nil {
		return nil, statuserrors.NewStatusError(statuserrors.InvalidArgument,
			fmt.Errorf("failed to match test metadata: %w", err))
	}

	// Write all matching test metadata to the results directory.
	const mdFilename = "test_metadata.json"
	if err := os.MkdirAll(resultRootDir, 0755); err != nil {
		return nil, statuserrors.NewStatusError(statuserrors.IOCreateError,
			fmt.Errorf("failed to create result root directory %v", resultRootDir))
	}
	defer func() {
		_ = exec.Command("chmod", "-R", "777", resultRootDir).Run()
	}()
	mdPath := filepath.Join(resultRootDir, mdFilename)
	if err := writeMetadata(mdPath, &api.TestCaseMetadataList{Values: matchedMdList}); err != nil {
		return nil, err
	}

	if out, err := exec.Command("gcloud", "auth", "list", "--format", "value(account)").CombinedOutput(); err != nil {
		logger.Println("Unable to run gcloud auth list: ", string(out))
	} else {
		logger.Println("The output of gcloud auth list: ", string(out))
	}

	driversToTests, err := driverToTestsMapping(logger, matchedMdList)
	if err != nil {
		return nil, err
	}
	allRspn := api.CrosTestResponse{}

	for driver, tests := range driversToTests {
		resultsDir := filepath.Join(resultRootDir, driver.Name())
		// Make sure the result directory exists.
		if err := os.MkdirAll(resultsDir, 0755); err != nil {
			return nil, statuserrors.NewStatusError(statuserrors.IOCreateError,
				fmt.Errorf("failed to create result directory %v", resultsDir))
		}

		rspn, err := driver.RunTests(ctx, resultsDir, req, tlwAddr, tests)
		if err != nil {
			return nil, err
		}
		allRspn.TestCaseResults = append(allRspn.TestCaseResults, rspn.TestCaseResults...)
		allRspn.GivenTestResults = append(allRspn.GivenTestResults, rspn.GivenTestResults...)
	}
	return &allRspn, nil
}

// readInput reads an execution_service json file and returns a pointer to RunTestsRequest.
func readInput(fileName string) (*api.CrosTestRequest, error) {
	f, err := os.ReadFile(fileName)
	if err != nil {
		return nil, statuserrors.NewStatusError(statuserrors.IOAccessError,
			fmt.Errorf("failed to read file %v: %w", fileName, err))
	}
	req := api.CrosTestRequest{}

	umrsh := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := umrsh.Unmarshal(f, &req); err != nil {
		return nil, statuserrors.NewStatusError(statuserrors.UnmarshalError,
			fmt.Errorf("failed to unmarshal file %v: %w", fileName, err))
	}
	return &req, nil
}

// writeMetadata writes a TestCaseMetadataList json.
func writeMetadata(outputPath string, mdList *api.TestCaseMetadataList) error {
	json, err := protojson.Marshal(mdList)
	if err != nil {
		return statuserrors.NewStatusError(statuserrors.MarshalError,
			fmt.Errorf("failed to marshall metadata list: %w", err))
	}
	if err := os.WriteFile(outputPath, json, 0666); err != nil {
		return statuserrors.NewStatusError(statuserrors.IOCreateError,
			fmt.Errorf("failed to write file %v: %w", outputPath, err))
	}
	return nil
}

// writeOutput writes a RunTestsResponse json.
func writeOutput(output string, resp *api.CrosTestResponse) error {
	json, err := protojson.Marshal(resp)
	if err != nil {
		return statuserrors.NewStatusError(statuserrors.MarshalError,
			fmt.Errorf("failed to marshall response: %w", err))
	}
	if err := os.WriteFile(output, json, 0666); err != nil {
		return statuserrors.NewStatusError(statuserrors.IOCreateError,
			fmt.Errorf("failed to write file %v: %w", output, err))
	}
	return nil
}
