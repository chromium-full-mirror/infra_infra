// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cft

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/lab/api/passport"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/scopes"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// ClientToScope puts cft container client to context scope.
func ClientToScope[C comparable](ctx context.Context, dut *tlw.Dut, client C, containerName string) error {
	if dut == nil {
		return errors.Reason("client to scopes: dut is not provided").Err()
	}
	var zero C
	if client == zero {
		return errors.Reason("client to scopes: client is not provided").Err()
	}
	if containerName == "" {
		return errors.Reason("client to scopes: container name is not provided").Err()
	}
	scopes.PutConfigParam(ctx, clientScopeKey(containerName), client)
	if _, err := ClientFromScope[C](ctx, dut, containerName); err != nil {
		return errors.Annotate(err, "client to scopes").Err()
	}
	log.Debugf(ctx, "Client of %q saved to the scope context!", containerName)
	return nil
}

// ClientFromScope reads cft container client from context scope.
func ClientFromScope[C comparable](ctx context.Context, dut *tlw.Dut, containerName string) (C, error) {
	var empty C
	if dut == nil {
		return empty, errors.Reason("client from scopes: dut is not provided").Err()
	}
	if containerName == "" {
		return empty, errors.Reason("client from scopes: container name is not provided").Err()
	}
	if v, ok := scopes.ReadConfigParam(ctx, clientScopeKey(containerName)); ok {
		if v != nil {
			if c, ok := v.(C); ok && c != empty {
				return c, nil
			}
		}
	}
	return empty, errors.Reason("client from scopes: not found").Err()
}

// ADBClientFromScope read adb service client from scope.
func ADBClientFromScope(ctx context.Context, dut *tlw.Dut) (api.ADBServiceClient, error) {
	return ClientFromScope[api.ADBServiceClient](ctx, dut, ADBName(dut))
}

// ServoClientFromScope read servo-nexus service client from scope.
func ServoClientFromScope(ctx context.Context, dut *tlw.Dut) (api.ServodServiceClient, error) {
	return ClientFromScope[api.ServodServiceClient](ctx, dut, ServoNexusName(dut))
}

// FoilProvisionClientFromScope read foil-provision service client from scope.
func FoilProvisionClientFromScope(ctx context.Context, dut *tlw.Dut) (api.GenericProvisionServiceClient, error) {
	return ClientFromScope[api.GenericProvisionServiceClient](ctx, dut, FoilProvisionName(dut))
}

// PassportSwitchClientFromScope fetches the switch service client from the current scope.
func PassportSwitchClientFromScope(ctx context.Context, dut *tlw.Dut) (passport.SwitchServiceClient, error) {
	return ClientFromScope[passport.SwitchServiceClient](ctx, dut, PassportSwitchName(dut))
}
