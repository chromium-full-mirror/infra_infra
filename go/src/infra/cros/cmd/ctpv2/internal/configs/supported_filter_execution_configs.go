// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package configs contains the definitions and storage of command configs.
package configs

import (
	"context"

	"infra/cros/cmd/common_lib/common_commands"
	"infra/cros/cmd/common_lib/common_configs"
	"infra/cros/cmd/common_lib/common_executors"
	"infra/cros/cmd/ctpv2/internal/commands"
	"infra/cros/cmd/ctpv2/internal/executors"
)

// All currently supported command-executor pairs.

var TranslateV1toV2RequestNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.TranslateV1toV2RequestType, ExecutorType: common_executors.NoExecutorType}
var TranslateRequestNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.TranslateRequestType, ExecutorType: common_executors.NoExecutorType}
var PrepareFilterContainersNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.PrepareFilterContainersCmdType, ExecutorType: common_executors.NoExecutorType}
var ExecuteFilterFilterExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.FilterExecutionCmdType, ExecutorType: executors.FilterExecutorType}
var SummarizeNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.SummarizeCmdType, ExecutorType: common_executors.NoExecutorType}

var CtrStartAsyncCtrExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.CtrServiceStartAsyncCmdType, ExecutorType: common_executors.CtrExecutorType}
var CtrStopCtrExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.CtrServiceStopCmdType, ExecutorType: common_executors.CtrExecutorType}
var GcloudAuthCtrExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.GcloudAuthCmdType, ExecutorType: common_executors.CtrExecutorType}
var ContainerStartContainerExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.ContainerStartCmdType, ExecutorType: common_executors.ContainerExecutorType}
var ContainerReadLogsContainerExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.ContainerReadLogsCmdType, ExecutorType: common_executors.ContainerExecutorType}
var ContainerCloseLogsContainerExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.ContainerCloseLogsCmdType, ExecutorType: common_executors.ContainerExecutorType}

var MiddleOutNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.MiddleoutExecutionType, ExecutorType: common_executors.NoExecutorType}
var GenerateTrv2ReqsNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.GenerateTrv2RequestsCmdType, ExecutorType: common_executors.NoExecutorType}
var ScheduleTasksNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.ScheduleTasksCmdType, ExecutorType: common_executors.NoExecutorType}
var AlStatusUpdateNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.AlStatusUpdateCmdType, ExecutorType: common_executors.NoExecutorType}
var AlStatusCleanUpNoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.AlStatusCleanUpCmdType, ExecutorType: common_executors.NoExecutorType}

// GenerateFilterConfigs generates cmd execution for ctpv2.
func GenerateFilterConfigs(ctx context.Context, totalFilters int) *common_configs.Configs {
	mainConfigs := []*common_configs.CommandExecutorPairedConfig{}

	// Update AL first for AL runs
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)
	// Translate request
	mainConfigs = append(mainConfigs,
		TranslateRequestNoExecutor)

	mainConfigs = append(mainConfigs,
		PrepareFilterContainersNoExecutor)

	mainConfigs = append(mainConfigs,
		ContainerReadLogsContainerExecutor)

	for i := 0; i < totalFilters; i++ {
		mainConfigs = append(mainConfigs,
			ContainerStartContainerExecutor,
			ExecuteFilterFilterExecutor,
		)
	}

	mainConfigs = append(mainConfigs,
		ContainerCloseLogsContainerExecutor.WithRequired(true))

	// Middleout
	mainConfigs = append(mainConfigs, MiddleOutNoExecutor)

	// Schedule tasks
	mainConfigs = append(mainConfigs, GenerateTrv2ReqsNoExecutor)
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)
	mainConfigs = append(mainConfigs, ScheduleTasksNoExecutor)
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)

	cleanUpCommands := []*common_configs.CommandExecutorPairedConfig{
		AlStatusCleanUpNoExecutor,
	}

	return &common_configs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanUpCommands}
}

// GeneratePreConfigs generates pre cmd execution for ctpv2.
func GeneratePreConfigs(ctx context.Context) *common_configs.Configs {
	mainConfigs := []*common_configs.CommandExecutorPairedConfig{}
	cleanupConfigs := []*common_configs.CommandExecutorPairedConfig{}

	// Translate v1 to v2, Start CTR and do GcloudAuth
	mainConfigs = append(mainConfigs,
		TranslateV1toV2RequestNoExecutor,
		CtrStartAsyncCtrExecutor,
		GcloudAuthCtrExecutor)

	// Cleanup configs
	cleanupConfigs = append(cleanupConfigs,
		CtrStopCtrExecutor)

	return &common_configs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanupConfigs}
}

// GeneratePostConfigs generates post cmd execution for ctpv2.
func GeneratePostConfigs(ctx context.Context) *common_configs.Configs {
	mainConfigs := []*common_configs.CommandExecutorPairedConfig{}

	// Stop Ctr
	mainConfigs = append(mainConfigs,
		SummarizeNoExecutor,
		CtrStopCtrExecutor.WithRequired(true))

	return &common_configs.Configs{MainConfigs: mainConfigs, CleanupConfigs: []*common_configs.CommandExecutorPairedConfig{}}
}
