// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// RdbPublishServiceStartCmd represents rdb publish service start cmd.
type RdbPublishServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	RdbPublishSrcDir string
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *RdbPublishServiceStartCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
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

func (cmd *RdbPublishServiceStartCmd) extractDepsFromHwTestStateKeeper(ctx context.Context, sk *data.HwTestStateKeeper) error {
	if sk.RdbPublishSrcDir == "" {
		logging.Warningf(ctx, "Warning: Cmd %q missing dependency: RdbPublishSrcDir", cmd.GetCommandType())
	}

	cmd.RdbPublishSrcDir = sk.RdbPublishSrcDir
	return nil
}

func NewRdbPublishServiceStartCmd(executor interfaces.ExecutorInterface) *RdbPublishServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(RdbPublishStartCmdType, executor)
	cmd := &RdbPublishServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
