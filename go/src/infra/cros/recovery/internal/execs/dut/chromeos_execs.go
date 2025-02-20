// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// hasDutBoardActionExec verifies that DUT provides board name.
func hasDutBoardActionExec(ctx context.Context, info *execs.ExecInfo) error {
	b := info.GetChromeos().GetBoard()
	log.Debugf(ctx, "DUT board name: %q", b)
	if b != "" {
		return nil
	}
	return errors.Reason("dut board name is empty").Err()
}

// hasDutModelActionExec verifies that DUT provides model name.
func hasDutModelActionExec(ctx context.Context, info *execs.ExecInfo) error {
	m := info.GetChromeos().GetModel()
	log.Debugf(ctx, "DUT model name: %q", m)
	if m != "" {
		return nil
	}
	return errors.Reason("dut model name is empty").Err()
}

// dutServolessExec verifies that setup is servoless.
func dutServolessExec(ctx context.Context, info *execs.ExecInfo) error {
	if s := info.GetChromeos().GetServo(); s.GetName() == "" && s.GetContainerName() == "" {
		log.Debugf(ctx, "DUT servoless confirmed!")
		return nil
	}
	return errors.Reason("dut is servoless").Err()
}

// hasDutDeviceSkuActionExec verifies that DUT has the device sku label.
func hasDutDeviceSkuActionExec(ctx context.Context, info *execs.ExecInfo) error {
	deviceSku := info.GetChromeos().GetDeviceSku()
	log.Debugf(ctx, "Device sku: %q.", deviceSku)
	if deviceSku == "" {
		return errors.Reason("dut device sku label is empty").Err()
	}
	return nil
}

// dutCheckModelExec checks whether the model name for the current DUT
// matches any of the values specified in config. It returns an error
// based on the directive in config to invert the results.
func dutCheckModelExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	invertResultsFlag := argsMap.AsBool(ctx, "invert_result", false)
	model := info.GetChromeos().GetModel()
	for _, v := range argsMap.AsStringSlice(ctx, "string_values", nil) {
		v = strings.TrimSpace(v)
		if strings.EqualFold(v, model) {
			msg := fmt.Sprintf("DUT Model %s found in the list of models in config", model)
			log.Debugf(ctx, "Dut Check Model Exec :%s.", msg)
			if invertResultsFlag {
				return errors.Reason("dut check model exec: %s", msg).Err()
			}
			return nil
		}
	}
	msg := "No matching model found"
	log.Debugf(ctx, "Dut Check Model Exec: %s", msg)
	if invertResultsFlag {
		return nil
	}
	return errors.Reason("dut check model exec: %s", msg).Err()
}

// dutCheckBoardExec checks whether the board name for the current DUT
// matches any of the values specified in config. It returns an error
// based on the directive in config to invert the results.
func dutCheckBoardExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	invertResultsFlag := argsMap.AsBool(ctx, "invert_result", false)
	board := info.GetChromeos().GetBoard()
	for _, v := range argsMap.AsStringSlice(ctx, "string_values", nil) {
		v = strings.TrimSpace(v)
		if strings.EqualFold(v, board) {
			msg := fmt.Sprintf("DUT Board %s found in the list of boards in config", board)
			log.Debugf(ctx, "Dut Check Board Exec :%s.", msg)
			if invertResultsFlag {
				return errors.Reason("dut check board exec: %s", msg).Err()
			}
			return nil
		}
	}
	msg := "No matching board found"
	log.Debugf(ctx, "Dut Check Board Exec: %s", msg)
	if invertResultsFlag {
		return nil
	}
	return errors.Reason("dut check board exec: %s", msg).Err()
}

// servoVerifySerialNumberExec verifies that the servo host attached
// to the DUT has a serial number configured.
func servoVerifySerialNumberExec(ctx context.Context, info *execs.ExecInfo) error {
	if sn := info.GetChromeos().GetServo().GetSerialNumber(); sn != "" {
		log.Debugf(ctx, "Servo Verify Serial Number : %q", sn)
		return nil
	}
	return errors.Reason("servo verify serial number: serial number is not available").Err()
}

// servoHostPresentExec verifies that servo host present under DUT.
func servoHostPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetServo() == nil {
		return errors.Reason("servo host present: data is not present").Err()
	}
	return nil
}

// dutInAudioBoxExec checks if DUT is in audio box.
func dutInAudioBoxExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetAudio().GetInBox() {
		return errors.Reason("is audio box: is not in audio-box").Err()
	}
	return nil
}

// hasBatteryExec checks if DUT is expected to have a battery.
func hasBatteryExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetBattery() == nil {
		return errors.Reason("has battery: data is not present").Err()
	}
	return nil
}

// matchBatteryStateExec match statet of battery with expected.
//
// Please provide expected state by action args [state:your-state]
func matchBatteryStateExec(ctx context.Context, info *execs.ExecInfo) error {
	battery := info.GetChromeos().GetBattery()
	if battery == nil {
		return errors.Reason("match battery state: data is not present in dut info").Err()
	}
	actionMap := info.GetActionArgs(ctx)
	state := strings.ToUpper(actionMap.AsString(ctx, "state", ""))
	if state == "" {
		return errors.Reason("match battery state: state is not provided").Err()
	}
	s, ok := tlw.HardwareState_value[state]
	if !ok {
		return errors.Reason("match battery state: state %q is invalid", state).Err()
	}
	currentState := battery.GetState()
	if s != int32(currentState) {
		return errors.Reason("match battery state: current state %s does not match expected state %q", currentState.String(), s).Err()
	}
	return nil
}

// hasDutHwidExec verifies that DUT has an HWID available.
func hasDutHwidExec(ctx context.Context, info *execs.ExecInfo) error {
	if h := info.GetChromeos().GetHwid(); h != "" {
		log.Debugf(ctx, "DUT Hwid: %q", h)
		return nil
	}
	return errors.Reason("dut Hwid is empty").Err()
}

// hasDutSerialNumberExec verifies that DUT has a serial number
// available.
func hasDutSerialNumberExec(ctx context.Context, info *execs.ExecInfo) error {
	if sn := info.GetChromeos().GetSerialNumber(); sn != "" {
		log.Debugf(ctx, "DUT Serial Number : %q", sn)
		return nil
	}
	return errors.Reason("dut serial number is empty").Err()
}

// hasCr50PhaseExec verifies whether the Cr50 firmware is present on
// the DUT.
func hasCr50PhaseExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetCr50Phase() == tlw.ChromeOS_CR50_PHASE_UNSPECIFIED {
		return errors.Reason("has Cr50 phase: Cr50 firmware phase could not be determined").Err()
	}
	return nil
}

// dutServoStateMandatesManualRepairExec checks whether the state of
// the servo on the DUT makes it imperative to set the state of the
// DUT to need manual repair.
func dutServoStateMandatesManualRepairExec(ctx context.Context, info *execs.ExecInfo) error {
	servoStates := info.GetActionArgs(ctx).AsStringSlice(ctx, "servo_states", nil)
	if servoStates == nil {
		log.Debugf(ctx, "DUT Servo State Mandates Manual Repair: no states mentioned in the action, don't have enough information to check whether servo state requires manual repair on the DUT.")
		return nil
	}
	// Assume this will always be non-nil because actions using this exec will be conditioned in the config on availability of ServoHost.
	for _, s := range servoStates {
		if state, ok := tlw.ServoHost_State_value[s]; ok && tlw.ServoHost_State(state) == info.GetChromeos().GetServo().GetState() {
			log.Debugf(ctx, "DUT Servo State Mandates Manual Repair")
			return nil
		}
	}
	return errors.Reason("dut servo state mandates manual repair: manual repair not required").Err()
}

func init() {
	// TODO(gregorynisbet): rename dut to chromeos to mark as data read from ChromeOS.
	execs.Register("dut_servo_host_present", servoHostPresentExec)
	execs.Register("dut_has_board_name", hasDutBoardActionExec)
	execs.Register("dut_has_model_name", hasDutModelActionExec)
	execs.Register("dut_has_device_sku", hasDutDeviceSkuActionExec)
	execs.Register("dut_check_model", dutCheckModelExec)
	execs.Register("dut_check_board", dutCheckBoardExec)
	execs.Register("dut_servoless", dutServolessExec)
	execs.Register("dut_is_in_audio_box", dutInAudioBoxExec)
	execs.Register("dut_servo_has_serial", servoVerifySerialNumberExec)
	execs.Register("dut_has_battery", hasBatteryExec)
	execs.Register("dut_match_battery_state", matchBatteryStateExec)
	execs.Register("dut_has_hwid", hasDutHwidExec)
	execs.Register("dut_has_serial_number", hasDutSerialNumberExec)
	execs.Register("dut_has_cr50", hasCr50PhaseExec)
	execs.Register("dut_servo_state_required_manual_attention", dutServoStateMandatesManualRepairExec)
}
