// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package configs contains the definitions and storage of command configs.
package configs

import (
	"context"

	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/commonconfigs"
	"go.chromium.org/infra/cros/cmd/common_lib/commonexecutors"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/commands"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/executors"
)

// All currently supported command-executor pairs.

var TranslateV1toV2RequestNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TranslateV1toV2RequestType, ExecutorType: commonexecutors.NoExecutorType}
var TranslateRequestNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TranslateRequestType, ExecutorType: commonexecutors.NoExecutorType}
var PrepareFilterContainersNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.PrepareFilterContainersCmdType, ExecutorType: commonexecutors.NoExecutorType}
var ExecuteFilterFilterExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.FilterExecutionCmdType, ExecutorType: executors.FilterExecutorType}
var SummarizeNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.SummarizeCmdType, ExecutorType: commonexecutors.NoExecutorType}

var CtrStartAsyncCtrExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.CtrServiceStartAsyncCmdType, ExecutorType: commonexecutors.CtrExecutorType}
var CtrStopCtrExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.CtrServiceStopCmdType, ExecutorType: commonexecutors.CtrExecutorType}
var GcloudAuthCtrExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.GcloudAuthCmdType, ExecutorType: commonexecutors.CtrExecutorType}
var ContainerManagerStartContainerManagerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerManagerStartCmdType, ExecutorType: commonexecutors.ContainerManagerExecutorType}
var ContainerManagerStopContainerManagerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerManagerStopCmdType, ExecutorType: commonexecutors.ContainerManagerExecutorType}
var ContainerReadLogsContainerManagerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerReadLogsCmdType, ExecutorType: commonexecutors.ContainerExecutorType}
var ContainerCloseLogsContainerManagerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerCloseLogsCmdType, ExecutorType: commonexecutors.ContainerExecutorType}

var MiddleOutNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.MiddleoutExecutionType, ExecutorType: commonexecutors.NoExecutorType}
var GenerateTrv2ReqsNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GenerateTrv2RequestsCmdType, ExecutorType: commonexecutors.NoExecutorType}
var ScheduleTasksNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ScheduleTasksCmdType, ExecutorType: commonexecutors.NoExecutorType}
var AlStatusUpdateNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.AlStatusUpdateCmdType, ExecutorType: commonexecutors.NoExecutorType}
var AlStatusCleanUpNoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.AlStatusCleanUpCmdType, ExecutorType: commonexecutors.NoExecutorType}

// GenerateFilterConfigs generates cmd execution for ctpv2.
func GenerateFilterConfigs(ctx context.Context, totalFilters int) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{}

	// Update AL first for AL runs
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)
	// Translate request
	mainConfigs = append(mainConfigs,
		TranslateRequestNoExecutor)

	mainConfigs = append(mainConfigs,
		PrepareFilterContainersNoExecutor)

	mainConfigs = append(mainConfigs,
		ContainerReadLogsContainerManagerExecutor.WithRequired(true))

	for range totalFilters {
		mainConfigs = append(mainConfigs,
			ExecuteFilterFilterExecutor,
		)
	}
	mainConfigs = append(mainConfigs,
		ContainerCloseLogsContainerManagerExecutor.WithRequired(true))

	// Done sending requests to the container manager
	mainConfigs = append(mainConfigs,
		ContainerManagerStopContainerManagerExecutor.WithRequired(true))

	// Middleout
	mainConfigs = append(mainConfigs, MiddleOutNoExecutor)

	// Schedule tasks
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)
	mainConfigs = append(mainConfigs, GenerateTrv2ReqsNoExecutor)
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)
	mainConfigs = append(mainConfigs, ScheduleTasksNoExecutor)
	mainConfigs = append(mainConfigs, AlStatusUpdateNoExecutor)

	cleanUpCommands := []*commonconfigs.CommandExecutorPairedConfig{
		AlStatusCleanUpNoExecutor,
	}

	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanUpCommands}
}

// GeneratePreConfigs generates pre cmd execution for ctpv2.
func GeneratePreConfigs(ctx context.Context) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{}
	cleanupConfigs := []*commonconfigs.CommandExecutorPairedConfig{}

	// Translate v1 to v2, Start CTR and do GcloudAuth
	mainConfigs = append(mainConfigs,
		TranslateV1toV2RequestNoExecutor,
		CtrStartAsyncCtrExecutor,
		GcloudAuthCtrExecutor,
		ContainerManagerStartContainerManagerExecutor)

	// Cleanup configs
	cleanupConfigs = append(cleanupConfigs,
		CtrStopCtrExecutor)

	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanupConfigs}
}

// GeneratePostConfigs generates post cmd execution for ctpv2.
func GeneratePostConfigs(ctx context.Context) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{}

	// Stop Ctr
	mainConfigs = append(mainConfigs,
		SummarizeNoExecutor,
		CtrStopCtrExecutor.WithRequired(true))

	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: []*commonconfigs.CommandExecutorPairedConfig{}}
}
