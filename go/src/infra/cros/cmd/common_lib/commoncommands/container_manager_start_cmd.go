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

// ContainerManagerStartCmd represents gcloud auth cmd.
type ContainerManagerStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	RequestChannel chan commontypes.ContainerManagementRequest
	// Number of producers that will be generating containger management requests
	RequestProducerCount int
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ContainerManagerStartCmd) ExtractDependencies(ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *ctpv2_data.PrePostFilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ContainerManagerStartCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *ctpv2_data.PrePostFilterStateKeeper) error {

	cmd.RequestChannel = sk.ContainerRequestChannel

	if sk.CtpV2Request != nil {
		cmd.RequestProducerCount = len(sk.CtpV2Request.GetRequests())
	} else {
		cmd.RequestProducerCount = len(sk.V1KeyToCTPv2Req)
	}

	return nil
}

func NewContainerManagerStartCmd(executor interfaces.ExecutorInterface) *ContainerManagerStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ContainerManagerStartCmdType, executor)
	cmd := &ContainerManagerStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
