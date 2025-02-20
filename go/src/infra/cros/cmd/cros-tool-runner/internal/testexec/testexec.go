// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testexec runs tests.
package testexec

import (
	"context"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/protobuf/jsonpb"

	config "go.chromium.org/chromiumos/config/go"
	build_api "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/common"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/libsserver"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/services"
)

// Run runs tests.
func Run(ctx context.Context, req *api.CrosToolRunnerTestRequest, crosTestContainer, crosDUTContainer *build_api.ContainerImageInfo, tokenFile string) (res *api.CrosToolRunnerTestResponse, err error) {
	// Use host network for dev environment which DUT address is in the form localhost:<port>
	const networkName = "host"

	if req.PrimaryDut == nil {
		return nil, errors.Reason("run test: primary DUT is not specified").Err()
	}

	artifactDir, err := filepath.Abs(req.ArtifactDir)
	if err != nil {
		return nil, errors.Annotate(err, "run test: failed to resolve artifact directory %v", req.ArtifactDir).Err()
	}
	// All non-test harness artifacts will be in <artifact_dir>/cros-test/cros-test.
	crosTestDir := path.Join(artifactDir, "cros-test", "cros-test")
	// All test result artifacts will be in <artifact_dir>/cros-test/artifact.
	resultDir := path.Join(artifactDir, "cros-test", "artifact")
	// The input file name.
	inputFileName := path.Join(crosTestDir, "request.json")
	// The directory for cros-dut artifacts.
	crosDUTDir := path.Join(artifactDir, "cros-dut")
	// The directory for artifacts from any test libraries run through cros-libs.
	crosLibsDir := path.Join(artifactDir, "cros-libs")

	// Setting up directories.
	if err := os.MkdirAll(crosTestDir, 0755); err != nil {
		return nil, errors.Annotate(err, "run test: failed to create directory %s", crosTestDir).Err()
	}
	log.Printf("Run test: created the cros-test directory %s", crosTestDir)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return nil, errors.Annotate(err, "run test: failed to create directory %s", resultDir).Err()
	}
	log.Printf("Run test: created the test artifact directory %s", resultDir)

	duts := []*lab_api.Dut{req.PrimaryDut.GetDut()}

	var companions []*api.CrosTestRequest_Device
	for _, c := range req.GetCompanionDuts() {
		companions = append(
			companions, &api.CrosTestRequest_Device{
				Dut: c.GetDut(),
			},
		)
		duts = append(duts, c.GetDut())
	}

	for _, dut := range duts {
		crosDUTDirForDut := path.Join(crosDUTDir, dut.Id.GetValue())
		if os.MkdirAll(crosDUTDirForDut, 0755) != nil {
			return nil, errors.Annotate(err, "run test: failed to create cros-dut directory %s", crosDUTDirForDut).Err()
		}
		log.Printf("Run test: created the cros-dut artifact directory %s", crosDUTDirForDut)
	}

	dutServices, err := services.CreateDutServicesForHostNetwork(ctx, crosDUTContainer, duts, crosDUTDir, tokenFile)
	if err != nil {
		return nil, errors.Annotate(err, "run test: failed to start DUT servers").Err()
	}
	defer func() {
		for _, d := range dutServices {
			d.Docker.Remove(ctx)
		}
	}()
	for i, c := range companions {
		c.DutServer = &lab_api.IpEndpoint{Address: "localhost", Port: dutServices[i+1].Port}
	}

	// Create and run LibsServer.
	libsServer, err := libsserver.New(log.Default(), crosLibsDir, tokenFile, req)
	if err != nil {
		return nil, errors.Annotate(err, "could not start libsserver").Err()
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		if err := libsServer.Serve(wg); err != nil {
			log.Printf("libsserver error: %v", err)
		}
	}()
	defer libsServer.Stop(ctx)

	wg.Wait()

	testReq := &api.CrosTestRequest{
		TestSuites: req.GetTestSuites(),
		Primary: &api.CrosTestRequest_Device{
			Dut:        req.PrimaryDut.GetDut(),
			DutServer:  &lab_api.IpEndpoint{Address: "localhost", Port: dutServices[0].Port},
			LibsServer: &lab_api.IpEndpoint{Address: "localhost", Port: libsServer.Port},
		},
		Companions: companions,
		Metadata:   req.GetMetadata(),
	}
	if err := writeTestInput(inputFileName, testReq); err != nil {
		return nil, errors.Annotate(err, "run test: failed to create input file %s", inputFileName).Err()
	}
	if err = services.RunTestCLI(ctx, crosTestContainer, networkName, inputFileName, crosTestDir, resultDir, tokenFile); err != nil {
		// Do not raise the err, as we want to still check for a results.json
		log.Printf("Get error while run cros-test: %v", err)
	}

	common.AddContentsToLog(services.InputFileName, crosTestDir, "Reading cros-test input file.")
	resultFileName := path.Join(crosTestDir, "result.json")
	if _, err := os.Stat(resultFileName); os.IsNotExist(err) {
		return nil, errors.Reason("run test: result not found").Err()
	}
	out, err := readTestOutput(resultFileName)
	if err != nil {
		return nil, errors.Annotate(err, "run test: failed to read test output").Err()
	}

	return prepareTestResponse(resultDir, out.TestCaseResults)
}

// writeTestInput writes a CrosTestRequest json.
func writeTestInput(file string, req *api.CrosTestRequest) error {
	f, err := os.Create(file)
	if err != nil {
		return errors.Annotate(err, "fail to create file %v", file).Err()
	}
	m := jsonpb.Marshaler{}
	if err := m.Marshal(f, req); err != nil {
		return errors.Annotate(err, "fail to marshal request to file %v", file).Err()
	}
	return nil
}

// readTestOutput reads output file generated by cros-test.
func readTestOutput(filePath string) (*api.CrosTestResponse, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Annotate(err, "read output").Err()
	}
	out := &api.CrosTestResponse{}

	umrsh := common.JsonPbUnmarshaler()
	err = umrsh.Unmarshal(r, out)
	return out, errors.Annotate(err, "read output").Err()
}

// prepareTestResponse prepares a response for test execution.
func prepareTestResponse(resultRootDir string, testCaseResults []*api.TestCaseResult) (res *api.CrosToolRunnerTestResponse, err error) {
	var results []*api.TestCaseResult
	for _, t := range testCaseResults {
		//Skip if ResultsDirPath is nil
		resultDir := ""
		if t.ResultDirPath == nil {
			log.Printf("Empty resultsDirPath in test case result: %s", t)
			// When there is no TC path, lets just use the root path to try to get any logs from the services.
			resultDir = filepath.Join(resultRootDir, services.CrosTestResultsDirInsideDocker)
			t.ResultDirPath = &config.StoragePath{}
		} else {
			// Create the full path to results in the test environment. For example:
			// 		t.ResultDirPath.Path = "/tmp/test/results/tauto"
			// 		services.CrosTestResultsDirInsideDocker = "/tmp/test/results"
			// 		resultRootDir = "home/chromeos-test/skylab_bots/c6-r6-r4-h3.393573271/w/ir/x/w/output_dir/cros-test/artifact"
			// Replace the `/tmp/test/results/` (from `t.ResultDirPath.Path`) with the `resultRootDir`, thus making the full resolved path to the logs for that test.
			resultDir = strings.Replace(t.GetResultDirPath().GetPath(), services.CrosTestResultsDirInsideDocker, resultRootDir, 1)
		}
		t.ResultDirPath.Path = resultDir
		results = append(results, t)
	}
	log.Printf("CTR: Cros-Test Results: %s", results)
	return &api.CrosToolRunnerTestResponse{
		TestCaseResults: results,
	}, nil
}
