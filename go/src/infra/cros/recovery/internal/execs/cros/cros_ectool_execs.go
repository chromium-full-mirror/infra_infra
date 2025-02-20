// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// resetEcExec resets EC from DUT side to wake CR50 up.
//
// @params: actionArgs should be in the format of:
// Ex: ["wait_timeout:x"]
func resetEcExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// Delay to wait for the ec reset command to be efftive. Default to be 30s.
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 30, time.Second)

	// Reset EC from DUT side.
	if err := cros.RebootECByEcTool(ctx, info.NewRunner(info.GetDut().Name)); err != nil {
		return err
	}
	log.Debugf(ctx, "waiting for %d seconds to let ec reset be effective.", waitTimeout)
	time.Sleep(waitTimeout)
	return nil
}

func init() {
	execs.Register("cros_reset_ec", resetEcExec)
}
