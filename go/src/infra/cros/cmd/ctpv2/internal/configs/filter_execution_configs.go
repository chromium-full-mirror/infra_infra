// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/commonconfigs"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// Config types
const (
	LuciBuildFilterExecutionConfigType interfaces.ConfigType = "LuciBuild"
	Ctpv2PreExecutionConfigType        interfaces.ConfigType = "Ctpv2PreExection"
	Ctpv2PostExecutionConfigType       interfaces.ConfigType = "Ctpv2PostExection"

	// For unit tests purposes only
	UnSupportedFilterExecutionConfigType interfaces.ConfigType = "UnsupportedTest"
)

// Ctpv2ExecutionConfig represents the configuration for any test execution.
type Ctpv2ExecutionConfig struct {
	*commonconfigs.CmdExecutionConfig

	TotalFilters int
}

func NewCtpv2ExecutionConfig(
	totalFilters int,
	configType interfaces.ConfigType,
	cmdConfig interfaces.CommandConfigInterface,
	ski interfaces.StateKeeperInterface) *Ctpv2ExecutionConfig {

	cmdExecutionConfig := commonconfigs.NewCmdExecutionConfig(configType, cmdConfig, ski)
	return &Ctpv2ExecutionConfig{
		CmdExecutionConfig: cmdExecutionConfig,
		TotalFilters:       totalFilters,
	}
}

func (ctpv2cfg *Ctpv2ExecutionConfig) GenerateConfig(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Generate configs: %s", ctpv2cfg.GetConfigType()))
	defer func() { step.End(err) }()

	switch configType := ctpv2cfg.GetConfigType(); configType {
	case Ctpv2PreExecutionConfigType:
		ctpv2cfg.Configs = GeneratePreConfigs(ctx)
	case LuciBuildFilterExecutionConfigType:
		ctpv2cfg.Configs = GenerateFilterConfigs(ctx, ctpv2cfg.TotalFilters)
	case Ctpv2PostExecutionConfigType:
		ctpv2cfg.Configs = GeneratePostConfigs(ctx)
	default:
		err = fmt.Errorf("config type %s is not supported", configType)
	}

	if ctpv2cfg.Configs != nil {
		configsLog := step.Log("generated configs")
		_, logErr := configsLog.Write([]byte(ctpv2cfg.Configs.ToString()))
		ctpv2cfg.CmdExecutionConfig.StateKeeper.AppendToAIExecutionContext("Execution Steps to follow: \n" + ctpv2cfg.Configs.ToString())
		if logErr != nil {
			logging.Infof(ctx, "error during writing generated configs: %s", logErr)
		}
	}

	return err
}
