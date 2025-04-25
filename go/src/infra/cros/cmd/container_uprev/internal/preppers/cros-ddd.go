// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package preppers

import (
	"context"

	"go.chromium.org/infra/cros/cmd/container_uprev/internal/vars"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/devscripts/devhelpers"
)

func PrepareCrosDDD(ctx context.Context, dir string) error {
	return devhelpers.PrepareCrosDDD(ctx, dir, vars.LoginMode)
}
