// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// UpdateDutStateCmd represents update dut state command.
type TkoDirectUploadCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	TkoPublishSrcDir string
	TestResponses    *testapi.CrosTestResponse
	CftTestRequest   *skylab_test_runner.CFTTestRequest
	TkoJobName       string // Optional but depends on env var
	GcsURL           string // Optional
	TesthausURL      string // Optional
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *TkoDirectUploadCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// Execute executes the command.
func (cmd *TkoDirectUploadCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Upload to TKO")
	defer func() { step.End(err) }()

	allValid := true
	for _, testResult := range cmd.TestResponses.GetTestCaseResults() {
		err = cmd.uploadTestResultToTKO(ctx, testResult)
		if err != nil {
			logging.Infof(ctx, "error while uploading to tko: %s", err)
			allValid = false
		}
	}

	if !allValid {
		err = fmt.Errorf("Failure while uploading some test results")
	}

	return err
}

// uploadTestResultToTKO uploads each test result to tko.
func (cmd *TkoDirectUploadCmd) uploadTestResultToTKO(ctx context.Context, testResult *testapi.TestCaseResult) error {
	var err error
	step, ctx := build.StartStep(ctx, testResult.GetTestCaseId().GetValue())
	defer func() { step.End(err) }()

	// Only supports tauto test results upload
	switch testResult.GetTestHarness().GetTestHarnessType().(type) {
	case *testapi.TestHarness_Tauto_:
		// Continue tko uploading for tauto
	default:
		step.SetSummaryMarkdown(fmt.Sprintf("TKO upload not supported for harness type %s", testResult.GetTestHarness()))
		return nil
	}

	// Log for debugging
	logging.Infof(ctx, "test harness type: %s", testResult.GetTestHarness())
	logging.Infof(ctx, "tko publish src dir: %s", cmd.TkoPublishSrcDir)
	logging.Infof(ctx, "tko job name: %s", cmd.TkoJobName)

	// absolute results dir path
	resultsDirPath := strings.Replace(testResult.GetResultDirPath().GetPath(), "/tmp/test", cmd.TkoPublishSrcDir, 1)
	logging.Infof(ctx, "results dir path: %s", resultsDirPath)

	// Write to Keyvals
	keyvalFilePath := path.Join(resultsDirPath, "keyval")
	logging.Infof(ctx, "keyval file path: %s", resultsDirPath)

	kvLog1 := step.Log("Keyval file contents before writing")
	kvFileContentsMap, err := common.GetFileContentsInMap(ctx, keyvalFilePath, "=", kvLog1)
	if err != nil {
		return errors.Annotate(err, "error while getting keyval file contents: ").Err()
	}

	kvList := []string{}
	// Get all existing values
	for k, v := range kvFileContentsMap {
		kvList = append(kvList, fmt.Sprintf("%s=%s", k, v))
	}

	// Add all the input values
	for k, v := range cmd.CftTestRequest.GetAutotestKeyvals() {
		kvList = append(kvList, fmt.Sprintf("%s=%s", k, v))
	}

	// Add custom values
	if cmd.GcsURL != "" {
		kvList = append(kvList, fmt.Sprintf("%s=%s", "synchronous_log_data_url", cmd.GcsURL))
	}
	if cmd.TesthausURL != "" {
		kvList = append(kvList, fmt.Sprintf("%s=%s", "synchronous_log_data_testhaus_url", cmd.TesthausURL))
	}
	if testResult.GetStartTime() != nil && testResult.GetDuration() != nil {
		endTime := testResult.GetStartTime().AsTime().Add((testResult.GetDuration().AsDuration())).Unix()
		kvList = append(kvList, fmt.Sprintf("%s=%s", "job_finished", strconv.FormatInt(endTime, 10)))
	}

	finalKvContents := strings.Join(kvList, "\n")
	err = common.WriteToExistingFile(ctx, keyvalFilePath, finalKvContents)
	if err != nil {
		return errors.Annotate(err, "error while writing keyval file contents: ").Err()
	}

	kvLog2 := step.Log("Keyval file contents after writing")
	_, err = kvLog2.Write([]byte(finalKvContents))
	if err != nil {
		logging.Infof(ctx, "error while writing keyval contents: %s", err)
	}

	// For VMLab test, preserve keyval appending that is required by CTS archiver,
	// but skip current workaround of TKO publish: it won't work as no script
	// installed on bot and TKO is scheduled to be deprecated in Q2 2023.
	if common.GetBotProvider() == common.BotProviderGce {
		logging.Infof(ctx, "skip TKO upload for VMLab: no script installed")
		return nil
	}

	parseCmd, err := tkoParseCmd(ctx, resultsDirPath, cmd.TkoJobName)
	if err != nil {
		return errors.Annotate(err, "error while getting tko-parse cmd: ").Err()
	}

	tkoParseLog := step.Log("Tko-Parse log")
	err = common.RunCommandWithCustomWriter(ctx, parseCmd, "tko-parse", tkoParseLog)
	if err != nil {
		return errors.Annotate(err, "error while executing tko-parse cmd: ").Err()
	}

	return nil
}

// tkoParseCmd constructs tko-parse command with all necessary args
func tkoParseCmd(ctx context.Context, resultsDirPath string, jobName string) (*exec.Cmd, error) {
	if strings.TrimSpace(resultsDirPath) == "" {
		return nil, fmt.Errorf("ResultsDirPath is empty")
	}
	if strings.TrimSpace(jobName) == "" {
		return nil, fmt.Errorf("JobName is empty")
	}
	args := []string{
		"--write-pidfile",
		resultsDirPath,
		"--effective_job_name", jobName,
		"-l", "3",
		"--record-duration", "-r", "-o", "--suite-report",
	}
	cmd := exec.CommandContext(ctx, common.TkoParseScriptPath, args...)
	return cmd, nil
}

func (cmd *TkoDirectUploadCmd) extractDepsFromHwTestStateKeeper(ctx context.Context, sk *data.HwTestStateKeeper) error {
	if sk.GcsURL == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: GcsURL", cmd.GetCommandType())
	}
	if sk.TesthausURL == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: TesthausURL", cmd.GetCommandType())
	}
	if sk.TestResponses == nil {
		return fmt.Errorf("Cmd %q missing dependency: HostName", cmd.GetCommandType())
	}
	if sk.CftTestRequest == nil {
		return fmt.Errorf("Cmd %q missing dependency: CftTestRequest", cmd.GetCommandType())
	}
	if sk.TkoPublishSrcDir == "" {
		return fmt.Errorf("Cmd %q missing dependency: TkoPublishSrcDir", cmd.GetCommandType())
	}

	swarmingTaksId := os.Getenv("SWARMING_TASK_ID")
	if swarmingTaksId == "" {
		return fmt.Errorf("Cmd %q missing dependency: SWARMING_TASK_ID in env to construct TkoJobName", cmd.GetCommandType())
	}
	// # A swarming task may have multiple attempts ("runs").
	// # The swarming task ID always ends in "0", e.g. "123456789abcdef0".
	// # The corresponding runs will have IDs ending in "1", "2", etc., e.g. "123456789abcdef1".
	// # All attempts should be recorded under same job ending with 0.
	formattedSwarmingTaskId := swarmingTaksId[:len(swarmingTaksId)-1]
	cmd.TkoJobName = fmt.Sprintf("swarming-%s0", formattedSwarmingTaskId)

	cmd.TestResponses = sk.TestResponses
	cmd.CftTestRequest = sk.CftTestRequest
	cmd.TkoPublishSrcDir = sk.TkoPublishSrcDir

	return nil
}

func NewTkoDirectUploadCmd() *TkoDirectUploadCmd {
	abstractCmd := interfaces.NewAbstractCmd(TkoDirectUploadCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &TkoDirectUploadCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
