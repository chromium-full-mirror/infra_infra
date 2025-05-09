// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	ctpv2_data "go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// ContainerReadLogsCmd represents container close logs command.
type ContainerReadLogsCmd struct {
	*interfaces.SingleCmdByExecutor

	ContainerLogsChannel chan *commontypes.ContainerLogInfo
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ContainerReadLogsCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *ctpv2_data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)
	case *data.HwTestStateKeeper:
	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ContainerReadLogsCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *ctpv2_data.FilterStateKeeper) error {

	cmd.ContainerLogsChannel = sk.ContainerLogsChannel

	return nil
}

func NewContainerReadLogsCmd(executor interfaces.ExecutorInterface) *ContainerReadLogsCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ContainerReadLogsCmdType, executor)
	cmd := &ContainerReadLogsCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
