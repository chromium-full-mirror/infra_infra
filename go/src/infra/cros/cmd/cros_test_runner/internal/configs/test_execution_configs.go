// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"context"
	"fmt"

	tpcommon "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/commonconfigs"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// Config types
const (
	HwTestExecutionConfigType       interfaces.ConfigType = "HwTest"
	PreLocalTestExecutionConfigType interfaces.ConfigType = "PreLocalTest"
	LocalTestExecutionConfigType    interfaces.ConfigType = "LocalTest"

	// For unit tests purposes only
	UnSupportedTestExecutionConfigType interfaces.ConfigType = "UnsupportedTest"
)

// TestExecutionConfig represents the configuration for any test execution.
type Trv2ExecutionConfig struct {
	*commonconfigs.CmdExecutionConfig

	// commandConfig  interfaces.CommandConfigInterface
	// stateKeeper    interfaces.StateKeeperInterface
	cftStepsConfig *tpcommon.CftStepsConfig
	// configs        *Configs

	executedCommands map[interfaces.CommandType]bool
}

func NewTrv2ExecutionConfig(
	configType interfaces.ConfigType,
	cmdConfig interfaces.CommandConfigInterface,
	ski interfaces.StateKeeperInterface,
	cftStepsConfig *tpcommon.CftStepsConfig) *Trv2ExecutionConfig {

	cmdExecutionConfig := commonconfigs.NewCmdExecutionConfig(configType, cmdConfig, ski)
	return &Trv2ExecutionConfig{
		CmdExecutionConfig: cmdExecutionConfig,
		cftStepsConfig:     cftStepsConfig,
	}
}

func (trv2cfg *Trv2ExecutionConfig) GenerateConfig(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Generate configs: %s", trv2cfg.GetConfigType()))
	defer func() { step.End(err) }()

	switch configType := trv2cfg.GetConfigType(); configType {
	case HwTestExecutionConfigType:
		isAndroidProvisionRequired := trv2cfg.isAndroidProvisioningRequired(ctx)
		trv2cfg.Configs = GenerateHwConfigs(ctx, trv2cfg.cftStepsConfig.GetHwTestConfig(), trv2cfg.StateKeeper.(*data.HwTestStateKeeper), trv2cfg.StateKeeper.(*data.HwTestStateKeeper).CrosTestRunnerRequest, isAndroidProvisionRequired)
	case LocalTestExecutionConfigType:
		trv2cfg.Configs = GenerateLocalConfigs(ctx, trv2cfg.StateKeeper.(*data.LocalTestStateKeeper))
	case PreLocalTestExecutionConfigType:
		trv2cfg.Configs = GeneratePreLocalConfigs(ctx)
	default:
		err = fmt.Errorf("Config type %s is not supported!", configType)
	}

	if trv2cfg.Configs != nil {
		configsLog := step.Log("generated configs")
		_, logErr := configsLog.Write([]byte(trv2cfg.Configs.ToString()))
		if logErr != nil {
			logging.Infof(ctx, "error during writing generated configs: %s", logErr)
		}
	}

	return err
}
