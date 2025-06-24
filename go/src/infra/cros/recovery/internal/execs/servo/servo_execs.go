// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	components_topology "go.chromium.org/infra/cros/recovery/internal/components/servo/topology"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/cros/battery"
	"go.chromium.org/infra/cros/recovery/internal/execs/servo/topology"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// This parameter represents the configuration for minimum number
	// of child servo devices to be verified-for.
	topologyMinChildArg = "min_child"

	// This parameter represents the configuration to control whether
	// the servo topology that read during servo topology verification
	// is persisted for use by other actions.
	persistTopologyArg = "persist_topology"

	// The default value that will be used to drive whether or not the
	// topology needs to be persisted. A value that is passed from the
	// configuration will over-ride this.
	persistTopologyDefaultValue = false

	// The default value that will be used for validating the number
	// of servo children in the servo topology. A value that is passed
	// from the configuration will over-ride this.
	topologyMinChildCountDefaultValue = 1

	// This command, when executed from servo host, checks whether the
	// servod process is responsive.
	servodHostCheckupCmd = "dut-control -p %d root.serialname"

	// This is the threshold voltage values between DUT and servo
	// Bus voltage on ppdut5. Value can be:
	//  - less than 500 - DUT is likely not connected
	//  - between 500 and 4000 - unexpected value
	//  - more than 4000 - DUT is likely connected
	maxPPDut5MVWhenNotConnected = 500
	minPPDut5MVWhenConnected    = 4000
	// File flag created in logs folder to request next servod start
	// use recovery mode by providing argument REC_MODE=1.
	ServodUseRecoveryModeFlag = "servod_use_recovery_mode"
)

// servodInitActionExec init servod options and start servod on servo-host.
func servodInitActionExec(ctx context.Context, info *execs.ExecInfo) error {
	d := info.GetDut()
	if d == nil || d.Name == "" {
		return errors.Reason("init servod: DUT is not specified").Err()
	}
	chromeos := info.GetChromeos()
	if chromeos == nil {
		return errors.Reason("init servod: chromeos is not specified").Err()
	}
	sh := chromeos.GetServo()
	if sh == nil {
		return errors.Reason("init servod: servo-host or servo is not specified").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	useRecoveryMode := actionArgs.AsBool(ctx, "recovery_mode", false)
	if !useRecoveryMode {
		// The request to use recovery mode can be specified by presence of a specific file.
		logRoot := info.GetLogRoot()
		flagPath := filepath.Join(logRoot, ServodUseRecoveryModeFlag)
		// If the call fail we think that file is not exist.
		// The call cannot fail as part of permission issue as file is created under the same user.
		if _, err := os.Stat(flagPath); err == nil {
			useRecoveryMode = true
		}
	}

	o := &tlw.ServodOptions{
		RecoveryMode:  useRecoveryMode,
		DutBoard:      chromeos.GetBoard(),
		DutModel:      chromeos.GetModel(),
		ServodPort:    int32(sh.GetServodPort()),
		ServoSerial:   sh.GetSerialNumber(),
		ServoDual:     false,
		UseCr50Config: false,
	}
	if vs, ok := d.ExtraAttributes[tlw.ExtraAttributeServoSetup]; ok {
		for _, v := range vs {
			if v == tlw.ExtraAttributeServoSetupDual {
				o.ServoDual = true
				break
			}
		}
	}
	if pools, ok := d.ExtraAttributes[tlw.ExtraAttributePools]; ok {
		for _, p := range pools {
			if strings.Contains(p, "faft-cr50") {
				o.UseCr50Config = true
				break
			}
		}
	}
	log.Debugf(ctx, "Servod options: %s", o)
	am := info.GetActionArgs(ctx)
	req := &tlw.InitServodRequest{
		Resource: d.Name,
		Options:  o,
		NoServod: am.AsBool(ctx, "no_servod", false),
	}
	if err := info.GetAccess().InitServod(ctx, req); err != nil {
		return errors.Annotate(err, "init servod").Err()
	}
	return nil
}

func servodStopActionExec(ctx context.Context, info *execs.ExecInfo) error {
	if err := info.GetAccess().StopServod(ctx, info.GetDut().Name); err != nil {
		return errors.Annotate(err, "stop servod").Err()
	}
	return nil
}

func servodCreateFlagToUseRecoveryModeExec(ctx context.Context, info *execs.ExecInfo) error {
	logRoot := info.GetLogRoot()
	if logRoot == "" {
		return errors.Reason("servod create flag to use recovery-mode: log root is not specified").Err()
	}
	flagPath := filepath.Join(logRoot, ServodUseRecoveryModeFlag)
	err := exec.CommandContext(ctx, "touch", flagPath).Run()
	return errors.WrapIf(err, "servod create flag to use recovery-mode")
}

// Verify that the root servo is enumerated/present on the host.
// To force re-read topology please specify `update_topology:true`.
func isRootServoPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("is root servo present: servo is not present as part of dut info").Err()
	}
	runner := info.NewRunner(sh.GetName())
	am := info.GetActionArgs(ctx)
	if am.AsBool(ctx, "update_topology", false) {
		servoTopology, err := topology.RetrieveServoTopology(ctx, runner, sh.GetSerialNumber())
		if err != nil {
			return errors.Annotate(err, "is root servo present exec").Err()
		}
		sh.ServoTopology = servoTopology
	}
	rootServo, err := topology.GetRootServo(ctx, runner, sh.GetSerialNumber())
	if err != nil {
		return errors.Annotate(err, "is root servo present exec").Err()
	}
	if !components_topology.IsItemGood(ctx, rootServo) {
		log.Infof(ctx, "Is Servo Root Present Exec: no good root servo found")
		return errors.Reason("is servo root present exec: no good root servo found").Err()
	}
	log.Infof(ctx, "is servo root present exec: success")
	return nil
}

// Verify that the root servo is enumerated/present on the host.
func servoTopologyUpdateExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo topology update: servo is not present as part of dut info").Err()
	}
	runner := info.NewRunner(sh.GetName())
	servoTopology, err := topology.RetrieveServoTopology(ctx, runner, sh.GetSerialNumber())
	if err != nil {
		return errors.Annotate(err, "servo topology update exec").Err()
	}
	if servoTopology.Root == nil {
		return errors.Reason("servo topology update exec: root servo not found").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	minChildCount := topologyMinChildCountDefaultValue
	persistTopology := persistTopologyDefaultValue
	for k, v := range argsMap {
		log.Debugf(ctx, "Servo Topology Update Exec: k:%q, v:%q", k, v)
		if v != "" {
			// a non-empty value string implies that the corresponding
			// action arg was parsed correctly.
			switch k {
			case topologyMinChildArg:
				// If the configuration contains any min_child parameters,
				// it will be used for validation here. If no such
				// argument is present, we will not conduct any validation
				// of number of child servo based min_child.
				minChildCount, err = strconv.Atoi(v)
				if err != nil {
					return errors.Reason("servo topology update exec: malformed min child config in action arg %q:%q", k, v).Err()
				}
			case persistTopologyArg:
				persistTopology, err = strconv.ParseBool(v)
				if err != nil {
					return errors.Reason("servo topology update exec: malformed update servo config in action arg %q:%q", k, v).Err()
				}
			}
		}
	}
	if len(servoTopology.Children) < minChildCount {
		return errors.Reason("servo topology update exec: expected a min of %d children, found %d", minChildCount, len(servoTopology.Children)).Err()
	}
	if persistTopology {
		// This verified topology will be used in all subsequent
		// action that need the servo topology. This will avoid time
		// with re-fetching the topology.
		sh.ServoTopology = servoTopology
	}
	return nil
}

// Verify that servod is responsive
func servoServodEchoHostExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo servod echo host: servo is not present as part of dut info").Err()
	}
	runner := info.NewRunner(sh.GetName())
	v, err := runner(ctx, time.Minute, fmt.Sprintf(servodHostCheckupCmd, info.NewServod().Port()))
	if err != nil {
		return errors.Annotate(err, "servo servod echo host exec: servod is not responsive for dut-control commands").Err()
	}
	log.Debugf(ctx, "Servo Servod Echo Host Exec: Servod is responsive: %q", v)
	return nil
}

// Verify that the servo firmware is up-to-date.
func servoFirmwareNeedsUpdateExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo firmware needs update: servo is not present as part of dut info").Err()
	}
	runner := info.NewRunner(sh.GetName())
	// The servo topology check should have already been done in an
	// action. The topology determined at that time would have been
	// saved in this data structure if the 'updateServo' argument was
	// passed for that action. We will make use of any such persisting
	// topology instead of re-computing it. This is avoid unnecessary
	// expenditure of time in obtaining the topology here.
	devices := topology.Devices(sh.GetServoTopology(), "")
	err := components_topology.VerifyServoTopologyItems(ctx, devices)
	if err != nil {
		// This situation can arise if the servo topology has been
		// verified in an earlier action, but the topology was not
		// persisted because the updateServo parameter was not set in
		// that action, or for some reason the stored topology is
		// corrupted. In this case we do not have any choice but to
		// re-compute the topology.
		devices, err = topology.ListOfDevices(ctx, runner, sh.GetSerialNumber())
		if err != nil {
			errors.Annotate(err, "servo firmware needs update exec").Err()
		}
		log.Debugf(ctx, "Servo Firmware Needs Update Exec: topology re-computed because pre-existing servo topology not found, or had errors.")
	}
	for _, d := range devices {
		if components_topology.IsItemGood(ctx, d) {
			log.Debugf(ctx, "Servo Firmware Needs Update Exec: device type (d.Type) :%q.", d.Type)
			if needsUpdate(ctx, runner, d, sh.GetFirmwareChannel()) {
				log.Debugf(ctx, "Servo Firmware Needs Update Exec: needs update is true")
				return errors.Reason("servo firmware needs update exec: servo needs update").Err()
			}
		}
	}
	return nil
}

// servoSetExec sets the command of the servo a specific value using servod.
// It reads the command and its value from the actionArgs argument.
//
// the actionArgs should be in the format of ["command:....", "string_value:...."]
func servoSetExec(ctx context.Context, info *execs.ExecInfo) error {
	m := info.GetActionArgs(ctx)
	command := strings.TrimSpace(m.AsString(ctx, "command", ""))
	if command == "" {
		return errors.Reason("servo set: command not found or empty").Err()
	}
	if !m.Has("string_value") {
		return errors.Reason("servo set: string value not specified").Err()
	}
	stringValue := strings.TrimSpace(m.AsString(ctx, "string_value", ""))
	servod := info.NewServod()
	timeout := m.AsDuration(ctx, "timeout", components.ServodDefaultTimeoutSec, time.Second)
	if v, err := servod.Call(ctx, components.ServodSet, timeout, command, stringValue); err != nil {
		return errors.Annotate(err, "servo set").Err()
	} else {
		log.Infof(ctx, "Call %q:%q and received %v", command, stringValue, v)
	}
	return nil
}

const (
	// This is the servod control for obtaining ppdut5 bsus voltage in
	// millivolts.
	servodPPDut5Cmd = "ppdut5_mv"
)

// Verify that the DUT is connected to Servo using the 'ppdut5_mv'
// servod control.
func servoLowPPDut5Exec(ctx context.Context, info *execs.ExecInfo) error {
	s := info.NewServod()
	if err := s.Has(ctx, servodPPDut5Cmd); err != nil {
		return errors.Annotate(err, "servo low ppdut5 exec").Err()
	}
	voltageValue, err := servodGetDouble(ctx, s, servodPPDut5Cmd)
	if err != nil {
		return errors.Annotate(err, "servo low ppdut5 exec").Err()
	}
	if voltageValue < maxPPDut5MVWhenNotConnected {
		return errors.Reason("servo low ppdut5 exec: the ppdut5_mv value %v is lower than the threshold %d", voltageValue, maxPPDut5MVWhenNotConnected).Err()
	}
	// TODO(vkjoshi@): add metrics to collect the value of the
	// servod control ppdut5_mv when it is below a certain threshold.
	// (ref:http://cs/chromeos_public/src/third_party/labpack/files/server/hosts/servo_repair.py?l=640).
	return nil
}

// Verify that the control has min double value by servod control.
func servoControlMinDoubleValueExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	control := argsMap.AsString(ctx, "control", "")
	minValue := argsMap.AsFloat64(ctx, "min_value", 0)
	if control == "" {
		return errors.Reason("servo control low double value: control is not provided").Err()
	}
	s := info.NewServod()
	if err := s.Has(ctx, control); err != nil {
		return errors.Annotate(err, "servo control low double value").Err()
	}
	receivedValue, err := servodGetDouble(ctx, s, control)
	if err != nil {
		return errors.Annotate(err, "servo control low double value").Err()
	}
	info.AddObservation(metrics.NewStringObservation("control", control))
	info.AddObservation(metrics.NewFloat64Observation("receivedValue", receivedValue))
	if receivedValue < minValue {
		return errors.Reason("servo control low double value: the value %v is lower than the threshold %v", receivedValue, minValue).Err()
	}
	log.Debugf(ctx, "Servo %q: the value %v is >= than the threshold %d", control, receivedValue, minValue)
	return nil
}

const (
	// removeFileCmd is the linux file removal command that used to remove files in the filesToRemoveSlice.
	removeFileCmd = `rm %s`
)

var filesToRemoveSlice = []string{
	"/var/lib/metrics/uma-events",
	"/var/spool/crash/*",
	"/var/log/chrome/*",
	"/var/log/ui/*",
	"/home/chronos/BrowserMetrics/*",
}

// servoLabstationDiskCleanUpExec remove files that are in the filesToRemoveSlice.
func servoLabstationDiskCleanUpExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	// Remove all files in the filesToRemoveSlice during the labstation disk clean up process.
	for _, filePath := range filesToRemoveSlice {
		if _, err := r(ctx, time.Minute, fmt.Sprintf(removeFileCmd, filePath)); err != nil {
			log.Debugf(ctx, "servo labstation disk clean up: %s", err.Error())
		}
		log.Infof(ctx, "labstation file removed: %s", filePath)
	}
	return nil
}

const (
	// removeOldServodLogsCmd is the command to remove any servod files that is older than the maximum days specified by d.
	removeOldServodLogsCmd = `/usr/bin/find /var/log/servod_* -mtime +%d -print -delete`
)

// servoServodOldLogsCleanupExec removes the old servod log files that existed more than keepLogsMaxDays days.
//
// @params: actionArgs should be in the format of: ["max_days:5"]
func servoServodOldLogsCleanupExec(ctx context.Context, info *execs.ExecInfo) error {
	daysMap := info.GetActionArgs(ctx)
	keepLogsMaxDaysString, existed := daysMap["max_days"]
	if !existed {
		return errors.Reason("servod old logs: missing max days information in the argument").Err()
	}
	keepLogsMaxDaysString = strings.TrimSpace(keepLogsMaxDaysString)
	if keepLogsMaxDaysString == "" {
		return errors.Reason("servod old logs: max days information is empty").Err()
	}
	keepLogsMaxDays, err := strconv.ParseInt(keepLogsMaxDaysString, 10, 64)
	if err != nil {
		return errors.Annotate(err, "servod old logs").Err()
	}
	log.Infof(ctx, "The max number of days for keeping old servod logs is: %v", keepLogsMaxDays)
	r := info.DefaultRunner()
	// remove old servod logs.
	if _, err := r(ctx, time.Minute, fmt.Sprintf(removeOldServodLogsCmd, keepLogsMaxDays)); err != nil {
		log.Debugf(ctx, "servo servod old logs clean up: %s", err.Error())
	}
	return nil
}

// servoValidateBatteryChargingExec uses servod controls to check
// whether or not the battery on a DUT is capable of getting
// charged. It marks the DUT for replacement if its battery cannot be
// charged.
func servoValidateBatteryChargingExec(ctx context.Context, info *execs.ExecInfo) error {
	// This is the number of times we will try to read the value of battery controls from servod.
	const servodBatteryReadRetryLimit = 3
	// This is the servod control to determine battery's last full charge.
	const batteryFullChargeServodControl = "battery_full_charge_mah"
	// This is the servod control to determine the bettery's full capacity by design.
	const batteryDesignFullCapacityServodControl = "battery_full_design_mah"
	var lastFullCharge, batteryCapacity int32
	var getLastFullCharge = func() error {
		var err error
		lastFullCharge, err = servodGetInt(ctx, info.NewServod(), batteryFullChargeServodControl)
		return err
	}
	if info.GetChromeos().GetBattery() == nil {
		return errors.Reason("servo validate battery charging: data is not present in dut info").Err()
	}
	if err := retry.LimitCount(ctx, servodBatteryReadRetryLimit, -1, getLastFullCharge, "get last full charge"); err != nil {
		log.Debugf(ctx, "Servo Validate Battery Charging: could not read last full charge despite trying %d times", servodBatteryReadRetryLimit)
		return errors.Annotate(err, "servo validate battery charging").Err()
	}
	log.Debugf(ctx, "Servo Validate Battery Charging: last full charge is %d", lastFullCharge)
	var getBatteryCapacity = func() error {
		var err error
		batteryCapacity, err = servodGetInt(ctx, info.NewServod(), batteryDesignFullCapacityServodControl)
		return err
	}
	if err := retry.LimitCount(ctx, servodBatteryReadRetryLimit, -1, getBatteryCapacity, "get battery capacity"); err != nil {
		log.Debugf(ctx, "Servo Validate Battery Charging: could not read battery capacity despite trying %d times", servodBatteryReadRetryLimit)
		return errors.Annotate(err, "servo validate battery charging").Err()
	}
	log.Debugf(ctx, "Servo Validate Battery Charging: battery capacity is %d", batteryCapacity)
	hardwareState := battery.DetermineHardwareStatus(ctx, float64(lastFullCharge), float64(batteryCapacity))
	log.Infof(ctx, "Battery hardware state: %s", hardwareState)
	if hardwareState == tlw.HardwareState_HARDWARE_UNSPECIFIED {
		return errors.Reason("servo validate battery charging: dut battery did not detected or state cannot extracted").Err()
	}
	if hardwareState == tlw.HardwareState_HARDWARE_NEED_REPLACEMENT {
		log.Infof(ctx, "Detected issue with storage on the DUT.")
		info.GetChromeos().GetBattery().State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
	}
	return nil
}

// initDutForServoExec initializes the DUT and sets all servo signals
// to default values.
func initDutForServoExec(ctx context.Context, info *execs.ExecInfo) error {
	s := info.NewServod()
	args := info.GetActionArgs(ctx)
	hwinitVerbose := args.AsBool(ctx, "hwinit_verbose", true)
	hwinitTimeout := args.AsDuration(ctx, "hwinit_timeout", 60, time.Second)
	start := time.Now()
	if _, err := s.Call(ctx, "hwinit", hwinitTimeout, hwinitVerbose); err != nil {
		return errors.Annotate(err, "init dut for servo exec").Err()
	}
	info.AddObservation(metrics.NewFloat64Observation("hwinit_execution_time", time.Since(start).Seconds()))
	usbMuxControl := "usb_mux_oe1"
	if err := s.Has(ctx, usbMuxControl); err == nil {
		if err2 := s.Set(ctx, usbMuxControl, "on"); err2 != nil {
			return errors.Fmt("init dut for servo exec: %w", err2)
		}
		if err := s.Set(ctx, "image_usbkey_pwr", "off"); err != nil {
			log.Debugf(ctx, "Fail to set USB power off: %s", err)
		}
	} else {
		log.Debugf(ctx, "Init Dut For Servo Exec: servod control %q is not available.", usbMuxControl)
	}
	return nil
}

// servoUpdateServoFirmwareExec updates all the servo devices' firmware based on the condition specified by the actionArgs
//
// @params try_attempt_count:  Count of attempts to update servo. For force option the count attempts is always 1 (one).
// @params try_force_update_after_fail:   Try force force option if fail to update in normal mode.
// @params force_update:       Run updater with force option. Override try_force_update_after_fail option.
// @params ignore_version:     Skip check the version on the device.
//
// @params: actionArgs should be in the format of:
// Ex: ["try_attempt_count:x", "try_force_update_after_fail:true/false",
//
//	"force_update:true/false", "ignore_version:true/false",
//	"servo_board:servo_micro"]
func servoUpdateServoFirmwareExec(ctx context.Context, info *execs.ExecInfo) (err error) {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo update servo firmware: servo is not present as part of dut info").Err()
	}
	fwUpdateMap := info.GetActionArgs(ctx)
	// If the passed in "try_attempt_count" is either 0 or cannot be parsed successfully,
	// then, we default the count to be 1 to at least try to update it once.
	tryAttemptCount := fwUpdateMap.AsInt(ctx, "try_attempt_count", 1)
	tryForceUpdateAfterFail := fwUpdateMap.AsBool(ctx, "try_force_update_after_fail", false)
	forceUpdate := fwUpdateMap.AsBool(ctx, "force_update", false)
	ignoreVersion := fwUpdateMap.AsBool(ctx, "ignore_version", false)
	filteredServoBoard := fwUpdateMap.AsString(ctx, "servo_board", "")
	if filteredServoBoard != "" {
		log.Debugf(ctx, "Servo update servo firmware: Only updating board: %q's firmware", filteredServoBoard)
	}
	run := info.NewRunner(sh.GetName())
	if forceUpdate {
		// If requested to update with force then first attempt will be with force
		// and there no second attempt.
		tryAttemptCount = 1
		tryForceUpdateAfterFail = false
	}
	req := FwUpdaterRequest{
		UseContainer:            IsContainerizedServoHost(ctx, sh),
		FirmwareChannel:         sh.GetFirmwareChannel(),
		TryAttemptCount:         tryAttemptCount,
		TryForceUpdateAfterFail: tryForceUpdateAfterFail,
		ForceUpdate:             forceUpdate,
		IgnoreVersion:           ignoreVersion,
	}
	devicesToUpdate := topology.Devices(sh.GetServoTopology(), filteredServoBoard)
	if len(devicesToUpdate) == 0 {
		return errors.Reason("servo update servo firmware: the number of servo devices to update fw is 0").Err()
	}
	failDevices := UpdateDevicesServoFw(ctx, run, req, devicesToUpdate)
	// Record every single servo device fw flash time as well as status to karte.
	for _, device := range devicesToUpdate {
		status := metrics.ActionStatusSuccess
		for _, failDevice := range failDevices {
			if failDevice.GetType() == device.GetType() {
				status = metrics.ActionStatusFail
				break
			}
		}
		observationStatusKind := fmt.Sprintf(metrics.ServoEachDeviceFwUpdateKind, device.Type)
		info.AddObservation(metrics.NewStringObservation(observationStatusKind, string(status)))
	}
	if len(failDevices) != 0 {
		sh.State = tlw.ServoHost_NEED_REPLACEMENT
		return errors.Reason("servo update servo firmware: %d servo devices fails the update process", len(failDevices)).Err()
	}
	return nil
}

// servoFakeDisconnectDUTExec tries to unplug DUT from servo and restore the connection.
//
// @params: actionArgs should be in the format of:
// Ex: ["delay_in_ms:x", "timeout_in_ms:x"]
func servoFakeDisconnectDUTExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// Delay to disconnect in milliseconds. Default to be 100ms.
	delayMS := argsMap.AsInt(ctx, "delay_in_ms", 100)
	// Timeout to wait to restore the connection. Default to be 2000ms.
	timeoutMS := argsMap.AsInt(ctx, "timeout_in_ms", 2000)
	disconnectCmd := fmt.Sprintf(`fakedisconnect %d %d`, delayMS, timeoutMS)
	if err := info.NewServod().Set(ctx, "root.servo_uart_cmd", disconnectCmd); err != nil {
		return errors.Annotate(err, "servod fake disconnect servo").Err()
	}
	// Formula to cover how long we wait to see the effect
	// when we convert params to seconds and then +2 seconds to apply effect.
	waitFinishExecutionTimeout := time.Duration(delayMS+timeoutMS)*time.Millisecond + 2*time.Second
	time.Sleep(waitFinishExecutionTimeout)
	return nil
}

// servoServodCCToggleExec is the servo repair action that toggles cc line off and then on.
//
// @params: actionArgs should be in the format of:
// Ex: ["off_timeout:x", "on_timeout:x"]
func servoServodCCToggleExec(ctx context.Context, info *execs.ExecInfo) error {
	ccToggleMap := info.GetActionArgs(ctx)
	// Timeout for shut down configuration channel. Default to be 10s.
	ccOffTimeout := ccToggleMap.AsInt(ctx, "off_timeout", 10)
	// Timeout for initialize configuration channel. Default to be 30s.
	ccOnTimeout := ccToggleMap.AsInt(ctx, "on_timeout", 30)
	// Turning off configuration channel.
	log.Infof(ctx, "Turn off configuration channel and wait %d seconds.", ccOffTimeout)
	if err := info.NewServod().Set(ctx, "root.servo_uart_cmd", "cc off"); err != nil {
		return errors.Annotate(err, "servod cc toggle").Err()
	}
	time.Sleep(time.Duration(ccOffTimeout) * time.Second)
	// Turning on configuration channel.
	log.Infof(ctx, "Turn on configuration channel and wait %d seconds.", ccOnTimeout)
	if err := info.NewServod().Set(ctx, servodPdRoleCmd, servodPdRoleValueSrc); err != nil {
		return errors.Annotate(err, "servod cc toggle").Err()
	}
	// "servo_dts_mode" is the servod command to enable/disable DTS mode on servo.
	// It has two value for this cmd: on and off.
	if err := info.NewServod().Set(ctx, "servo_dts_mode", "on"); err != nil {
		return errors.Annotate(err, "servod cc toggle").Err()
	}
	time.Sleep(time.Duration(ccOnTimeout) * time.Second)
	return nil
}

// servoServodDTSAndServoRoleToggleExec is the servo repair action that toggles dts mode and servo role.
// Note: do not change the sequence of operations as the repair actions here are executed in a sequence like  -
// "dut-control -p $port servo_dts_mode:off sleep:1 servo_pd_role:snk sleep:1 servo_pd_role:src sleep:1 servo_dts_mode:on"
// @params: actionArgs should be in the format of:
// Ex: ["off_timeout:x", "on_timeout:x"]
func servoServodDTSAndServoRoleToggleExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// Timeout to turn off dts.
	dtsOffTimeout := argsMap.AsDuration(ctx, "dts_to_off_timeout", 1, time.Second)
	dtsOnTimeout := argsMap.AsDuration(ctx, "dts_to_on_timeout", 5, time.Second)
	pdSnkTimeout := argsMap.AsDuration(ctx, "pd_role_to_snk_timeout", 1, time.Second)
	pdSrcTimeout := argsMap.AsDuration(ctx, "pd_role_to_src_timeout", 1, time.Second)
	verifyGscPresent := argsMap.AsBool(ctx, "verify_gsc_present", true)
	verifyRetryCount := argsMap.AsInt(ctx, "verify_retry_count", 5)
	servod := info.NewServod()
	hostAccess := info.NewHostAccess(info.GetChromeos().GetServo().GetName())

	// "servo_dts_mode" is the servod command to enable/disable DTS mode on servo.
	// turn off dts mode
	log.Infof(ctx, "Turn off dts and wait %s.", dtsOffTimeout)
	if err := servod.Set(ctx, "servo_dts_mode", "off"); err != nil {
		return errors.Annotate(err, "servod dts and servo role toggle").Err()
	}
	time.Sleep(dtsOffTimeout)

	// "servo_pd_role" is the servod command to toggle servo role.
	log.Infof(ctx, "Set servo pd role to snk and wait %s.", pdSnkTimeout)
	if err := servod.Set(ctx, "servo_pd_role", "snk"); err != nil {
		return errors.Annotate(err, "servod dts and servo role toggle").Err()
	}
	time.Sleep(pdSnkTimeout)

	log.Infof(ctx, "Set servo pd role to src and wait %s.", pdSrcTimeout)
	if err := servod.Set(ctx, "servo_pd_role", "src"); err != nil {
		return errors.Annotate(err, "servod dts and servo role toggle").Err()
	}
	time.Sleep(pdSrcTimeout)

	// turn on dts mode.
	log.Infof(ctx, "Turn on dts and wait %s.", dtsOnTimeout)
	if err := servod.Set(ctx, "servo_dts_mode", "on"); err != nil {
		return errors.Annotate(err, "servod dts and servo role toggle").Err()
	}
	time.Sleep(dtsOnTimeout)
	log.Debugf(ctx, "Fix performed without error.")

	if verifyGscPresent {
		var gscSerial string
		for _, child := range info.GetChromeos().GetServo().GetServoTopology().GetChildren() {
			if servo.GSC_SERVOS[child.GetType()] {
				gscSerial = child.GetSerial()
				log.Debugf(ctx, "Found GSC serial: %q", gscSerial)
				break
			}
		}
		if gscSerial == "" {
			log.Debugf(ctx, "Skip verification due missed GSC serial.")
		} else {
			cmd := fmt.Sprintf("lsusb -v -d 18d1: | grep %q", gscSerial)
			var lastErr error
			for range verifyRetryCount {
				log.Debugf(ctx, "Attempt %d to find servo with serial: %q.", gscSerial)
				if _, err := hostAccess.Run(ctx, 30*time.Second, cmd); err != nil {
					lastErr = errors.Annotate(err, "verify recovery gsc").Err()
					log.Debugf(ctx, "Fail to find servo with serial: %q", gscSerial)
					time.Sleep(time.Second)
				} else {
					lastErr = nil
					log.Debugf(ctx, "Found servo with serial: %q", gscSerial)
					log.Debugf(ctx, "Verification passed!")
					break
				}
			}
			// If verification fail then fix didn't work.
			if lastErr != nil {
				return errors.Annotate(lastErr, "servod dts and servo role toggle").Err()
			}
		}
	}
	return nil
}

// servoSetEcUartCmdExec will set "ec_uart_cmd" to the specific value based on the passed in parameter.
// Before and after the set of the "ec_uart_cmd", it will toggle the value of "ec_uart_flush".
//
// @params: actionArgs should be in the format of:
// Ex: ["wait_timeout:x", "value:xxxx"]
func servoSetEcUartCmdExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// Timeout to wait for setting the ec_uart_cmd. Default to be 1s.
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 1, time.Second)
	// The value of 'pd dualrole' postfix for the servod command 'ec_uart_cmd'
	value := argsMap.AsString(ctx, "value", "")
	if value == "" {
		return errors.Reason("servo set ec uart cmd: the passed in value cannot be empty").Err()
	}
	servod := info.NewServod()
	if err := servo.SetEcUartCmd(ctx, servod, value, waitTimeout); err != nil {
		return errors.Annotate(err, "servo set ec uart cmd").Err()
	}
	return nil
}

// servoPowerStateResetExec using servod command to reset power state
// to achieve the behaviour of DUT reboot to recover some servo controls depending on EC console.
//
// Some servo controls, like lid_open, requires communicating with DUT through EC UART console.
// Failure of this kinds of controls can be recovered by rebooting the DUT.
//
// @params: actionArgs should be in the format of:
// Ex: ["wait_timeout:x", "timeout:x", "more_logs:true"]
func servoPowerStateResetExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	waitTimeoutAfterCall := argsMap.AsDuration(ctx, "wait_timeout", 1, time.Second)
	timeout := argsMap.AsDuration(ctx, "timeout", components.ServodDefaultTimeoutSec, time.Second)
	enableMoreLogs := argsMap.AsBool(ctx, "more_logs", false)

	servod := info.NewServod()
	// Set hcdebug every to be able to collect inforof cold-reset.
	// We run it always as some board
	if _, err := servod.Call(ctx, components.ServodSet, timeout, "power_state", "reset"); err != nil {
		return errors.Annotate(err, "servo power state reset").Err()
	}
	if enableMoreLogs {
		// We enable more logs as some board can disable them by default.
		// More detail b/325124302#14.
		ecCommands := []string{"hcdebug every", "chan 0xffffffff"}
		for _, ecCommand := range ecCommands {
			if err := servod.Set(ctx, "ec_uart_cmd", ecCommand); err != nil {
				log.Debugf(ctx, "Fail to set %q: %s", ecCommand, err)
			}
		}
	}

	time.Sleep(waitTimeoutAfterCall)
	return nil
}

// servoPowerStateMatchExec matches powerstate received by servod to expected values.
//
// power_states can be S0, S3, S4, S5, G3.
func servoECPowerStateMatchExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	expectedStates := argsMap.AsStringSlice(ctx, "power_states", nil)
	if len(expectedStates) == 0 {
		return errors.Reason("servo EC power state match: power states not specified").Err()
	}
	currentState, err := servodGetString(ctx, info.NewServod(), "ec_system_powerstate")
	if err != nil {
		return errors.Annotate(err, "servo power state reset").Err()
	}
	log.Debugf(ctx, "Current EC powerstate is %q.", currentState)
	for _, expectedState := range expectedStates {
		if expectedState == currentState {
			log.Debugf(ctx, "EC powerstate match found!")
			return nil
		}
	}
	return errors.Reason("servo EC power state match: no match found").Err()
}

func init() {
	execs.Register("servo_host_servod_init", servodInitActionExec)
	execs.Register("servo_host_servod_stop", servodStopActionExec)
	execs.Register("servo_create_flag_to_use_recovery_mode", servodCreateFlagToUseRecoveryModeExec)
	execs.Register("servo_v4_root_present", isRootServoPresentExec)
	execs.Register("servo_topology_update", servoTopologyUpdateExec)
	execs.Register("servo_servod_echo_host", servoServodEchoHostExec)
	execs.Register("servo_fw_need_update", servoFirmwareNeedsUpdateExec)
	execs.Register("servo_set", servoSetExec)
	execs.Register("servo_low_ppdut5", servoLowPPDut5Exec)
	execs.Register("servo_control_min_double_value", servoControlMinDoubleValueExec)
	execs.Register("servo_labstation_disk_cleanup", servoLabstationDiskCleanUpExec)
	execs.Register("servo_servod_old_logs_cleanup", servoServodOldLogsCleanupExec)
	execs.Register("servo_battery_charging", servoValidateBatteryChargingExec)
	execs.Register("init_dut_for_servo", initDutForServoExec)
	execs.Register("servo_update_servo_firmware", servoUpdateServoFirmwareExec)
	execs.Register("servo_fake_disconnect_dut", servoFakeDisconnectDUTExec)
	execs.Register("servo_servod_cc_toggle", servoServodCCToggleExec)
	execs.Register("servo_set_ec_uart_cmd", servoSetEcUartCmdExec)
	execs.Register("servo_power_state_reset", servoPowerStateResetExec)
	execs.Register("servo_power_state_match", servoECPowerStateMatchExec)
	execs.Register("servo_servod_dts_and_servo_role_toggle_exec", servoServodDTSAndServoRoleToggleExec)
}
