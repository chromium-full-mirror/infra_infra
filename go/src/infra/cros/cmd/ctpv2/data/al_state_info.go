// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/common"

	"cloud.google.com/go/pubsub"
)

// AlStateInfo captures the state info for Al runs
type AlStateInfo struct {
	IsAlRun bool
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
	WorkUnitTrees map[string]*androidapi.WorkUnitNode
}
