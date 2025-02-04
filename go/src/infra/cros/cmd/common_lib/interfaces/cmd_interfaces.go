// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package interfaces defines the common interface types.
package interfaces

import (
	"context"

	"go.chromium.org/luci/common/errors"
)

type CommandType string

// CommandInterface defines the contract a command will have to satisfy.
type CommandInterface interface {
	// GetCommandType returns the command type.
	GetCommandType() CommandType

	// Instantiate pulls data from the provided state keeper
	// to instantiate it during config generation.
	Instantiate(context.Context, StateKeeperInterface) error

	// ExtractDependencies extracts command dependencies from
	// the provided state keeper before command execution.
	ExtractDependencies(context.Context, StateKeeperInterface) error

	// Execute executes the command.
	Execute(context.Context) error

	// UpdateStateKeeper updates the provided state keeper after command execution.
	UpdateStateKeeper(context.Context, StateKeeperInterface) error
}

// AbstractCmd satisfies the command requirement that is common to all.
type AbstractCmd struct {
	CommandInterface

	cmdType CommandType
}

func NewAbstractCmd(cmdType CommandType) *AbstractCmd {
	return &AbstractCmd{cmdType: cmdType}
}

func (cmd *AbstractCmd) GetCommandType() CommandType {
	return cmd.cmdType
}

func (cmd *AbstractCmd) Instantiate(
	ctx context.Context,
	ski StateKeeperInterface) error {
	// Nothing to instantiate (Sub classes will overwrite if necessary)
	return nil
}

func (cmd *AbstractCmd) ExtractDependencies(
	ctx context.Context,
	ski StateKeeperInterface) error {
	// No deps (Sub classes will overwrite if necessary)
	return nil
}

func (cmd *AbstractCmd) UpdateStateKeeper(
	ctx context.Context,
	ski StateKeeperInterface) error {
	// Nothing to update (Sub classes will overwrite if necessary)
	return nil
}

// AbstractSingleCmdByNoExecutor represents a single command with no executor.
type AbstractSingleCmdByNoExecutor struct {
	*AbstractCmd
}

// SingleCmdByExecutor represents a single command executed by an executor.
type SingleCmdByExecutor struct {
	*AbstractCmd

	ConcreteCmd CommandInterface

	commandExecutor ExecutorInterface
}

func NewSingleCmdByExecutor(
	cmdType CommandType,
	executor ExecutorInterface) *SingleCmdByExecutor {
	return &SingleCmdByExecutor{
		AbstractCmd:     NewAbstractCmd(cmdType),
		commandExecutor: executor,
	}
}

func (cmd *SingleCmdByExecutor) Execute(ctx context.Context) error {
	err := cmd.commandExecutor.ExecuteCommand(ctx, cmd.ConcreteCmd)
	if err != nil {
		return errors.Annotate(err, "error executing command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// MultipleCmdsByExecutor represents multiple commands
// inside a single command executed by an executor.
type MultipleCmdsByExecutor struct {
	*AbstractCmd

	commandExecutor ExecutorInterface
	commandList     []CommandInterface
}

func NewMultipleCmdsByExecutor(
	cmdType CommandType,
	executor ExecutorInterface,
	cmdList []CommandInterface) CommandInterface {
	return &MultipleCmdsByExecutor{
		AbstractCmd:     NewAbstractCmd(cmdType),
		commandExecutor: executor,
		commandList:     cmdList,
	}
}

func (cmd *MultipleCmdsByExecutor) Execute(ctx context.Context) error {
	var err error
	currCmdType := cmd.GetCommandType()

	for _, currCmd := range cmd.commandList {
		currCmdType = currCmd.GetCommandType()
		if err = cmd.commandExecutor.ExecuteCommand(ctx, currCmd); err != nil {
			break
		}
	}

	if err != nil {
		return errors.Annotate(err, "error executing command %s: ", currCmdType).Err()
	}

	return nil
}
