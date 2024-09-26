// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
var TranslateV1toV2Request_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.TranslateV1toV2RequestType, ExecutorType: common_executors.NoExecutorType}
var TranslateRequest_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.TranslateRequestType, ExecutorType: common_executors.NoExecutorType}
var PrepareFilterContainers_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.PrepareFilterContainersCmdType, ExecutorType: common_executors.NoExecutorType}
var ExecuteFilter_FilterExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.FilterExecutionCmdType, ExecutorType: executors.FilterExecutorType}
var Summarize_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.SummarizeCmdType, ExecutorType: common_executors.NoExecutorType}

var CtrStartAsync_CtrExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.CtrServiceStartAsyncCmdType, ExecutorType: common_executors.CtrExecutorType}
var CtrStop_CtrExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.CtrServiceStopCmdType, ExecutorType: common_executors.CtrExecutorType}
var GcloudAuth_CtrExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.GcloudAuthCmdType, ExecutorType: common_executors.CtrExecutorType}
var ContainerStart_ContainerExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.ContainerStartCmdType, ExecutorType: common_executors.ContainerExecutorType}
var ContainerReadLogs_ContainerExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.ContainerReadLogsCmdType, ExecutorType: common_executors.ContainerExecutorType}
var ContainerCloseLogs_ContainerExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: common_commands.ContainerCloseLogsCmdType, ExecutorType: common_executors.ContainerExecutorType}

var MiddleOut_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.MiddleoutExecutionType, ExecutorType: common_executors.NoExecutorType}
var GenerateTrv2Reqs_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.GenerateTrv2RequestsCmdType, ExecutorType: common_executors.NoExecutorType}
var ScheduleTasks_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.ScheduleTasksCmdType, ExecutorType: common_executors.NoExecutorType}
var AlStatusUpdate_NoExecutor = &common_configs.CommandExecutorPairedConfig{CommandType: commands.AlStatusUpdateCmdType, ExecutorType: common_executors.NoExecutorType}

// GenerateFilterConfigs generates cmd execution for ctpv2.
func GenerateFilterConfigs(ctx context.Context, totalFilters int) *common_configs.Configs {
	mainConfigs := []*common_configs.CommandExecutorPairedConfig{}

	// Update AL first for AL runs
	mainConfigs = append(mainConfigs, AlStatusUpdate_NoExecutor)
	// Translate request
	mainConfigs = append(mainConfigs,
		TranslateRequest_NoExecutor)

	mainConfigs = append(mainConfigs,
		PrepareFilterContainers_NoExecutor)

	mainConfigs = append(mainConfigs,
		ContainerReadLogs_ContainerExecutor)

	for i := 0; i < totalFilters; i++ {
		mainConfigs = append(mainConfigs,
			ContainerStart_ContainerExecutor,
			ExecuteFilter_FilterExecutor,
		)
	}

	mainConfigs = append(mainConfigs,
		ContainerCloseLogs_ContainerExecutor.WithRequired(true))

	// Middleout
	mainConfigs = append(mainConfigs, MiddleOut_NoExecutor)

	// Schedule tasks
	mainConfigs = append(mainConfigs, GenerateTrv2Reqs_NoExecutor)
	mainConfigs = append(mainConfigs, AlStatusUpdate_NoExecutor)
	mainConfigs = append(mainConfigs, ScheduleTasks_NoExecutor)
	mainConfigs = append(mainConfigs, AlStatusUpdate_NoExecutor)

	return &common_configs.Configs{MainConfigs: mainConfigs, CleanupConfigs: []*common_configs.CommandExecutorPairedConfig{}}
}

// GeneratePreConfigs generates pre cmd execution for ctpv2.
func GeneratePreConfigs(ctx context.Context) *common_configs.Configs {
	mainConfigs := []*common_configs.CommandExecutorPairedConfig{}
	cleanupConfigs := []*common_configs.CommandExecutorPairedConfig{}

	// Translate v1 to v2, Start CTR and do GcloudAuth
	mainConfigs = append(mainConfigs,
		TranslateV1toV2Request_NoExecutor,
		CtrStartAsync_CtrExecutor,
		GcloudAuth_CtrExecutor)

	// Cleanup configs
	cleanupConfigs = append(cleanupConfigs,
		CtrStop_CtrExecutor)

	return &common_configs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanupConfigs}
}

// GeneratePostConfigs generates post cmd execution for ctpv2.
func GeneratePostConfigs(ctx context.Context) *common_configs.Configs {
	mainConfigs := []*common_configs.CommandExecutorPairedConfig{}

	// Stop Ctr
	mainConfigs = append(mainConfigs,
		Summarize_NoExecutor,
		CtrStop_CtrExecutor.WithRequired(true))

	return &common_configs.Configs{MainConfigs: mainConfigs, CleanupConfigs: []*common_configs.CommandExecutorPairedConfig{}}
}
