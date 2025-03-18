// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// ServiceState is a single state representation.
// TODO: Get rid of this state machine.
type ServiceState interface {
	// Execute Runs the state
	Execute(ctx context.Context, log *log.Logger) (*api.FirmwareProvisionResponse, api.InstallResponse_Status, error)
	// Next gets the next state in the state machine
	Next() ServiceState
	// Name gets the fully qualified name of this state
	Name() string
}
