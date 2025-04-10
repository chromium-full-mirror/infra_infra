// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	tpcommon "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/commonconfigs"
	"go.chromium.org/infra/cros/cmd/common_lib/commonexecutors"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/executors"
)

// All currently supported command-executor pairs.
var InputValidation_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.BuildInputValidationCmdType, ExecutorType: executors.NoExecutorType}
var ParseEnvInfo_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ParseEnvInfoCmdType, ExecutorType: executors.NoExecutorType}
var InvServiceStart_InvExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.InvServiceStartCmdType, ExecutorType: executors.InvServiceExecutorType}
var InvServiceStop_InvExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.InvServiceStopCmdType, ExecutorType: executors.InvServiceExecutorType}
var LoadDutTopology_InvExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.LoadDutTopologyCmdType, ExecutorType: executors.InvServiceExecutorType}
var BuildDutTopology_InvExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.BuildDutTopologyCmdType, ExecutorType: executors.InvServiceExecutorType}
var CtrStartAsync_CtrExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.CtrServiceStartAsyncCmdType, ExecutorType: commonexecutors.CtrExecutorType}
var CtrStop_CtrExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.CtrServiceStopCmdType, ExecutorType: commonexecutors.CtrExecutorType}
var GcloudAuth_CtrExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.GcloudAuthCmdType, ExecutorType: commonexecutors.CtrExecutorType}
var DutServerStart_CrosDutExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.DutServiceStartCmdType, ExecutorType: executors.CrosDutExecutorType}
var AndroidCompanionDutServerStart_AndroidDutExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.AndroidCompanionDutServiceStartCmdType, ExecutorType: executors.AndroidDutExecutorType}
var AndroidProvisionServerStart_AndroidProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.AndroidProvisionServiceStartCmdType, ExecutorType: executors.AndroidProvisionExecutorType}
var AndroidProvisionInstall_AndroidProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.AndroidProvisionInstallCmdType, ExecutorType: executors.AndroidProvisionExecutorType}
var ProvisionServerStart_CrosProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ProvisionServiceStartCmdType, ExecutorType: executors.CrosProvisionExecutorType}
var ProvisionInstall_CrosProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ProvisonInstallCmdType, ExecutorType: executors.CrosProvisionExecutorType}
var TestServerStart_CrosTestExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TestServiceStartCmdType, ExecutorType: executors.CrosTestExecutorType}
var TestsExecution_CrosTestExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TestsExecutionCmdType, ExecutorType: executors.CrosTestExecutorType}
var TestFinderServerStart_CrosTestFinderExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TestFinderServiceStartCmdType, ExecutorType: executors.CrosTestFinderExecutorType}
var TestFinderExecution_CrosTestFinderExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TestFinderExecutionCmdType, ExecutorType: executors.CrosTestFinderExecutorType}
var GcsPublishStart_CrosGcsPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GcsPublishStartCmdType, ExecutorType: executors.CrosGcsPublishExecutorType}
var GcsPublishUpload_CrosGcsPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GcsPublishUploadCmdType, ExecutorType: executors.CrosGcsPublishExecutorType}
var RdbPublishStart_CrosRdbPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.RdbPublishStartCmdType, ExecutorType: executors.CrosRdbPublishExecutorType}
var RdbPublishUpload_CrosRdbPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.RdbPublishUploadCmdType, ExecutorType: executors.CrosRdbPublishExecutorType}
var TkoPublishStart_CrosTkoPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TkoPublishStartCmdType, ExecutorType: executors.CrosTkoPublishExecutorType}
var TkoPublishUpload_CrosTkoPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TkoPublishUploadCmdType, ExecutorType: executors.CrosTkoPublishExecutorType}
var CpconPublishStart_CrosCpconPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.CpconPublishStartCmdType, ExecutorType: executors.CrosPublishExecutorType}
var CpconPublishUpload_CrosCpconPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.CpconPublishUploadCmdType, ExecutorType: executors.CrosPublishExecutorType}
var ProcessResults_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ProcessResultsCmdType, ExecutorType: executors.NoExecutorType}
var UpdateDutState_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.UpdateDutStateCmdType, ExecutorType: executors.NoExecutorType}
var TkoDirectUpload_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.TkoDirectUploadCmdType, ExecutorType: executors.NoExecutorType}
var SshStartTunnel_SshTunnelExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.SshStartTunnelCmdType, ExecutorType: executors.SshTunnelExecutorType}
var SshStartReverseTunnel_SshTunnelExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.SshStartReverseTunnelCmdType, ExecutorType: executors.SshTunnelExecutorType}
var SshStopTunnels_SshTunnelExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.SshStopTunnelsCmdType, ExecutorType: executors.SshTunnelExecutorType}
var CacheServerStart_CacheServerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.CacheServerStartCmdType, ExecutorType: executors.CacheServerExecutorType}
var UpdateContainerImagesLocally_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.UpdateContainerImagesLocallyCmdType, ExecutorType: executors.NoExecutorType}
var FetchContainerMetadata_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.FetchContainerMetadataCmdType, ExecutorType: executors.NoExecutorType}
var ParseArgs_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ParseArgsCmdType, ExecutorType: executors.NoExecutorType}
var DutVmCacheServerStart_CacheServerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.DutVmCacheServerStartCmdType, ExecutorType: executors.CacheServerExecutorType}
var DutVmGetImage_CrosDutVmExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.DutVmGetImageCmdType, ExecutorType: executors.CrosDutVmExecutorType}
var DutServiceStart_CrosDutVmExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.DutServiceStartCmdType, ExecutorType: executors.CrosDutVmExecutorType}
var VMProvisionServerStart_CrosVMProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.VMProvisionServiceStartCmdType, ExecutorType: executors.CrosVMProvisionExecutorType}
var VMProvisionLease_CrosVMProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.VMProvisionLeaseCmdType, ExecutorType: executors.CrosVMProvisionExecutorType}
var VMProvisionRelease_CrosVMProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.VMProvisionReleaseCmdType, ExecutorType: executors.CrosVMProvisionExecutorType}
var ContainerStart_ContainerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerStartCmdType, ExecutorType: commonexecutors.ContainerExecutorType}
var ContainerCloseLogs_ContainerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerCloseLogsCmdType, ExecutorType: commonexecutors.ContainerExecutorType}
var ContainerReadLogs_ContainerExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commoncommands.ContainerReadLogsCmdType, ExecutorType: commonexecutors.ContainerExecutorType}
var GenericProvision_GenericProvisionExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GenericProvisionCmdType, ExecutorType: executors.GenericProvisionExecutorType}
var GenericTests_GenericTestsExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GenericTestsCmdType, ExecutorType: executors.GenericTestsExecutorType}
var GenericPostProcess_GenericPostProcessExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GenericPostProcessCmdType, ExecutorType: executors.GenericPostProcessExecutorType}
var GenericPublish_GenericPublishExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GenericPublishCmdType, ExecutorType: executors.GenericPublishExecutorType}
var GenericService_GenericServiceExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.GenericServiceCmdType, ExecutorType: executors.GenericServiceExecutorType}
var ParseDutTopology_NoExecutor = &commonconfigs.CommandExecutorPairedConfig{CommandType: commands.ParseDutTopologyCmdType, ExecutorType: commonexecutors.NoExecutorType}

// GenerateHwConfigs generates hw tests execution for lab environment.
func GenerateHwConfigs(ctx context.Context, cftHwStepsConfig *tpcommon.HwTestConfig, sk *data.HwTestStateKeeper, inputV2 *api.CrosTestRunnerDynamicRequest, isAndroidProvisionRequired bool) *commonconfigs.Configs {
	platform := common.GetBotProvider()
	if inputV2 != nil {
		return hwConfigsForPlatformV2(cftHwStepsConfig, inputV2, platform)
	}
	return hwConfigsForPlatform(cftHwStepsConfig, sk, platform, isAndroidProvisionRequired)
}

// hwConfigsForPlatform generates platform-specific configs.
// GCE platform will get configs for VM test on GCE.
// Non-GCE platforms (Drone and Unknown) will get configs for HW test on Drone.
func hwConfigsForPlatform(cftHwStepsConfig *tpcommon.HwTestConfig, sk *data.HwTestStateKeeper, platform common.SwarmingBotProvider, isAndroidProvisionRequired bool) *commonconfigs.Configs {
	// Overwrite configs that don't apply to VM test
	if platform == common.BotProviderGce {
		if cftHwStepsConfig == nil {
			cftHwStepsConfig = &tpcommon.HwTestConfig{}
		}
		// Skip DutTopology and Provision steps, as those are done in the
		// non-skippable Starting Dut Service step
		cftHwStepsConfig.SkipLoadingDutTopology = true
		cftHwStepsConfig.SkipProvision = true
		cftHwStepsConfig.SkipStartingDutService = false
	}
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{}

	// Input validation and parse env commands
	mainConfigs = append(mainConfigs,
		InputValidation_NoExecutor,
		ParseEnvInfo_NoExecutor)

	// Dut topology commands
	if !cftHwStepsConfig.GetSkipLoadingDutTopology() {
		mainConfigs = append(mainConfigs,
			InvServiceStart_InvExecutor,
			LoadDutTopology_InvExecutor,
			InvServiceStop_InvExecutor,
			ParseDutTopology_NoExecutor)
	}

	// Start CTR and gcloud auth commands
	mainConfigs = append(mainConfigs,
		CtrStartAsync_CtrExecutor,
		GcloudAuth_CtrExecutor)

	// Start dut server command
	if !cftHwStepsConfig.GetSkipStartingDutService() {
		if platform == common.BotProviderGce {
			// Prepare image, lease VM, start cache server before finally start Dut service
			mainConfigs = append(mainConfigs,
				DutVmGetImage_CrosDutVmExecutor,
				VMProvisionServerStart_CrosVMProvisionExecutor,
				VMProvisionLease_CrosVMProvisionExecutor,
				ParseDutTopology_NoExecutor,
				DutVmCacheServerStart_CacheServerExecutor,
				DutServiceStart_CrosDutVmExecutor)
		} else {
			mainConfigs = append(mainConfigs,
				DutServerStart_CrosDutExecutor)
			if isAndroidProvisionRequired {
				mainConfigs = append(mainConfigs,
					AndroidCompanionDutServerStart_AndroidDutExecutor)
			}
		}
	}

	// Provision commands
	if !cftHwStepsConfig.GetSkipProvision() {
		if sk != nil {
			if _, ok := sk.ContainerImages[common.ServoNexus]; ok {
				mainConfigs = append(mainConfigs, ContainerStart_ContainerExecutor.WithRequired(true))
			}
		}
		mainConfigs = append(mainConfigs,
			ProvisionServerStart_CrosProvisionExecutor,
			ProvisionInstall_CrosProvisionExecutor)
		if isAndroidProvisionRequired {
			mainConfigs = append(mainConfigs,
				AndroidProvisionServerStart_AndroidProvisionExecutor,
				AndroidProvisionInstall_AndroidProvisionExecutor)
		}
	}

	// Test execution commands
	if !cftHwStepsConfig.GetSkipTestExecution() {
		mainConfigs = append(mainConfigs,
			TestServerStart_CrosTestExecutor,
			TestsExecution_CrosTestExecutor)
	}

	// Add support for dynamic command/executor for post-process.
	if !cftHwStepsConfig.GetSkipPostProcess() {
		mainConfigs = append(mainConfigs,
			ContainerStart_ContainerExecutor.WithRequired(true),
			GenericPostProcess_GenericPostProcessExecutor.WithRequired(true))
	}

	// If VM run, release VM before publish
	if platform == common.BotProviderGce {
		mainConfigs = append(mainConfigs,
			VMProvisionRelease_CrosVMProvisionExecutor.WithRequired(true))
	}

	// Publish commands
	if !cftHwStepsConfig.GetSkipAllResultPublish() {
		// Rdb publish commands
		if !cftHwStepsConfig.GetSkipRdbPublish() {
			mainConfigs = append(mainConfigs,
				RdbPublishStart_CrosRdbPublishExecutor,
				RdbPublishUpload_CrosRdbPublishExecutor)
		}

		// Gcs publish commands
		if !cftHwStepsConfig.GetSkipGcsPublish() {
			mainConfigs = append(mainConfigs,
				GcsPublishStart_CrosGcsPublishExecutor.WithRequired(true),
				GcsPublishUpload_CrosGcsPublishExecutor.WithRequired(true))
		}

		// Cpcon publish commands
		if cftHwStepsConfig.GetRunCpconPublish() {
			mainConfigs = append(mainConfigs,
				CpconPublishStart_CrosCpconPublishExecutor.WithRequired(true),
				CpconPublishUpload_CrosCpconPublishExecutor.WithRequired(true))
		}
	}

	// Stop CTR and result processing commands
	if platform == common.BotProviderGce {
		mainConfigs = append(mainConfigs,
			CtrStop_CtrExecutor.WithRequired(true),
			ProcessResults_NoExecutor.WithRequired(true))
	} else {
		mainConfigs = append(mainConfigs,
			CtrStop_CtrExecutor.WithRequired(true),
			UpdateDutState_NoExecutor.WithRequired(true),
			ProcessResults_NoExecutor.WithRequired(true))
	}

	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: []*commonconfigs.CommandExecutorPairedConfig{}}
}

func GeneratePreLocalConfigs(ctx context.Context) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{
		ParseArgs_NoExecutor,
		FetchContainerMetadata_NoExecutor,
		UpdateContainerImagesLocally_NoExecutor,
	}

	// Clean up configs. They will be executed if any failures occurs
	// in main configs. If any of the cleanup cmd is already executed,
	// they will be skipped.
	cleanupConfigs := []*commonconfigs.CommandExecutorPairedConfig{}

	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanupConfigs}
}

// hwConfigsForPlatformV2 generates the command/executor pair configs for the CrosTestRunnerRequest.
func hwConfigsForPlatformV2(cftHwStepsConfig *tpcommon.HwTestConfig, inputV2 *api.CrosTestRunnerDynamicRequest, platform common.SwarmingBotProvider) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{}
	cleanupConfigs := []*commonconfigs.CommandExecutorPairedConfig{}

	// Overwrite configs that don't apply to VM test
	if platform == common.BotProviderGce {
		if cftHwStepsConfig == nil {
			cftHwStepsConfig = &tpcommon.HwTestConfig{}
		}
		// Skip DutTopology and Provision steps, as those are done in the
		// non-skippable Starting Dut Service step
		cftHwStepsConfig.SkipLoadingDutTopology = true
		cftHwStepsConfig.SkipProvision = true
		cftHwStepsConfig.SkipStartingDutService = false
	}

	mainConfigs = append(mainConfigs,
		InputValidation_NoExecutor,
		ParseEnvInfo_NoExecutor)

	if !cftHwStepsConfig.GetSkipLoadingDutTopology() {
		mainConfigs = append(mainConfigs,
			InvServiceStart_InvExecutor,
			LoadDutTopology_InvExecutor,
			InvServiceStop_InvExecutor,
			ParseDutTopology_NoExecutor)
	}

	// Start CTR and gcloud auth commands
	mainConfigs = append(mainConfigs,
		CtrStartAsync_CtrExecutor,
		GcloudAuth_CtrExecutor,
		ContainerReadLogs_ContainerExecutor)

	if !cftHwStepsConfig.GetSkipStartingDutService() && platform == common.BotProviderGce {
		mainConfigs = append(mainConfigs,
			DutVmGetImage_CrosDutVmExecutor,
			VMProvisionServerStart_CrosVMProvisionExecutor,
			VMProvisionLease_CrosVMProvisionExecutor,
			ParseDutTopology_NoExecutor)
	}

	// Add task configs
	mainConfigs = append(mainConfigs, generateTaskConfigs(inputV2, platform).MainConfigs...)

	// Stop CTR and result processing commands
	mainConfigs = append(mainConfigs,
		ContainerCloseLogs_ContainerExecutor.WithRequired(true),
		CtrStop_CtrExecutor.WithRequired(true),
	)

	if platform != common.BotProviderGce {
		mainConfigs = append(mainConfigs, UpdateDutState_NoExecutor.WithRequired(true))
	}

	mainConfigs = append(mainConfigs, ProcessResults_NoExecutor.WithRequired(true))
	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanupConfigs}
}

func GenerateLocalConfigs(ctx context.Context, sk *data.LocalTestStateKeeper) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{
		CtrStartAsync_CtrExecutor,
		GcloudAuth_CtrExecutor,
	}

	if !sk.Args.SkipCacheServer {
		mainConfigs = append(mainConfigs,
			CacheServerStart_CacheServerExecutor)
	}

	if !sk.Args.SkipSshTunnel {
		mainConfigs = append(mainConfigs,
			SshStartTunnel_SshTunnelExecutor)
	}

	if !sk.Args.SkipSshReverseTunnel {
		mainConfigs = append(mainConfigs,
			SshStartReverseTunnel_SshTunnelExecutor)
	}

	if !sk.Args.SkipBuildDutTopology {
		mainConfigs = append(mainConfigs,
			BuildDutTopology_InvExecutor)
	}

	mainConfigs = append(mainConfigs,
		ParseDutTopology_NoExecutor)

	if !sk.Args.SkipDutServer {
		mainConfigs = append(mainConfigs,
			DutServerStart_CrosDutExecutor)
	}
	if !sk.Args.SkipProvision {
		mainConfigs = append(mainConfigs,
			ProvisionServerStart_CrosProvisionExecutor,
			ProvisionInstall_CrosProvisionExecutor)
	}
	if !sk.Args.SkipTestFinder {
		mainConfigs = append(mainConfigs,
			TestFinderServerStart_CrosTestFinderExecutor,
			TestFinderExecution_CrosTestFinderExecutor)
	}
	if !sk.Args.SkipTest {
		mainConfigs = append(mainConfigs,
			TestServerStart_CrosTestExecutor,
			TestsExecution_CrosTestExecutor)
	}

	// Cpcon publish commands
	if sk.Args.RunCpconPublish {
		mainConfigs = append(mainConfigs,
			CpconPublishStart_CrosCpconPublishExecutor,
			CpconPublishUpload_CrosCpconPublishExecutor)
	}

	mainConfigs = append(mainConfigs,
		CtrStop_CtrExecutor,
		SshStopTunnels_SshTunnelExecutor,
		ProcessResults_NoExecutor)

	// Clean up configs. They will be executed if any failures occurs
	// in main configs. If any of the cleanup cmd is already executed,
	// they will be skipped.
	cleanupConfigs := []*commonconfigs.CommandExecutorPairedConfig{
		CtrStop_CtrExecutor,
		SshStopTunnels_SshTunnelExecutor,
		ProcessResults_NoExecutor,
	}

	return &commonconfigs.Configs{MainConfigs: mainConfigs, CleanupConfigs: cleanupConfigs}
}

func generateTaskConfigs(inputV2 *api.CrosTestRunnerDynamicRequest, platform common.SwarmingBotProvider) *commonconfigs.Configs {
	mainConfigs := []*commonconfigs.CommandExecutorPairedConfig{}
	vmReleased := false

	for _, task := range inputV2.GetOrderedTasks() {
		for range task.GetOrderedContainerRequests() {
			mainConfigs = append(mainConfigs,
				ContainerStart_ContainerExecutor.WithRequired(task.Required))
		}
		switch task.Task.(type) {
		case *api.CrosTestRunnerDynamicRequest_Task_Provision:
			mainConfigs = append(mainConfigs,
				GenericProvision_GenericProvisionExecutor.WithRequired(task.Required))
		case *api.CrosTestRunnerDynamicRequest_Task_PreTest:
		case *api.CrosTestRunnerDynamicRequest_Task_Test:
			mainConfigs = append(mainConfigs,
				GenericTests_GenericTestsExecutor.WithRequired(task.Required))
		case *api.CrosTestRunnerDynamicRequest_Task_PostTest:
			mainConfigs = append(mainConfigs,
				GenericPostProcess_GenericPostProcessExecutor.WithRequired(task.Required))
		case *api.CrosTestRunnerDynamicRequest_Task_Publish:
			if platform == common.BotProviderGce && !vmReleased {
				vmReleased = true
				mainConfigs = append(mainConfigs,
					VMProvisionRelease_CrosVMProvisionExecutor.WithRequired(true))
			}
			mainConfigs = append(mainConfigs,
				GenericPublish_GenericPublishExecutor.WithRequired(task.Required))
		case *api.CrosTestRunnerDynamicRequest_Task_Generic:
			mainConfigs = append(mainConfigs,
				GenericService_GenericServiceExecutor.WithRequired(task.Required))
		default:
		}
	}

	return &commonconfigs.Configs{MainConfigs: mainConfigs}
}
