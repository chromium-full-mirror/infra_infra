// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servonexus contains methods to work with an servo-nexus container.
package servonexus

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/scopes"
	"infra/cros/recovery/tlw"
)

// ToScope puts servo-nexus client to context scope.
func ToScope(ctx context.Context, dut *tlw.Dut, client api.ServodServiceClient) error {
	if dut == nil {
		return errors.Reason("servo-nexus client to scopes: dut is not provided").Err()
	}
	if client == nil {
		return errors.Reason("servo-nexus client to scopes: client is not provided").Err()
	}
	containerName := cft.ServoNexusName(dut)
	scopes.PutConfigParam(ctx, containerName, client)
	if _, err := FromScope(ctx, dut); err != nil {
		return errors.Annotate(err, "servo-nexus client to scopes").Err()
	}
	log.Debugf(ctx, "servo-nexus client saved to the scope context!")
	return nil
}

// FromScope reads servo-nexus client from context scope.
func FromScope(ctx context.Context, dut *tlw.Dut) (api.ServodServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("servo-nexus client from scopes: dut is not provided").Err()
	}
	containerName := cft.ServoNexusName(dut)
	if v, ok := scopes.ReadConfigParam(ctx, containerName); ok {
		if v != nil {
			if c, ok := v.(api.ServodServiceClient); ok && c != nil {
				return c, nil
			}
		}
	}
	return nil, errors.Reason("servo-nexus client from scopes: not found").Err()
}
