// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// DutServiceStartCmd represents dut service start cmd.
type SshStartTunnelCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	HostName string

	// Updates
	SshTunnelPort uint16
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *SshStartTunnelCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *SshStartTunnelCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.LocalTestStateKeeper:
		err = cmd.updateLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *SshStartTunnelCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	cmd.HostName = sk.HostName

	return nil
}

func (cmd *SshStartTunnelCmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if cmd.SshTunnelPort != 0 {
		sk.DutSshAddress = &labapi.IpEndpoint{
			Address: "localhost",
			Port:    int32(cmd.SshTunnelPort),
		}
	}

	return nil
}

func NewSshStartTunnelCmd(executor interfaces.ExecutorInterface) *SshStartTunnelCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(SshStartTunnelCmdType, executor)
	cmd := &SshStartTunnelCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
