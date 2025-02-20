// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

const (
	androidProvisionRequestMetadata = "type.googleapis.com/chromiumos.test.api.AndroidProvisionRequestMetadata"
)

func (trv2cfg *Trv2ExecutionConfig) isAndroidProvisioningRequired(ctx context.Context) bool {
	switch sk := trv2cfg.CmdExecutionConfig.StateKeeper.(type) {
	case *data.HwTestStateKeeper:
		return isAndroidProvisioningRequiredFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		return isAndroidProvisioningRequiredFromHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	}
	return false
}

func isAndroidProvisioningRequiredFromHwTestStateKeeper(ctx context.Context,
	sk *data.HwTestStateKeeper) bool {

	companionDuts := sk.CftTestRequest.GetCompanionDuts()
	if companionDuts == nil {
		return false
	}
	for _, companionDut := range companionDuts {
		provisionMetadata := companionDut.GetProvisionState().GetProvisionMetadata()
		if provisionMetadata == nil {
			continue
		}
		if provisionMetadata.TypeUrl != androidProvisionRequestMetadata {
			continue
		}
		var androidProvisionRequestMetadata api.AndroidProvisionRequestMetadata
		err := provisionMetadata.UnmarshalTo(&androidProvisionRequestMetadata)
		if err != nil {
			logging.Infof(ctx, "error during isAndroidProvisioningRequired: %s", err)
			return false
		}
		if androidProvisionRequestMetadata.GetAndroidOsImage() != nil || androidProvisionRequestMetadata.GetCipdPackages() != nil {
			return true
		}
	}
	return false
}
