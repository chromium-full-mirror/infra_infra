// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// DutServiceStartCmd represents dut service start cmd.
type CacheServerStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// Updates
	CacheServerAddress *labapi.IpEndpoint
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *CacheServerStartCmd) UpdateStateKeeper(
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

func (cmd *CacheServerStartCmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if cmd.CacheServerAddress != nil {
		sk.CacheServerAddress = cmd.CacheServerAddress
	}

	return nil
}

func NewCacheServerStartCmd(executor interfaces.ExecutorInterface) *CacheServerStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(CacheServerStartCmdType, executor)
	cmd := &CacheServerStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
