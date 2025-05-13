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
	"os"
	"runtime"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/common-utils/cache"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// DownloadArtifactsState represents the tasks needed to download any artifacts
// needed for kernel provisioning.
type DownloadArtifactsState struct {
	service *service.KernelProvisionService
}

func NewDownloadArtifactsState(service *service.KernelProvisionService) commonutils.ServiceState {
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
	if err := s.downloadFastboot(log, cacheClient); err != nil {
		return nil, api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED, fmt.Errorf("downloading fastboot executable: %w", err)
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

// downloadFastboot downloads the fastboot binary from the Android OS build, makes it executable,
// and saves the local path in the KernelProvisionService.
func (s *DownloadArtifactsState) downloadFastboot(log *log.Logger, cacheClient *cache.Client) error {
	// `fastboot` should be an artifact of the same build that produced the OS image.
	buildID, buildTarget, _, err := common.ParseAndroidPath(s.service.OSImagePath.GetPath())
	if err != nil {
		return fmt.Errorf("parsing AL OS image path: %w", err)
	}
	localPath, _, err := cacheClient.DownloadABArtifact(buildID, buildTarget, "fastboot")
	if err != nil {
		return fmt.Errorf("downloading fastboot binary: %w", err)
	}
	log.Printf("Downloaded fastboot binary to local path: %s\n", localPath)
	if err := makeExecutable(localPath); err != nil {
		return fmt.Errorf("making fastboot binary executable: %w", err)
	}
	s.service.FastbootPath = localPath
	return nil
}

// makeExecutable changes a file's mode to be executable by all users.
func makeExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Executable mode is not meaningful in Windows")
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("getting file info for %s: %w", path, err)
	}
	currentMode := fileInfo.Mode()
	newMode := currentMode | 0111
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("changing mode of %s from %v to %v: %w", path, currentMode, newMode, err)
	}
	return nil
}

func (s *DownloadArtifactsState) Next() commonutils.ServiceState {
	return NewKernelProvisionProvisionState(s.service)
}

func (s *DownloadArtifactsState) Name() string {
	return "Kernel-Provision Download Artifacts"
}
