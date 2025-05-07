// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package statemachine contains the individual states representing the kernel
// provision state machine.
package statemachine

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/common-utils/cache"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// DownloadArtifactsState represents the tasks needed to download any artifacts
// needed for kernel provisioning.
type DownloadArtifactsState struct {
	service *service.KernelProvisionService
}

func NewDownloadArtifactsState(service *service.KernelProvisionService) common_utils.ServiceState {
	return &DownloadArtifactsState{service: service}
}

func (s *DownloadArtifactsState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	cacheClient, err := s.createCacheClient()
	if err != nil {
		return nil, api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED, err
	}
	for _, pi := range s.service.KPRequest.GetPartitionImages() {
		localPath, err := cacheClient.DownloadABArtifactByStoragePath(pi.GetImagePath())
		if err != nil {
			return nil, api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED, fmt.Errorf("downloading kernel prebuilts: %w", err)
		}
		log.Printf("Downloaded artifact '%s' to container at path '%s'", pi, localPath)
		s.service.LocalArtifactPaths[pi.GetImagePath().GetPath()] = localPath
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *DownloadArtifactsState) createCacheClient() (*cache.Client, error) {
	cacheServerAddr, err := cache.IPEndpointToHostPort(s.service.DUT.GetCacheServer().GetAddress())
	if err != nil {
		return nil, fmt.Errorf("invalid cache server address: %w", err)
	}
	cacheURL := url.URL{Scheme: "http", Host: cacheServerAddr}
	return cache.NewClient(cacheURL)
}

func (s *DownloadArtifactsState) Next() common_utils.ServiceState {
	return NewKernelProvisionProvisionState(s.service)
}

func (s *DownloadArtifactsState) Name() string {
	return "Kernel-Provision Download Artifacts"
}
