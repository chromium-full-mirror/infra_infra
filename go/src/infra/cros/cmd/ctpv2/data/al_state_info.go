// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
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
}
