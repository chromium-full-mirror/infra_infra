// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// TkoPublishServiceStartCmd represents tko publish service cmd.
type TkoPublishServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	TkoPublishSrcDir string
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *TkoPublishServiceStartCmd) ExtractDependencies(
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

func (cmd *TkoPublishServiceStartCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if sk.TkoPublishSrcDir == "" {
		return fmt.Errorf("Cmd %q missing dependency: TkoPublishSrcDir", cmd.GetCommandType())
	}

	cmd.TkoPublishSrcDir = sk.TkoPublishSrcDir

	return nil
}

func NewTkoPublishServiceStartCmd(executor interfaces.ExecutorInterface) *TkoPublishServiceStartCmd {
	singleCmdExec := interfaces.NewSingleCmdByExecutor(TkoPublishStartCmdType, executor)
	cmd := &TkoPublishServiceStartCmd{SingleCmdByExecutor: singleCmdExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
