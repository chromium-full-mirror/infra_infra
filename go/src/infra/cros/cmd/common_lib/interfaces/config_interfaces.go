// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import (
	"context"
)

type ConfigType string

// TestExecutionConfigInterface defines the contract an execution config
// will have to satisfy.
type TestExecutionConfigInterface interface {
	// GetConfigType returns the config type.
	GetConfigType() ConfigType

	// GenerateConfigs generates configs.
	GenerateConfigs(context.Context) error

	// Execute executes the generated execution config.
	Execute(context.Context, StateKeeperInterface) error
}

// AbstractTestExecutionConfig satisfies the config requirement
// that is common to all.
type AbstractTestExecutionConfig struct {
	configType ConfigType
}

func NewAbstractTestExecutionConfig(configType ConfigType) *AbstractTestExecutionConfig {
	return &AbstractTestExecutionConfig{configType: configType}
}

func (absConfig *AbstractTestExecutionConfig) GetConfigType() ConfigType {
	return ConfigType(absConfig.configType)
}

// CommandConfigInterface defines the contract a command config
// will have to satisfy.
type CommandConfigInterface interface {
	// GetCommand returns the concrete command based on
	// provided command and executor type.
	GetCommand(CommandType, ExecutorType) (CommandInterface, error)
}

// ExecutorConfigInterface defines the contract an executor config
// will have to satisfy.
type ExecutorConfigInterface interface {
	// GetExecutor returns the concrete executor based on provided executor type.
	GetExecutor(execType ExecutorType) (ExecutorInterface, error)
}

// ContainerConfigInterface defines the contract container config
// will have to satisfy.
type ContainerConfigInterface interface {
	// GetContainer returns the concrete container based on
	// provided container type.
	GetContainer(contType ContainerType) (ContainerInterface, error)
}
