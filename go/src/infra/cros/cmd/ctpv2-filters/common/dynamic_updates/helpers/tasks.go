// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package helpers

import (
	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"

	commonlib "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

// DefaultProvisionStartUpRequest outlines the default values
// typically required for starting up a provision task.
func DefaultProvisionStartUpRequest(deviceId *common.DeviceIdentifier) *interfaces.ProvisionTaskStartUpRequest {
	servoTaskIdentifier := common.NewTaskIdentifier(common.ServoNexus).AddDeviceId(deviceId)
	return &interfaces.ProvisionTaskStartUpRequest{
		DynamicInputs: map[string]string{
			"dut":            deviceId.GetDevice("dut"),
			"dutServer":      deviceId.GetCrosDutServer(),
			"servoNexusAddr": servoTaskIdentifier.Id,
		},
	}
}

// DefaultProvisionStartUpRequestForVM outlines the default values for starting up a provision task to vmlab ChromeOS.
func DefaultProvisionStartUpRequestForVM(deviceId *common.DeviceIdentifier) *interfaces.ProvisionTaskStartUpRequest {
	return &interfaces.ProvisionTaskStartUpRequest{
		DynamicInputs: map[string]string{
			"dut":                             deviceId.GetDevice("dut"),
			"dutServer":                       deviceId.GetCrosDutServer(),
			"dut.cacheServer.address":         commonlib.CacheServer,
			"dut.cacheServer.address.address": commonlib.HostIP,
		},
	}
}

// DefaultCrosProvisionInstallRequest sets up a cros-provision request
// with an empty metadata field, solely passing along the installPath.
func DefaultCrosProvisionInstallRequest(installPath string) *interfaces.ProvisionTaskInstallRequest {
	crosProvisionMetadata, _ := anypb.New(&api.CrOSProvisionMetadata{})
	return &interfaces.ProvisionTaskInstallRequest{
		StaticImagePath: &_go.StoragePath{
			HostType: _go.StoragePath_GS,
			Path:     installPath,
		},
		StaticMetadata: crosProvisionMetadata,
	}
}

// DefaultAndroidProvisionInstallRequest sets up a android-provision request.
func DefaultAndroidProvisionInstallRequest() *interfaces.ProvisionTaskInstallRequest {
	androidProvisionMetadata, _ := anypb.New(&api.AndroidProvisionRequestMetadata{
		CipdPackages: []*api.CIPDPackage{
			{
				VersionOneof: &api.CIPDPackage_Ref{
					Ref: "latest_stable",
				},
				AndroidPackage: api.AndroidPackage_GMS_CORE,
			},
		},
	})
	return &interfaces.ProvisionTaskInstallRequest{
		StaticMetadata: androidProvisionMetadata,
	}
}
