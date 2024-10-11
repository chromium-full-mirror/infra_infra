// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common_commands"
	"infra/cros/cmd/common_lib/common_configs"
	"infra/cros/cmd/common_lib/common_executors"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func TestGetCommand_UnsupportedCmdType(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported command type", t, func(t *ftt.Test) {
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		contConfig := common_configs.NewContainerConfig(ctr, nil, false)
		execConfig := NewExecutorConfig(ctr, contConfig)
		cmdConfig := NewCommandConfig(execConfig)
		cmd, err := cmdConfig.GetCommand(common_commands.UnSupportedCmdType, common_executors.NoExecutorType)
		assert.Loosely(t, cmd, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

// TODO: Fix tests
// func TestGetCommand_SupportedCmdType(t *testing.T) {
// 	t.Parallel()
//
// 	ftt.Run("Supported command type", t, func(t *ftt.Test) {
// 		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
// 		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
// 		contConfig := common_configs.NewContainerConfig(ctr, getMockContainerImagesInfo(), false)
// 		execConfig := NewExecutorConfig(ctr, contConfig)
// 		cmdConfig := NewCommandConfig(execConfig)
//
// 		cmd, err := cmdConfig.GetCommand(commands.BuildInputValidationCmdType, executors.NoExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.ParseEnvInfoCmdType, executors.NoExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.InvServiceStartCmdType, executors.InvServiceExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.InvServiceStopCmdType, executors.InvServiceExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.LoadDutTopologyCmdType, executors.InvServiceExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.CtrServiceStartAsyncCmdType, executors.CtrExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.CtrServiceStopCmdType, executors.CtrExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.GcloudAuthCmdType, executors.CtrExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.DutServiceStartCmdType, executors.CrosDutExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.DutVmGetImageCmdType, executors.CrosDutVmExecutorType)
// 		assert.Loosely(t, cmd.GetCommandType(), should.Equal(commands.DutVmGetImageCmdType))
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.DutVmLeaseCmdType, executors.CrosDutVmExecutorType)
// 		assert.Loosely(t, cmd.GetCommandType(), should.Equal(commands.DutVmLeaseCmdType))
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.DutVmReleaseCmdType, executors.CrosDutVmExecutorType)
// 		assert.Loosely(t, cmd.GetCommandType(), should.Equal(commands.DutVmReleaseCmdType))
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.DutVmCacheServerStartCmdType, executors.CrosDutVmExecutorType)
// 		assert.Loosely(t, cmd.GetCommandType(), should.Equal(commands.DutVmCacheServerStartCmdType))
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.ProvisionServiceStartCmdType, executors.CrosProvisionExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.ProvisonInstallCmdType, executors.CrosProvisionExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.TestServiceStartCmdType, executors.CrosTestExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.TestsExecutionCmdType, executors.CrosTestExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.RdbPublishStartCmdType, executors.CrosRdbPublishExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.RdbPublishUploadCmdType, executors.CrosRdbPublishExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.TkoPublishStartCmdType, executors.CrosTkoPublishExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.TkoPublishUploadCmdType, executors.CrosTkoPublishExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.GcsPublishStartCmdType, executors.CrosGcsPublishExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.SshStartReverseTunnelCmdType, executors.SshTunnelExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.SshStartTunnelCmdType, executors.SshTunnelExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.SshStopTunnelsCmdType, executors.SshTunnelExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.TestFinderServiceStartCmdType, executors.CrosTestFinderExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.TestFinderExecutionCmdType, executors.CrosTestFinderExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.BuildDutTopologyCmdType, executors.InvServiceExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.CacheServerStartCmdType, executors.CacheServerExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.ParseArgsCmdType, executors.NoExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		cmd, err = cmdConfig.GetCommand(commands.UpdateContainerImagesLocallyCmdType, executors.NoExecutorType)
// 		assert.Loosely(t, cmd, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
// 	})
// }
