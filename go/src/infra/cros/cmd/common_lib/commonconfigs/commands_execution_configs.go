// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonconfigs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// CommandExecutorPairedConfig represents command and executor pair
type CommandExecutorPairedConfig struct {
	CommandType  interfaces.CommandType
	ExecutorType interfaces.ExecutorType

	required bool
}

// ToString returns string representation of the object.
func (cepc *CommandExecutorPairedConfig) ToString() string {
	if cepc == nil {
		return ""
	}

	return fmt.Sprintf("%s_%s", cepc.CommandType, cepc.ExecutorType)
}

// IsRequired returns whether the command is required.
func (cepc *CommandExecutorPairedConfig) IsRequired() bool {
	return cepc.required
}

// WithRequired returns back the CommandExecutorPairedConfig with required set.
func (cepc *CommandExecutorPairedConfig) WithRequired(required bool) *CommandExecutorPairedConfig {
	return &CommandExecutorPairedConfig{
		CommandType:  cepc.CommandType,
		ExecutorType: cepc.ExecutorType,
		required:     required,
	}
}

// Config types
const (
	HwTestExecutionConfigType       interfaces.ConfigType = "HwTest"
	PreLocalTestExecutionConfigType interfaces.ConfigType = "PreLocalTest"
	LocalTestExecutionConfigType    interfaces.ConfigType = "LocalTest"

	// For unit tests purposes only
	UnSupportedTestExecutionConfigType interfaces.ConfigType = "UnsupportedTest"
)

// Configs represents main and cleanup configs.
// Cleanup configs are executed if any error occurs with main config execution.
// If any command of cleanup config is already executed,
// they will be skipped during cleanup.
type Configs struct {
	MainConfigs    []*CommandExecutorPairedConfig
	CleanupConfigs []*CommandExecutorPairedConfig
}

// ToString returns string representation of the object.
func (configs *Configs) ToString() string {
	if configs == nil {
		return ""
	}

	retStringList := []string{}

	// Get main configs
	if len(configs.MainConfigs) > 0 {
		retStringList = append(retStringList, "Main Configs:")

		for _, mainConfig := range configs.MainConfigs {
			retStringList = append(retStringList, mainConfig.ToString())
		}
	}

	// Extra space
	if len(retStringList) > 0 {
		retStringList = append(retStringList, "\n")
	}

	// Get cleanup configs
	if len(configs.CleanupConfigs) > 0 {
		retStringList = append(retStringList, "Cleanup Configs:")

		for _, cleanupConfig := range configs.CleanupConfigs {
			retStringList = append(retStringList, cleanupConfig.ToString())
		}
	}

	if len(retStringList) == 0 {
		return ""
	}

	return strings.Join(retStringList, "\n")
}

// CmdExecutionConfig represents the configuration for any test execution.
type CmdExecutionConfig struct {
	*interfaces.AbstractTestExecutionConfig

	CommandConfig interfaces.CommandConfigInterface
	StateKeeper   interfaces.StateKeeperInterface
	Configs       *Configs

	executedCommands map[interfaces.CommandType]bool
}

func NewCmdExecutionConfig(
	configType interfaces.ConfigType,
	cmdConfig interfaces.CommandConfigInterface,
	ski interfaces.StateKeeperInterface) *CmdExecutionConfig {

	executedCmdMap := make(map[interfaces.CommandType]bool)
	abstractConfig := interfaces.NewAbstractTestExecutionConfig(configType)
	return &CmdExecutionConfig{
		AbstractTestExecutionConfig: abstractConfig,
		CommandConfig:               cmdConfig,
		StateKeeper:                 ski,
		executedCommands:            executedCmdMap}
}

func (tecfg *CmdExecutionConfig) Execute(ctx context.Context) error {
	if tecfg.Configs == nil || len(tecfg.Configs.MainConfigs) == 0 {
		return fmt.Errorf("cannot execute nil or empty conifgs. Please generate configs first")
	}

	// Process main configs
	cmds, err := tecfg.processCommandConfig(ctx, tecfg.Configs.MainConfigs)
	if err != nil {
		return errors.Annotate(
			err,
			"error during processing main configs for config type %s: ", tecfg.GetConfigType()).Err()
	}
	if len(cmds) == 0 {
		return fmt.Errorf("no valid commands found for config type %s", tecfg.GetConfigType())
	}

	// Process cleanup configs
	cleanupCmds, err := tecfg.processCommandConfig(ctx, tecfg.Configs.CleanupConfigs)
	if err != nil {
		return errors.Annotate(err, "error during processing clean up configs for config type %s: ", tecfg.GetConfigType()).Err()
	}

	err = tecfg.executeCommands(ctx, cmds, tecfg.Configs.MainConfigs, false, false)
	if err != nil {
		step, ctx := build.StartStep(ctx, "Clean up commands")
		defer func() { step.End(err) }()

		logging.Infof(ctx, "error during execution of main config commmands, %s", err)
		// execute clean up commands
		cleanupErr := tecfg.executeCommands(ctx, cleanupCmds, tecfg.Configs.CleanupConfigs, true, true)
		if cleanupErr != nil {
			err = fmt.Errorf("main error: %w; cleanup error: %w", err, cleanupErr)
		}
		return errors.Annotate(err, "error during executing config type %s: ", tecfg.GetConfigType()).Err()
	}

	return nil
}

// processCommandConfig processes paired configs to specific commands.
func (tecfg *CmdExecutionConfig) processCommandConfig(
	ctx context.Context,
	pairedConfig []*CommandExecutorPairedConfig) ([]interfaces.CommandInterface, error) {

	cmds := []interfaces.CommandInterface{}
	for _, cmdConfig := range pairedConfig {
		cmd, err := tecfg.CommandConfig.GetCommand(cmdConfig.CommandType, cmdConfig.ExecutorType)
		tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Processing cmd: %s %s", cmdConfig.CommandType, cmdConfig.ExecutorType))
		if err != nil {
			tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Error occurred: %s", err))
			return nil, errors.Annotate(err, "error during getting command for cmd type %s and executor type %s: ", cmdConfig.CommandType, cmdConfig.ExecutorType).Err()
		}
		logging.Infof(ctx, "Processing cmd: %T", cmd)
		if err := cmd.Instantiate(ctx, tecfg.StateKeeper); err != nil {
			tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Error occurred: %s", err))
			return nil, errors.Annotate(err, "error while instantiation command for cmd type %s and executor type %s: ", cmdConfig.CommandType, cmdConfig.ExecutorType).Err()
		}
		tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Processed."))
		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

// executeCommands executes given commands.
// Assuming the given commands are correct, it will execute repeated ones.
func (tecfg *CmdExecutionConfig) executeCommands(
	ctx context.Context,
	cmds []interfaces.CommandInterface,
	cmdExecPairConfigs []*CommandExecutorPairedConfig,
	executeAllCmds, isCleanup bool) error {
	var allErr error
	var singleErr error
	var firstErr error
	foundErr := false
	tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Executing Commands"))
	for i, cmd := range cmds {
		if firstErr == nil {
			// Once allErr is not nil, should only have a single error.
			firstErr = allErr
		}
		if foundErr && !executeAllCmds && !cmdExecPairConfigs[i].IsRequired() {
			continue
		}
		cmdType := cmd.GetCommandType()
		var cleanUpTag string
		if isCleanup {
			cleanUpTag = "(CLEAN UP)"
		}
		logging.Infof(ctx, "Executing cmd: %s %s", cleanUpTag, cmdType)
		tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Executing cmd: %s %s", cleanUpTag, cmdType))

		if singleErr = cmd.ExtractDependencies(ctx, tecfg.StateKeeper); singleErr != nil {
			foundErr = true
			allErr = errors.Append(allErr, singleErr)
			logging.Infof(ctx, "Command type %s extract dependencies failed, %s", cmdType, singleErr)
			tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Command type %s extract dependencies failed, %s", cmdType, singleErr))
			continue
		}

		var bqClient *bigquery.Client
		var reqs *api.CTPv2Request
		var req *api.CTPRequest

		var buildstate *build.State
		switch sk := tecfg.StateKeeper.(type) {
		case *data.PrePostFilterStateKeeper:
			bqClient = sk.BQClient
			reqs = sk.CtpV2Request
			buildstate = sk.BuildState
		case *data.FilterStateKeeper:
			bqClient = sk.BQClient
			buildstate = sk.BuildState
			req = sk.CtpReq
		}

		analytics.SoftInsertStepWCtp2Req(ctx, bqClient, &analytics.BqData{Step: string(cmdType), Status: analytics.Start}, reqs, buildstate, req)
		startTime := time.Now()
		status := analytics.Success
		if singleErr = cmd.Execute(ctx); singleErr != nil {
			foundErr = true
			allErr = errors.Append(allErr, singleErr)
			logging.Infof(ctx, "Command type %s execution failed: %s.", cmdType, singleErr)
			tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Command type %s execution failed: %s.", cmdType, singleErr))
			logging.Infof(ctx, "Attempting to update state keeper.")
			if innerErr := cmd.UpdateStateKeeper(ctx, tecfg.StateKeeper); innerErr != nil {
				logging.Infof(ctx, "Command type %s could not update state keeper: %s", cmdType, innerErr)
			}
			continue
		}

		analytics.SoftInsertStepWCtp2Req(ctx, bqClient, &analytics.BqData{Step: string(cmdType), Status: status, Duration: float32(time.Since(startTime).Seconds())}, reqs, buildstate, req)

		logging.Infof(ctx, "Cmd completed: %s", cmdType)

		tecfg.executedCommands[cmdType] = true
		if singleErr = cmd.UpdateStateKeeper(ctx, tecfg.StateKeeper); singleErr != nil {
			foundErr = true
			allErr = errors.Append(allErr, singleErr)
			tecfg.StateKeeper.AppendToAIExecutionContext(fmt.Sprintf("Command type %s UpdateStateKeeper failed: %s.", cmdType, singleErr))
			continue
		}
	}

	if allErr != nil {
		logging.Infof(ctx, "errors found: %s", allErr.Error())
	}

	// Most important error for display is the first one found.
	// Other subsequent errors should be relegated to logging.
	return firstErr
}
