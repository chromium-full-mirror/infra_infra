// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifirouter

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
)

func fetchAsusWrtModelExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostAsusWrtRouterController(ctx, info)
	if err != nil {
		return err
	}
	if err := c.FetchAsusModel(ctx); err != nil {
		return errors.Annotate(err, "failed to fetch Asus model from device").Err()
	}
	return nil
}

func init() {
	execs.Register("wifi_router_asuswrt_fetch_model", fetchAsusWrtModelExec)
}
