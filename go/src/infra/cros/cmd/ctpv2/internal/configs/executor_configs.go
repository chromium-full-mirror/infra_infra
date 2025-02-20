// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"fmt"

	"go.chromium.org/infra/cros/cmd/common_lib/commonexecutors"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/executors"
)

// ExecutorConfig represents executor configs.
type ExecutorConfig struct {
	ContainerConfig   interfaces.ContainerConfigInterface
	InvServiceAddress string
	Ctr               *crostoolrunner.CrosToolRunner

	execsMap map[interfaces.ExecutorType]interfaces.ExecutorInterface
}

func NewExecutorConfig(
	ctr *crostoolrunner.CrosToolRunner,
	contConfig interfaces.ContainerConfigInterface) interfaces.ExecutorConfigInterface {
	execsMap := make(map[interfaces.ExecutorType]interfaces.ExecutorInterface)
	return &ExecutorConfig{Ctr: ctr, ContainerConfig: contConfig, execsMap: execsMap}
}

// GetExecutor returns the concrete executor based on provided executor type.
func (cfg *ExecutorConfig) GetExecutor(execType interfaces.ExecutorType) (interfaces.ExecutorInterface, error) {
	// Return executor if already created.
	if savedExec, ok := cfg.execsMap[execType]; ok {
		return savedExec, nil
	}

	var exec interfaces.ExecutorInterface

	// Get executor type based on executor type.
	switch execType {
	case commonexecutors.CtrExecutorType:
		if cfg.Ctr == nil {
			return nil, fmt.Errorf("crosToolRunner is nil")
		}
		exec = commonexecutors.NewCtrExecutor(cfg.Ctr)

	case executors.FilterExecutorType:
		exec = executors.NewFilterExecutor()

	case commonexecutors.ContainerExecutorType:
		if cfg.Ctr == nil {
			return nil, fmt.Errorf("crosToolRunner is nil")
		}
		exec = commonexecutors.NewContainerExecutor(cfg.Ctr)

	default:
		return nil, fmt.Errorf("executor type %s not supported in executor configs", execType)
	}

	cfg.execsMap[execType] = exec
	return exec, nil
}
