// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

// StateKeeperInterface defines the contract a state keeper
// will have to satisfy.
type StateKeeperInterface interface {
	// IsStateKeeper indicates if current object is a state keeper.
	IsStateKeeper()

	AppendToAIExecutionContext(context string)
}

// StateKeeper that can be extended by other state keepers.
type StateKeeper struct{}

func (sk *StateKeeper) IsStateKeeper() {}

func (sk *StateKeeper) AppendToAIExecutionContext(context string) {}
