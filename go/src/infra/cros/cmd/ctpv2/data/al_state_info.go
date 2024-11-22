// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"cloud.google.com/go/pubsub"

	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"infra/cros/cmd/common_lib/common"
)

// AlStateInfo captures the state info for Al runs
type AlStateInfo struct {
	IsAlRun     bool
	DoneTesting bool

	// Input test job that should not change during execution and only be used as reader
	InputTestJob *common.TestJobMessage

	// Current state that will be changed by cmds during execution
	CurrentTestJob           *common.TestJobMessage
	CurrentTestJobEvent      *common.TestJobEventMessage
	TestJobEventPubSubClient *pubsub.Client

	// WorkUnitTrees is a map that points to the head of each ATP request's
	// beginning node.
	//
	// NOTE: For the time being this map will only contain one tree until we
	// begin to support multiple ATP requests per CTP build.
	WorkUnitTrees map[string]*androidapi.WorkUnitTree

	// ATP is the instantiated ATP service API that we will reuse throughout the
	// build.
	ATP *androidapi.Service

	BuildID string

	// Fields for invocation generation logic.

	GenerateInvocation bool
	WorkUnitsOnly      bool
	ATPWorkUnit        *androidbuildinternal.WorkUnit
	ATPInvocation      *androidbuildinternal.Invocation
}

// GetWorkUnitTree fetches the ATP work unit tree if one exists and is being
// tracked.
func (a *AlStateInfo) GetWorkUnitTree() *androidapi.WorkUnitTree {
	if a.WorkUnitTrees == nil {
		return nil
	}

	if tree, ok := a.WorkUnitTrees["test"]; ok {
		return tree
	}

	return nil
}
