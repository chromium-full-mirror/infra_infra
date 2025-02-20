// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// setStateBrokenExec sets state as BROKEN.
func setStateBrokenExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetChromeos().GetChameleon().State = tlw.Chameleon_BROKEN
	return nil
}

// setStateWorkingExec sets state as WORKING.
func setStateWorkingExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetChromeos().GetChameleon().State = tlw.Chameleon_WORKING
	return nil
}

// setStateNotApplicableExec sets state as NOT_APPLICABLE.
func setStateNotApplicableExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetChromeos().GetChameleon().State = tlw.Chameleon_NOT_APPLICABLE
	return nil
}

// chameleonNotPresentExec check if chameleon is absent.
// return error if chameleon exists.
func chameleonNotPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetChameleon().GetName() != "" {
		return errors.Reason("chameleon not present: chameleon hostname exist").Err()
	}
	return nil
}

// chameleonCheckAudioboxJackpluggerExec checks the state of AudioBoxJackPlugger
// this function will set the state according to the result executed by runner
// it will always return nil to prevent affecting chameleon state
func chameleonCheckAudioboxJackpluggerExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetChameleon() == nil {
		log.Debugf(ctx, "chameleon is not found.")
		return errors.Reason("chameleon is not found.").Err()
	}
	if !info.GetChromeos().GetTestbedCapability().GetAudiojack() {
		log.Debugf(ctx, "testbed has no audiobox jackplugger - Mark as Not Applicable.")
		info.GetChromeos().GetChameleon().Audioboxjackpluggerstate = tlw.Chameleon_AUDIOBOX_JACKPLUGGER_NOT_APPLICABLE
		return nil
	}
	if !info.GetChromeos().GetAudio().GetInBox() {
		log.Debugf(ctx, "chameleon is not in AudioBox - Not Applicable to jack plugger.")
		info.GetChromeos().GetChameleon().Audioboxjackpluggerstate = tlw.Chameleon_AUDIOBOX_JACKPLUGGER_NOT_APPLICABLE
		return nil
	}
	runner := info.NewRunner(info.GetChromeos().GetChameleon().GetName())
	output, err := runner(ctx, time.Minute, "check_audiobox_jackplugger")

	if output == "WORKING" {
		info.GetChromeos().GetChameleon().Audioboxjackpluggerstate = tlw.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING
	} else {
		info.GetChromeos().GetChameleon().Audioboxjackpluggerstate = tlw.Chameleon_AUDIOBOX_JACKPLUGGER_BROKEN
	}
	if err != nil {
		log.Debugf(ctx, "unable interpret AudioBoxJackPlugger status: %s", err)
		return errors.Reason("unable to interpret AudioBoxJackPlugger status: %s", err).Err()
	}

	return nil
}

func init() {
	execs.Register("chameleon_state_broken", setStateBrokenExec)
	execs.Register("chameleon_state_working", setStateWorkingExec)
	execs.Register("chameleon_state_not_applicable", setStateNotApplicableExec)
	execs.Register("chameleon_not_present", chameleonNotPresentExec)
	execs.Register("chameleon_check_audiobox_jackplugger", chameleonCheckAudioboxJackpluggerExec)
}
