// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// matchStatefulWithOSExec validates that the stateful and OS are the correct pair that works.
func matchStatefulWithOSExec(ctx context.Context, info *execs.ExecInfo) error {
	// TODO(b:232147693): Implement target logic. More detail in the bug.
	run := info.DefaultRunner()
	_, err := run(ctx, time.Minute, "true")
	return errors.WrapIf(err, "match stateful with OS")
}

func init() {
	execs.Register("cros_match_statefull_with_os", matchStatefulWithOSExec)
}
