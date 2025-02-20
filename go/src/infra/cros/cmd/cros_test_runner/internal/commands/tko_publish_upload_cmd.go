// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"os"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// TkoPublishUploadCmd represents tko publish upload cmd.
type TkoPublishUploadCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	TkoJobName string
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *TkoPublishUploadCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

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

func (cmd *TkoPublishUploadCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	swarmingTaksId := os.Getenv("SWARMING_TASK_ID")
	if swarmingTaksId == "" {
		return fmt.Errorf("Cmd %q missing dependency: SWARMING_TASK_ID in env to construct TkoJobName", cmd.GetCommandType())
	}
	// # A swarming task may have multiple attempts ("runs").
	// # The swarming task ID always ends in "0", e.g. "123456789abcdef0".
	// # The corresponding runs will have IDs ending in "1", "2", etc., e.g. "123456789abcdef1".
	// # All attempts should be recorded under same job ending with 0.
	formattedSwarmignTaskId := swarmingTaksId[:len(swarmingTaksId)-1]
	jobName := fmt.Sprintf("swarming-%s0", formattedSwarmignTaskId)
	cmd.TkoJobName = jobName

	return nil
}

func NewTkoPublishUploadCmd(executor interfaces.ExecutorInterface) *TkoPublishUploadCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(TkoPublishUploadCmdType, executor)
	cmd := &TkoPublishUploadCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
