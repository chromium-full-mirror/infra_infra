// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/scopes"
	"go.chromium.org/infra/cros/recovery/tlw"
	ufsProto "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// hasDutNameActionExec verifies that DUT provides name.
func hasDutNameActionExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() != nil && info.GetDut().Name != "" {
		log.Debugf(ctx, "DUT name: %q", info.GetDut().Name)
		return nil
	}
	return errors.Reason("dut name is empty").Err()
}

// regexNameMatchExec checks if name match to provided regex.
func regexNameMatchExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	d := info.GetDut()
	if d == nil {
		return errors.Reason("regex name match: DUT not found").Err()
	}
	regex := actionMap.AsString(ctx, "regex", "")
	if regex == "" {
		return errors.Reason("regex name match: regex is empty").Err()
	}
	m, err := regexp.MatchString(regex, d.Name)
	if err != nil {
		return errors.Annotate(err, "regex name match").Err()
	}
	if !m {
		return errors.Reason("regex name match: not match").Err()
	}
	return nil
}

// setDutStateExec sets the state of the DUT to the value passed in
// the action arguments.
func setDutStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToLower(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("set dut state: state is not provided").Err()
	}
	state := dutstate.State(newState)
	if dutstate.ConvertToUFSState(state) == ufsProto.State_STATE_UNSPECIFIED {
		return errors.Reason("set dut state: unsupported state %q", newState).Err()
	}
	log.Debugf(ctx, "Old DUT state: %s", info.GetDut().State)
	info.GetDut().State = state
	log.Infof(ctx, "New DUT state: %s", newState)
	return nil
}

// resetDutStateReasonExec reset dut-state-reason for DUT.
func resetDutStateReasonExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() == nil {
		log.Debugf(ctx, "DUT is not present")
	} else {
		info.GetDut().DutStateReason = tlw.DutStateReasonEmpty
	}
	return nil
}

// isDutStateReasonEmptyExec checks if dut-state-reason is empty.
func isDutStateReasonEmptyExec(ctx context.Context, info *execs.ExecInfo) error {
	if d := info.GetDut(); d != nil {
		if len(d.DutStateReason) == 0 {
			return nil
		}
		return errors.Reason("dut state reason is empty: reason %q specified", d.DutStateReason).Err()
	}
	return errors.Reason("dut state reason is empty: dut not found").Err()
}

// setDutStateReasonFromTaskTagsExec set data from task tags as a dut-state-reason.
func setDutStateReasonFromTaskTagsExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() == nil {
		return errors.Reason("set dut state reason from task tags: dut not found").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	tagName := actionArgs.AsString(ctx, "tag_name", "")
	if tagName == "" {
		return errors.Reason("set dut state reason from task tags: tag is not specified or empty").Err()
	}
	rawTags, ok := scopes.GetParam(ctx, scopes.ParamKeySwarmingTaskTags)
	if !ok {
		log.Debugf(ctx, "Set DUT state reason from task tags: scope is empty")
		return nil
	}
	tags, ok := rawTags.(map[string]string)
	if !ok {
		return errors.Reason("set dut state reason from task tags: task tags not found").Err()
	}
	for k, v := range tags {
		if k == tagName {
			if cv := strings.TrimSpace(v); cv != "" {
				log.Infof(ctx, "New DUT state reason: %q", cv)
				info.GetDut().DutStateReason = tlw.DutStateReason(cv)
				break
			}
		}
	}
	return nil
}

// setDutStateReasonExec set dut-state-reason for DUT.
//
// By default `allow_override` flag is set to true.
func setDutStateReasonExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	allowOverride := args.AsBool(ctx, "allow_override", true)
	reason := args.AsString(ctx, "reason", "")
	if info.GetDut().DutStateReason.NotEmpty() && !allowOverride {
		log.Debugf(ctx, "DUT state reason already specified")
		return nil
	} else {
		info.GetDut().DutStateReason = tlw.DutStateReason(reason)
	}
	return nil
}

func init() {
	execs.Register("dut_has_name", hasDutNameActionExec)
	execs.Register("dut_regex_name_match", regexNameMatchExec)
	execs.Register("dut_set_state", setDutStateExec)
	execs.Register("dut_set_state_reason", setDutStateReasonExec)
	execs.Register("dut_reset_state_reason", resetDutStateReasonExec)
	execs.Register("dut_state_reason_is_empty", isDutStateReasonEmptyExec)
	execs.Register("dut_state_reason_set_from_tags", setDutStateReasonFromTaskTagsExec)
}
