// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifirouter

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
)

func fetchUbuntuSystemProductName(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostUbuntuRouterController(ctx, info)
	if err != nil {
		return err
	}
	if err := c.FetchSystemProductName(ctx); err != nil {
		return errors.Annotate(err, "fetch Ubuntu system product name").Err()
	}
	return nil
}

func fetchUbuntuNetworkControllerName(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostUbuntuRouterController(ctx, info)
	if err != nil {
		return err
	}
	if err := c.FetchNetworkControllerName(ctx); err != nil {
		return errors.Annotate(err, "fetch Ubuntu network controller name").Err()
	}
	return nil
}

func init() {
	execs.Register("wifi_router_ubuntu_fetch_system_product_name", fetchUbuntuSystemProductName)
	execs.Register("wifi_router_ubuntu_fetch_network_controller_name", fetchUbuntuNetworkControllerName)
}
