// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Stub NotImplementedState.
package state_machine

import (
	"context"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	ashchromeservice "go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/service"
	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

// NotImplementedState is a stub state that fails.
type NotImplementedState struct {
	service *ashchromeservice.AshChromeService
}

// Execute return provisioning failure.
func (s NotImplementedState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("Started %s\n", s.Name())
	return nil, api.InstallResponse_STATUS_PROVISIONING_FAILED, nil
}

func (s NotImplementedState) Next() common_utils.ServiceState {
	return nil
}

func (s NotImplementedState) Name() string {
	return "Not Implemented"
}
