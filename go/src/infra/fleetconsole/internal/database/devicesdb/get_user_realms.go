// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"

	"go.chromium.org/luci/server/auth"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

func GetUserRealms(ctx context.Context, cloudProject string) ([]string, error) {
	// When running locally auth.QueryRealms is not implemented,
	// this defaults to showing all devices
	if cloudProject == "" {
		return nil, nil
	}

	realms, err := auth.QueryRealms(ctx, ufsUtil.InventoriesList, "", nil)
	if err != nil {
		return nil, err
	}
	realms = append(realms, "") // Devices with no realms are visible to everyone
	return realms, nil
}
