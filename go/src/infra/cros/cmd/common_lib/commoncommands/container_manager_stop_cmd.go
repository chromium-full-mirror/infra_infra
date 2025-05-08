// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	ctpv2_data "go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// ContainerManagerStopCmd represents gcloud auth cmd.
type ContainerManagerStopCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	RequestChannel chan commontypes.ContainerManagementRequest
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ContainerManagerStopCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *ctpv2_data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ContainerManagerStopCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *ctpv2_data.FilterStateKeeper) error {

	cmd.RequestChannel = sk.ContainerRequestChannel

	return nil
}

func NewContainerManagerStopCmd(executor interfaces.ExecutorInterface) *ContainerManagerStopCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ContainerManagerStopCmdType, executor)
	cmd := &ContainerManagerStopCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
