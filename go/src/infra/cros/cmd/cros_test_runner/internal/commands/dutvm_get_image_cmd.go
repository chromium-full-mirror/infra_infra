// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
	vmlabapi "infra/libs/vmlab/api"
)

// DutVmGetImageCmd defines the step I/O of get the GCE image of Dut VM.
type DutVmGetImageCmd struct {
	*interfaces.SingleCmdByExecutor
	// Deps
	Build string

	// Updates
	DutVmGceImage *vmlabapi.GceImage
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *DutVmGetImageCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		buildName := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "build")
		if buildName == "" {
			return fmt.Errorf("cmd %q missing dependency: AutotestKeyvals['build']", cmd.GetCommandType())
		}
		cmd.Build = buildName
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}
	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *DutVmGetImageCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		sk.DutVmGceImage = cmd.DutVmGceImage
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}
	return nil
}

func NewDutVmGetImageCmd(executor interfaces.ExecutorInterface) *DutVmGetImageCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(DutVmGetImageCmdType, executor)
	cmd := &DutVmGetImageCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
