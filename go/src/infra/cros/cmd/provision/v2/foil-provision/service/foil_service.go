// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Container for the FoilProvision state machine
package service

import (
	"fmt"
	"net/url"
	"regexp"

	api1 "go.chromium.org/chromiumos/config/go/test/lab/api"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	cross_over "go.chromium.org/chromiumos/test/provision/v2/common-utils/cross-over"
	"go.chromium.org/chromiumos/test/provision/v2/common-utils/metadata"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
)

var buildIdPatterns = []*regexp.Regexp{
	regexp.MustCompile(`build_details/(P?[0-9]+)/`),
	regexp.MustCompile(`artifacts_list/(P?[0-9]+)/`)}

// FoilService inherits ServiceInterface
type FoilService struct {
	Connection      common_utils.ServiceAdapterInterface
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
	CacheServerUrl   url.URL
	DutClient        api.DutServiceClient
	ServoNexusAddr   string
	Dut              *lab_api.Dut
	Req              *api.InstallRequest
	CrossOver        bool
	Params           *cross_over.CrossOverParameters
}

func NewFoilService(dut *lab_api.Dut, req *api.InstallRequest, dutClient api.DutServiceClient, servoNexusAddr string) (*FoilService, error) {
	cacheServerAddr, err := ipEndpointToHostPort(dut.GetCacheServer().GetAddress())
	if err != nil {
		return nil, fmt.Errorf("invalid cache server address %v", err)
	}
	var cacheUrl url.URL
	cacheUrl.Scheme = "http"
	cacheUrl.Host = cacheServerAddr
	// TODO: Verify that the req.ImagePath.HostType is Android_build.
	imagePath := req.GetImagePath().GetPath()
	build, err := TargetBuild(imagePath)
	if err != nil {
		return nil, err
	}
	return &FoilService{
		DutIp:          dut.GetChromeos().GetSsh().GetAddress(),
		TargetBuild:    build,
		CacheServerUrl: cacheUrl,
		ImagePath: &conf.StoragePath{
			Path: imagePath,
		},
		DutClient:      dutClient,
		ServoNexusAddr: servoNexusAddr,
		Dut:            dut,
		Req:            req,
	}, nil
}

func TargetBuild(imagePath string) (string, error) {
	for _, re := range buildIdPatterns {
		matches := re.FindStringSubmatch(imagePath)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("could not extract buildId from %s", imagePath)
}

// CleanupOnFailure is called if one of service's states fails to Execute() and
// should clean up the temporary files, and undo the execution, if feasible.
func (c *FoilService) CleanupOnFailure(states []common_utils.ServiceState, executionErr error) error {
	// TODO: evaluate whether cleanup is needed.
	return nil
}

func ipEndpointToHostPort(i *api1.IpEndpoint) (string, error) {
	if len(i.GetAddress()) == 0 {
		return "", fmt.Errorf("IpEndpoint missing address")
	}
	if i.GetPort() == 0 {
		return "", fmt.Errorf("IpEndpoint missing port")
	}
	return fmt.Sprintf("%v:%v", i.GetAddress(), i.GetPort()), nil
}
