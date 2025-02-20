// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// IsRootFSVerityEnabled checks if rootfs is setup using fs-verity
func IsRootFSVerityEnabled(ctx context.Context, r components.Runner) (bool, error) {
	// Check if rootdev outputs /dev/dm
	output, err := r(ctx, time.Minute, "rootdev")
	if err != nil {
		return false, errors.Annotate(err, "failed to run rootdev").Err()
	}
	if strings.HasPrefix(output, "/dev/dm") {
		return true, nil
	}
	return false, nil
}
