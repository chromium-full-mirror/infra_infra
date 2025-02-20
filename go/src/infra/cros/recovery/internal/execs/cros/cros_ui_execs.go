// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// stopSartUIExec checks the command "stop ui" won't crash the DUT.
//
// We run 'stop ui' in AU and provision. We found some bad images broke
// this command and then broke all the provision of all following test. We add
// this verifier to ensure it works and will trigger reimaging to a good
// version if it fails.
func stopSartUIExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetDut().Name)
	out, err := run(ctx, info.GetExecTimeout(), "stop ui && start ui")
	if execs.SSHErrorLinuxTimeout.In(err) {
		// Timeout Running the command.
		log.Debugf(ctx, "Got timeout when stop ui/start ui. DUT might crash.")
		return errors.Annotate(err, "stop start ui").Err()
	} else if err != nil {
		log.Debugf(ctx, "Not Critical: %s", err)
	}
	log.Debugf(ctx, "Stdout: %s", out)
	return nil
}

// startUIExec executes the start of UI on the DUT.
func startUIExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetDut().Name)
	out, err := run(ctx, info.GetExecTimeout(), "start ui")
	if err != nil {
		return errors.Annotate(err, "start UI exec").Err()
	}
	log.Debugf(ctx, "Stdout: %q", out)
	return nil
}

func init() {
	execs.Register("cros_stop_start_ui", stopSartUIExec)
	execs.Register("cros_start_ui", startUIExec)
}
