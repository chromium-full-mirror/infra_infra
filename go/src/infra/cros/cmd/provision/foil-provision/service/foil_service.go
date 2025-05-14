// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Container for the FoilProvision state machine
package service

import (
	"fmt"
	"net/url"
	"regexp"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/common-utils/cache"
	cross_over "go.chromium.org/infra/cros/cmd/provision/common-utils/cross-over"
	"go.chromium.org/infra/cros/cmd/provision/common-utils/metadata"
)

var buildIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`build_details/(P?[0-9]+)/`),
	regexp.MustCompile(`artifacts_list/(P?[0-9]+)/`)}

// FoilService inherits ServiceInterface
type FoilService struct {
	Connection      commonutils.ServiceAdapterInterface
	MachineMetadata metadata.MachineMetadata
	// ImagePath is the android build explorer path.
	// example1: android-build/build_explorer/build_details/P78687640/brya-trunk_staging-userdebug/android-desktop-ota-packages.zip
	// example2: android-build/build_explorer/artifacts_list/12330924/brya-trunk_staging-userdebug/brya-ota-12330924.zip
	ImagePath        *conf.StoragePath
	OverwritePayload *conf.StoragePath
	SkipUpdate       bool
	QuickResetDevice bool
	DutIp            string
	UpdateEnginePid  string
	CurrentBuild     string
	TargetBuild      string
	CacheServerURL   url.URL
	DutClient        api.DutServiceClient
	ServoNexusAddr   string
	Dut              *labapi.Dut
	Req              *api.InstallRequest
	CrossOver        bool
	Params           *cross_over.CrossOverParameters
}

func NewFoilService(dut *labapi.Dut, req *api.InstallRequest, dutClient api.DutServiceClient, servoNexusAddr string) (*FoilService, error) {
	cacheServerAddr, err := cache.IPEndpointToHostPort(dut.GetCacheServer().GetAddress())
	if err != nil {
		return nil, fmt.Errorf("invalid cache server address %w", err)
	}
	cacheURL := url.URL{Scheme: "http", Host: cacheServerAddr}
	// TODO: Verify that the req.ImagePath.HostType is Android_build.
	imagePath := req.GetImagePath().GetPath()
	build, err := ExtractBuildID(imagePath)
	if err != nil {
		return nil, err
	}
	return &FoilService{
		DutIp:          dut.GetChromeos().GetSsh().GetAddress(),
		TargetBuild:    build,
		CacheServerURL: cacheURL,
		ImagePath: &conf.StoragePath{
			Path: imagePath,
		},
		DutClient:      dutClient,
		ServoNexusAddr: servoNexusAddr,
		Dut:            dut,
		Req:            req,
	}, nil
}

// ExtractBuildID parses the Android build ID out from an Android image path.
func ExtractBuildID(imagePath string) (string, error) {
	for _, re := range buildIDPatterns {
		matches := re.FindStringSubmatch(imagePath)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("could not extract build ID from %s", imagePath)
}

// CleanupOnFailure is called if one of service's states fails to Execute() and
// should clean up the temporary files, and undo the execution, if feasible.
func (c *FoilService) CleanupOnFailure(states []commonutils.ServiceState, executionErr error) error {
	// TODO: evaluate whether cleanup is needed.
	return nil
}
