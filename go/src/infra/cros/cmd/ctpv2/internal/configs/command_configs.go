// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"fmt"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/commands"
)

// CommandConfig represents command config.
type CommandConfig struct {
	ExecutorConfig interfaces.ExecutorConfigInterface

	commandsMap map[interfaces.CommandType]interfaces.CommandInterface
}

func NewCommandConfig(execConfig interfaces.ExecutorConfigInterface) interfaces.CommandConfigInterface {
	cmdMap := make(map[interfaces.CommandType]interfaces.CommandInterface)
	return &CommandConfig{ExecutorConfig: execConfig, commandsMap: cmdMap}
}

// GetCommand returns the concrete command based on provided command and executor type.
func (cfg *CommandConfig) GetCommand(
	cmdType interfaces.CommandType,
	execType interfaces.ExecutorType) (interfaces.CommandInterface, error) {

	// Return cmd if already created.
	if savedCmd, ok := cfg.commandsMap[cmdType]; ok {
		return savedCmd, nil
	}

	var cmd interfaces.CommandInterface

	// Get cmd based on cmd type.
	switch cmdType {
	case commands.TranslateV1toV2RequestType:
		cmd = commands.NewTranslateV1toV2Cmd()

	case commands.TranslateRequestType:
		cmd = commands.NewTranslateRequestCmd()

	case commands.PrepareFilterContainersCmdType:
		cmd = commands.NewPrepareFilterContainersInfoCmd()

	case commands.MiddleoutExecutionType:
		cmd = commands.NewMiddleOutRequestCmd()

	case commands.GenerateTrv2RequestsCmdType:
		cmd = commands.NewGenerateTrv2RequestsCmd()

	case commands.ScheduleTasksCmdType:
		cmd = commands.NewScheduleTasksCmd()

	case commands.SummarizeCmdType:
		cmd = commands.NewSummarizeCmd()

	case commands.AlStatusUpdateCmdType:
		cmd = commands.NewAlStatusUpdateCmd()
	case commands.AlStatusCleanUpCmdType:
		cmd = commands.NewAlStatusCleanUpCmd()

	case commands.FilterExecutionCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commands.NewFilterExecutionCmd(exec)

	case commoncommands.CtrServiceStartAsyncCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewCtrServiceStartAsyncCmd(exec)

	case commoncommands.CtrServiceStopCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewCtrServiceStopCmd(exec)

	case commoncommands.GcloudAuthCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewGcloudAuthCmd(exec)
	case commoncommands.ContainerStartCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewContainerStartCmd(exec)

	case commoncommands.ContainerReadLogsCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewContainerReadLogsCmd(exec)

	case commoncommands.ContainerCloseLogsCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewContainerCloseLogsCmd(exec)

	case commoncommands.ContainerManagerStartCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewContainerManagerStartCmd(exec)

	case commoncommands.ContainerManagerStopCmdType:
		exec, err := cfg.ExecutorConfig.GetExecutor(execType)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting executor for command type %s: ", cmdType).Err()
		}
		cmd = commoncommands.NewContainerManagerStopCmd(exec)

	default:
		return nil, fmt.Errorf("command type %s not supported in command configs", cmdType)
	}

	cfg.commandsMap[cmdType] = cmd
	return cmd, nil
}
