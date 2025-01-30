// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"fmt"

	"infra/cros/cmd/common_lib/common_executors"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/ctpv2/internal/executors"
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
	case common_executors.CtrExecutorType:
		if cfg.Ctr == nil {
			return nil, fmt.Errorf("crosToolRunner is nil")
		}
		exec = common_executors.NewCtrExecutor(cfg.Ctr)

	case executors.FilterExecutorType:
		exec = executors.NewFilterExecutor()

	case common_executors.ContainerExecutorType:
		if cfg.Ctr == nil {
			return nil, fmt.Errorf("crosToolRunner is nil")
		}
		exec = common_executors.NewContainerExecutor(cfg.Ctr)

	default:
		return nil, fmt.Errorf("executor type %s not supported in executor configs", execType)
	}

	cfg.execsMap[execType] = exec
	return exec, nil
}
