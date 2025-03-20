// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sync"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	c "go.chromium.org/chromiumos/test/util/adb"

	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/common"
	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/device"
)

// MoblyDriver runs Mobly tests.
type MoblyDriver struct {
	logger *log.Logger
}

// NewMoblyDriver creates a new Mobly driver.
func NewMoblyDriver(logger *log.Logger) *MoblyDriver {
	return &MoblyDriver{logger: logger}
}

// Name returns the name of the driver.
func (md *MoblyDriver) Name() string {
	return "mobly"
}

// buildMoblyCommand constructs the Mobly execution command.
func buildMoblyCommand(logger *log.Logger, test *api.TestCaseMetadata) (*exec.Cmd, error) {
	parFile, ok := test.GetTestCaseInfo().GetExtraInfo()["executable_name"]
	if !ok {
		return nil, fmt.Errorf("missing 'executable_name' in test metadata")
	}

	par := filepath.Join("/usr/local/mobly", parFile)

	args := []string{
		"--", // Separator between global and test-specific arguments.
		"-c", "/usr/local/mobly/test_config.yml",
		"--tests", test.GetTestCase().GetName(),
	}

	// Check if the executable supports the ipv6 flag. if it does, we disable
	// enforcing kernel ipv6 support as our lab does not support this generally.
	// This is currently relevant for the g3 executable.
	// See b/390240237 for more details.
	helpCmd := exec.Command(par, "--helpfull")
	out, _ := helpCmd.CombinedOutput()
	flagRegex := regexp.MustCompile(`enforce_kernel_ipv6_support`)
	if flagRegex.MatchString(string(out)) {
		args = append([]string{"--enforce_kernel_ipv6_support=false"}, args...)
	}
	cmd := exec.Command(par, args...)
	return cmd, nil
}

// runMoblyTest executes a Mobly test.
func runMoblyTest(ctx context.Context, logger *log.Logger, test *api.TestCaseMetadata, serials []string, metadata []*api.Arg, devices []*labapi.Dut) error {
	config := NewMoblyConfig(logger, serials, metadata, devices)
	if err := config.Write(logger, "/usr/local/mobly"); err != nil {
		return fmt.Errorf("generating Mobly config: %w", err)
	}

	cmd, err := buildMoblyCommand(logger, test)
	if err != nil {
		return fmt.Errorf("building Mobly command: %w", err)
	}

	logger.Printf("Running Mobly: %s", cmd.String())
	return launchAndReadMobly(cmd, logger)
}

// processMoblyResults processes the results of a Mobly test.
func (md *MoblyDriver) processMoblyResults(resultsDir, testName string, resp *api.CrosTestResponse) error {
	defer func() {
		if r := recover(); r != nil {
			md.logger.Printf("Panic during Mobly result processing: %v\n%s", r, debug.Stack())
		}
	}()

	rd := filepath.Join(resultsDir, "LocalTestBed")
	resultsPath := filepath.Join(rd, "latest", "test_summary.yaml")

	moblyResultBytes, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("reading Mobly results: %w", err)
	}

	testCaseResults, err := common.TranslateMoblyResults(moblyResultBytes, rd)
	if err != nil {
		return fmt.Errorf("translating Mobly results: %w", err)
	}
	resp.TestCaseResults = append(resp.TestCaseResults, testCaseResults...)
	return nil

}

// RunTests executes Mobly tests.
func (md *MoblyDriver) RunTests(ctx context.Context, resultsDir string, req *api.CrosTestRequest, tlwAddr string, tests []*api.TestCaseMetadata) (*api.CrosTestResponse, error) {
	resp := &api.CrosTestResponse{}

	serials, err := device.DerviceSerials(req)
	if err != nil {
		return nil, fmt.Errorf("getting device serials: %w", err)
	}

	metadata, _, err := common.UnpackMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("unpacking metadata: %w", err)
	}
	metadata = append(metadata, common.ExtraArgs(req)...)

	if err := c.SetupAdbConnections(md.logger, serials); err != nil {
		return nil, fmt.Errorf("setting up ADB connections: %w", err)
	}
	defer func() {
		for _, s := range serials {
			if err := c.TeardownAdb(md.logger, s); err != nil {
				md.logger.Printf("Tearing down ADB connection to %s: %s", s, err)
			}
		}
	}()
	exit := make(chan struct{})
	defer close(exit)
	c.KeepAdbAlive(md.logger, serials, exit)

	for _, test := range tests {
		resDir := filepath.Join("/tmp", "test", "results", "mobly", test.GetTestCase().GetName())
		os.Setenv("MOBLY_LOGPATH", resDir)

		if err := runMoblyTest(ctx, md.logger, test, serials, metadata, device.Devices(req)); err != nil {
			return nil, fmt.Errorf("running Mobly test: %w", err)
		}

		if err := md.processMoblyResults(resDir, test.GetTestCase().GetName(), resp); err != nil {
			return nil, fmt.Errorf("processing Mobly Results: %w", err)
		}
	}

	return resp, nil
}

// launchAndReadMobly launches a Mobly command and reads its output.
func launchAndReadMobly(cmd *exec.Cmd, logger *log.Logger) error {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("getting stderr pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("getting stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting Mobly command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		common.TestScanner(stderr, logger, "Mobly")
	}()
	go func() {
		defer wg.Done()
		common.TestScanner(stdout, logger, "Mobly")
	}()
	wg.Wait()
	return nil
}
