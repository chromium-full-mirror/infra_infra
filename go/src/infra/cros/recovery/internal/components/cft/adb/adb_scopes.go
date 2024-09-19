// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package adb contains methods to work with an ADB-base container.
package adb

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/scopes"
	"infra/cros/recovery/tlw"
)

// ToScope puts ADB client to context scope.
func ToScope(ctx context.Context, dut *tlw.Dut, client api.ADBServiceClient) error {
	if dut == nil {
		return errors.Reason("adb client to scopes: dut is not provided").Err()
	}
	if client == nil {
		return errors.Reason("adb client to scopes: client is not provided").Err()
	}
	containerName := cft.ADBName(dut)
	scopes.PutConfigParam(ctx, containerName, client)
	if _, err := FromScope(ctx, dut); err != nil {
		return errors.Annotate(err, "adb client to scopes").Err()
	}
	log.Debugf(ctx, "ADB client saved to the scope context!")
	return nil
}

// FromScope reads ADB client from context scope.
func FromScope(ctx context.Context, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("adb client from scopes: dut is not provided").Err()
	}
	containerName := cft.ADBName(dut)
	if v, ok := scopes.ReadConfigParam(ctx, containerName); ok {
		if v != nil {
			if c, ok := v.(api.ADBServiceClient); ok && c != nil {
				return c, nil
			}
		}
	}
	return nil, errors.Reason("adb client from scopes: not found").Err()
}
