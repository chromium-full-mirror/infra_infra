// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// UserDefinedDynamicUpdateGenerator is any function that creates a list of dynamic updates.
type UserDefinedDynamicUpdateGenerator func() ([]*api.UserDefinedDynamicUpdate, error)

// Function types for concise definitions.
type InsertInstructionGetter func(task *api.CrosTestRunnerDynamicRequest_Task) *api.UserDefinedDynamicUpdate
type ModifyActionGetter func(modifications []*api.UpdateAction_Modify_Modification) *api.UserDefinedDynamicUpdate

// GenericTaskMessageRequest defines the inputs for the start, run, and stop
// rpc requests for a given generic task.
type GenericTaskMessageRequest struct {
	// Key to the InsertInstruction map.
	InsertionId string

	// Map of protos which will be wrapped by an Any.
	StaticInputs map[string]proto.Message
	// Map of dynamic dependencies. Items pointing to a non-any
	// object should be initialized within StaticInputs and postfixed
	// with `.value`.
	DynamicInputs map[string]string
}

// ProvisionTaskStartUpRequest defines the inputs for the startup request
// for a given provision task.
type ProvisionTaskStartUpRequest struct {
	StaticDut       *labapi.Dut
	StaticDutServer *labapi.IpEndpoint
	// Map of dynamic dependencies whose target would be the above
	// static inputs.
	DynamicInputs map[string]string
}

// ProvisionTaskInstallRequest defines the inputs for the install request
// for a given provision task.
type ProvisionTaskInstallRequest struct {
	StaticImagePath *_go.StoragePath
	StaticMetadata  *anypb.Any
	// Map of dynamic dependencies whose target would be the above
	// static inputs.
	DynamicInputs map[string]string
}
