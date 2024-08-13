// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

func isCTRUp(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start adb container").Err()
	}
	if ctrInfo.IsUp() {
		log.Infof(ctx, "CTR is up")
		return nil
	}
	return errors.Reason("is CTR up: reported as not connected").Err()
}

func init() {
	execs.Register("ctr_is_up", isCTRUp)
}
