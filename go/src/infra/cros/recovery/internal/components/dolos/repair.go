// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// CallRepair - call doloscmd repair on the host.
func CallRepair(ctx context.Context, run components.Runner, dolosInfo *tlw.Dolos, timeout time.Duration) error {

	const dolosSubCmdRepair = "repair"

	log.Infof(ctx, "Running doloscmd repair.")
	_, err := runCommand(ctx, run, dolosSubCmdRepair, dolosInfo, timeout)
	if err != nil {
		return errors.Annotate(err, "Repair call to doloscmd failed.").Err()
	}

	return nil
}
