// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import "context"

type ExecutorType string

// ExecutorInterface defines the contract an executor will have to satisfy.
type ExecutorInterface interface {
	// GetExecutorType returns the executor type.
	GetExecutorType() ExecutorType

	// ExecuteCommand executes the provided command via current executor.
	ExecuteCommand(context.Context, CommandInterface) error
}

// AbstractExecutor satisfies the executor requirement that is common to all.
type AbstractExecutor struct {
	ExecutorInterface

	executorType ExecutorType
}

func NewAbstractExecutor(exType ExecutorType) *AbstractExecutor {
	return &AbstractExecutor{executorType: exType}
}

func (ex *AbstractExecutor) GetExecutorType() ExecutorType {
	return ex.executorType
}
