// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DeployState that deploys to DUT.
package state_machine

import (
	"context"
	"log"
	"path/filepath"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	ashchromeservice "go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/service"
	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

// DeployState is a state that deploy to DUT
type DeployState struct {
	service *ashchromeservice.AshChromeService
}

// Execute returns deploy result.
func (s DeployState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("Started %s\n", s.Name())
	s.service.LogChromeVersion(ctx)
	if err := s.service.MakeRootfsWritable(ctx); err != nil {
		log.Printf("Failed to MakeRootfsWritable: %v", err)
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
	}
	if err := s.service.ChromiteDeployChrome(ctx, filepath.Join(s.service.GetTmpDir(), "chrome")); err != nil {
		log.Printf("Failed to deploy chrome: %v", err)
		return nil, api.InstallResponse_STATUS_PROVISIONING_FAILED, err
	}
	log.Println("Ash-Chrome-Provision completed.")
	s.service.LogChromeVersion(ctx)
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s DeployState) Next() common_utils.ServiceState {
	return nil
}

func (s DeployState) Name() string {
	return "Deploy State"
}
