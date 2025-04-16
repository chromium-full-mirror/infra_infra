// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	ashchromeservice "go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/service"
	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

type AshChromePrepareState struct {
	service *ashchromeservice.AshChromeService
}

func NewAshChromePrepareState(service *ashchromeservice.AshChromeService) common_utils.ServiceState {
	return AshChromePrepareState{
		service: service,
	}
}

// AshChromePrepareState downloads and extracts build artifacts.
// The already downloaded images will not be downloaded and extracted again.
func (s AshChromePrepareState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("Started %s\n", s.Name())
	if err := s.service.WaitForReconnect(ctx); err != nil {
		return nil, api.InstallResponse_STATUS_DUT_UNREACHABLE_PRE_PROVISION, err
	}

	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s AshChromePrepareState) Next() common_utils.ServiceState {
	return NotImplementedState(s)
}

func (s AshChromePrepareState) Name() string {
	return "Ash Chrome Prepare"
}
