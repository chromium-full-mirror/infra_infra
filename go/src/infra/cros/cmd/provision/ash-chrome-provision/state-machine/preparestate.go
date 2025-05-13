// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/types/known/anypb"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"

	ashchromeservice "go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/service"
	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

type AshChromePrepareState struct {
	service *ashchromeservice.AshChromeService
}

func NewAshChromePrepareState(service *ashchromeservice.AshChromeService) commonutils.ServiceState {
	return AshChromePrepareState{
		service: service,
	}
}

// AshChromePrepareState downloads and extracts build artifacts.
// The already downloaded images will not be downloaded and extracted again.
func (s AshChromePrepareState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Printf("Started %s\n", s.Name())
	artifact := s.service.GetArtifactPath()
	tmpDir := s.service.GetTmpDir()
	switch artifact.HostType {
	case conf.StoragePath_GS:
		if err := s.service.DownloadChromeArtifactsFromGS(ctx, artifact.Path, filepath.Join(tmpDir, "chrome.tar.zst")); err != nil {
			return nil, api.InstallResponse_STATUS_GS_DOWNLOAD_FAILED, err
		}
		if err := os.Mkdir(filepath.Join(tmpDir, "chrome"), 0755); err != nil {
			return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
		}
		if err := s.service.ExtractChromeArtifacts(ctx, filepath.Join(tmpDir, "chrome.tar.zst"), filepath.Join(tmpDir, "chrome")); err != nil {
			return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
		}
	default:
		log.Printf("Unhandled types: %v", artifact.HostType)
		return nil, api.InstallResponse_STATUS_INVALID_REQUEST, fmt.Errorf("unsupported artifact host_type: %v", artifact.HostType)
	}

	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s AshChromePrepareState) Next() commonutils.ServiceState {
	return DeployState(s)
}

func (s AshChromePrepareState) Name() string {
	return "Ash Chrome Prepare"
}
