// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// verifyRootfsIsUsingVerity confirms that rootfs is using verity
func verifyRootfsIsUsingVerity(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	rfsv, err := cros.IsRootFSVerityEnabled(ctx, r)
	if err != nil {
		return errors.Annotate(err, "failed to get rootfs info").Err()
	}
	if !rfsv {
		return errors.Reason("rootfs not on fs-verity").Err()
	}
	return nil
}

func init() {
	execs.Register("cros_verify_rootfs_verity", verifyRootfsIsUsingVerity)
}
