// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros/cellular"
	"infra/cros/recovery/internal/components/cros/cellular/starfish"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
	"infra/cros/recovery/logger/metrics"
	"infra/cros/recovery/tlw"
)

func init() {
	execs.Register("cros_audit_cellular_modem", auditCellularModemExec)
	execs.Register("cros_audit_cellular_configuration", auditCellularConfigExec)
	execs.Register("cros_collect_supported_carriers", collectSupportedCarriersExec)
	execs.Register("cros_update_cellular_modem_labels", updateCellularModemLabelsExec)
	execs.Register("cros_update_cellular_sim_labels", updateCellularSIMLabelsExec)
	execs.Register("cros_has_only_one_sim_profile", hasOnlyOneSIMProfileExec)
	execs.Register("cros_audit_cellular_connection", auditCellularConnectionExec)
	execs.Register("cros_has_mmcli", hasModemManagerCLIExec)
	execs.Register("cros_has_modemmanager_job", hasModemManagerJobExec)
	execs.Register("cros_modemmanager_running", modemManagerRunningExec)
	execs.Register("cros_modem_state_not_in", modemStateNotInExec)
	execs.Register("cros_sim_info_empty", simInfoEmptyExec)
	execs.Register("cros_imei_empty", modemIMEImptyExec)
	execs.Register("cros_restart_modemmanager", restartModemManagerExec)
	execs.Register("set_cellular_modem_state", setCellularModemStateExec)
	execs.Register("has_cellular_info", hasCellularInfoExec)
}

// hasModemManagerCLIExec validates that mmcli is present on the DUT
func hasModemManagerCLIExec(ctx context.Context, info *execs.ExecInfo) error {
	if !cellular.HasModemManagerCLI(ctx, info.DefaultRunner(), info.GetExecTimeout()) {
		return errors.Reason("has modem manager cli: mmcli is not found on device").Err()
	}
	return nil
}

// hasModemManagerJobExec validates that modemmanager job is known by upstart and present on the DUT.
func hasModemManagerJobExec(ctx context.Context, info *execs.ExecInfo) error {
	if !cellular.HasModemManagerJob(ctx, info.DefaultRunner(), info.GetExecTimeout()) {
		return errors.Reason("has modem manager job: modemmanager is not found on device").Err()
	}
	return nil
}

// modemManagerRunningExec ensures modemmanager is running on the DUT and starts it if it's not already.
func modemManagerRunningExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.DefaultRunner()
	argsMap := info.GetActionArgs(ctx)
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 10, time.Second)
	startTimeout := argsMap.AsDuration(ctx, "start_timeout", 30, time.Second)
	if cellular.WaitForModemManager(ctx, runner, waitTimeout) == nil {
		return nil
	}

	if err := cellular.StartModemManager(ctx, runner, startTimeout); err != nil {
		return errors.Annotate(err, "start modemmanager").Err()
	}

	if err := cellular.WaitForModemManager(ctx, runner, waitTimeout); err != nil {
		return errors.Annotate(err, "wait for modemmanager to start").Err()
	}
	return nil
}

// restartModemManagerExec restarts modemmanagr on the DUT.
func restartModemManagerExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.DefaultRunner()
	argsMap := info.GetActionArgs(ctx)
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 10, time.Second)
	restartTimeout := argsMap.AsDuration(ctx, "restart_timeout", 30, time.Second)
	if err := cellular.RestartModemManager(ctx, runner, restartTimeout); err != nil {
		return errors.Annotate(err, "restart modemmanager").Err()
	}

	if err := cellular.WaitForModemManager(ctx, runner, waitTimeout); err != nil {
		return errors.Annotate(err, "wait for modemmanager to start").Err()
	}
	return nil
}

// hasCellularInfoExec validates that cellular data is populated in the dut info.
func hasCellularInfoExec(ctx context.Context, info *execs.ExecInfo) error {
	if c := info.GetChromeos().GetCellular(); c == nil {
		return errors.Reason("has cellular info: cellular data is not present in dut info").Err()
	}
	return nil
}

// setCellularModemStateExec sets the DUT's modem state to the requested value.
func setCellularModemStateExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("set cellular modem state: cellular data is not present in dut info").Err()
	}

	actionMap := info.GetActionArgs(ctx)
	state := strings.ToUpper(actionMap.AsString(ctx, "state", ""))
	if state == "" {
		return errors.Reason("set cellular modem state: state is not provided").Err()
	}
	s, ok := tlw.HardwareState_value[state]
	if !ok {
		return errors.Reason("set cellular modem state state: state %q is invalid", state).Err()
	}

	c.ModemState = tlw.HardwareState(s)
	return nil
}

// modemStateNotInExec verifies that the modem state exported by ModemManager is not in the provided list.
func modemStateNotInExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("modem state not in: cellular data is not present in dut info").Err()
	}

	argsMap := info.GetActionArgs(ctx)
	timeout := argsMap.AsDuration(ctx, "wait_modem_timeout", 15, time.Second)
	modemInfo, err := cellular.WaitForModemInfo(ctx, info.DefaultRunner(), timeout)
	if err != nil {
		return errors.Reason("modem state not in: no modem exported by ModemManager").Err()
	}

	if modemInfo.GetState() == "" {
		return errors.Reason("modem state not in: modem state is empty").Err()
	}

	modemState := modemInfo.GetState()
	states := argsMap.AsStringSlice(ctx, "states", []string{})
	for _, state := range states {
		if strings.EqualFold(state, modemState) {
			return errors.Reason("modem state not in: modem state %q is in the provided list", modemState).Err()
		}
	}

	return nil
}

// reportCellularConnectionInfo reports relevant connection info after an action.
func reportCellularConnectionInfo(ctx context.Context, info *execs.ExecInfo, timeout time.Duration, pi *tlw.Cellular_SIMProfileInfo) error {
	runner := info.DefaultRunner()
	modemInfo, err := cellular.WaitForModemInfo(ctx, runner, timeout)
	if err != nil {
		info.AddObservation(metrics.NewStringObservation("cellularModemHWState_"+pi.GetIccid(), "MISSING"))
		return err
	}

	pi.State = modemInfo.GetSIMState()
	connectionState := "UNKNOWN"
	if modemInfo.GetState() != "" {
		connectionState = strings.ToUpper(modemInfo.GetState())
	}
	info.AddObservation(metrics.NewStringObservation("cellularModemHWState_"+pi.GetIccid(), "AVAILABLE"))
	info.AddObservation(metrics.NewStringObservation("cellularConnectionState_"+pi.GetIccid(), connectionState))

	// Signal strength may not always be available by the modem so only report if there's no error.
	if signalStrength, err := cellular.GetSignalStrength(ctx, runner, timeout); err == nil {
		for _, strength := range signalStrength {
			prefix := fmt.Sprintf("cellular%vSignal", strength.Technology)
			if strength.RSRP != nil {
				info.AddObservation(metrics.NewFloat64Observation(prefix+"RSRP+_"+pi.GetIccid(), *strength.RSRP))
			}
			if strength.RSSI != nil {
				info.AddObservation(metrics.NewFloat64Observation(prefix+"RSSI_"+pi.GetIccid(), *strength.RSSI))
			}
			if strength.SNR != nil {
				info.AddObservation(metrics.NewFloat64Observation(prefix+"SNR_"+pi.GetIccid(), *strength.SNR))
			}
		}
	}
	return nil
}

// auditCellularConfigExec verifies that the cellular SIM information in UFS is complete and accurate.
func auditCellularConfigExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("imei empty: cellular data is not present in dut info").Err()
	}

	for _, si := range c.GetSimInfos() {
		for j, pi := range si.GetProfileInfos() {
			// All SIMs should have ICCID populated.
			if pi.GetIccid() == "" {
				pi.State = tlw.Cellular_SIMProfileInfo_WRONG_CONFIG
				return errors.Reason("audit cellular configs: missing ICCCID for sim: %q, profile: %d", si.GetSlotId(), j).Err()
			}

			// Check if the ICCID for this SIM slot matches, if it doesn't then the SIM was likely replaced
			// at some time and the labels weren't updated.
			if pi.GetDetectedIccid() != "" && !strings.EqualFold(pi.GetDetectedIccid(), pi.GetIccid()) {
				pi.State = tlw.Cellular_SIMProfileInfo_WRONG_CONFIG
				return errors.Reason("audit cellular configs: Mismatched OwnNumber for ICCID %q, got %q, expected: %q",
					pi.GetIccid(), pi.GetDetectedIccid(), pi.GetIccid()).Err()
			}

			switch c.GetCarrier() {
			// If the DUT is a ATT/TMOBILE/VERIZON device, then it will be used for SMS tests so we need
			// to verify that the OwnNumber is accurate
			case "TMOBILE", "ATT", "VERIZON":
				if pi.GetOwnNumber() == "" {
					pi.State = tlw.Cellular_SIMProfileInfo_WRONG_CONFIG
					return errors.Reason("audit cellular configs: missing OwnNumber for ICCID: %q", pi.GetIccid()).Err()
				}
				if pi.GetDetectedIccid() != "" && pi.GetDetectedOwnNumber() != "" && pi.GetOwnNumber() != pi.GetDetectedOwnNumber() {
					pi.State = tlw.Cellular_SIMProfileInfo_WRONG_CONFIG
					return errors.Reason("audit cellular configs: Mismatched OwnNumber for ICCID %q, got %q, expected: %q",
						pi.GetIccid(), pi.GetDetectedOwnNumber(), pi.GetOwnNumber()).Err()
				}
			// If the DUT is a PINLOCK device, then we need the PIN/PUK locks to be populated.
			case "PINLOCK":
				if pi.GetSimPin() == "" {
					pi.State = tlw.Cellular_SIMProfileInfo_WRONG_CONFIG
					return errors.Reason("audit cellular configs: missing PIN for ICCID: %q", pi.GetIccid()).Err()
				}
				if pi.GetSimPuk() == "" {
					pi.State = tlw.Cellular_SIMProfileInfo_WRONG_CONFIG
					return errors.Reason("audit cellular configs: missing PUK for ICCID: %q", pi.GetIccid()).Err()
				}
				// We don't manage PINLOCK DUTs connection states in auditCellularConnectionExec like we do for other devices
				// so we should just set it to WORKING
				pi.State = tlw.Cellular_SIMProfileInfo_WORKING
			default:
				// If state is still default value then just convert to working. This means
				// that we don't have a specific audit available for this carrier but we
				// would still like to schedule against it so set it to WORKING.
				if pi.State == tlw.Cellular_SIMProfileInfo_UNSPECIFIED {
					pi.State = tlw.Cellular_SIMProfileInfo_WORKING
				}
			}
		}
	}
	return nil
}

// auditCellularConnectionExec verifies that the device is able to connect to the provided cellular network.
func auditCellularConnectionExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("imei empty: cellular data is not present in dut info").Err()
	}

	runner := info.DefaultRunner()
	argsMap := info.GetActionArgs(ctx)

	// When we connect successfully, it doesn't tend to take very long,
	// if it takes longer than 30 seconds we will probably never connect.
	waitTimeout := argsMap.AsDuration(ctx, "wait_connected_timeout", 30, time.Second)

	// Action requires at least 1 minute more than wait_connected_timeout to successfully complete the action.
	if waitTimeout+time.Minute > info.GetExecTimeout() {
		return errors.Reason("audit cellular connection: exec timeout must be >= wait_connected_timeout + 60s").Err()
	}

	connectAttempts := argsMap.AsInt(ctx, "connect_attempts", 1)
	attemptDelay := argsMap.AsDuration(ctx, "attempt_delay", 5, time.Second)
	resetCmdTimeout := argsMap.AsDuration(ctx, "reset_command_timeout", 5, time.Second)
	resetDelay := argsMap.AsDuration(ctx, "reset_delay", 10, time.Second)
	resetWaitTimeout := argsMap.AsDuration(ctx, "reset_wait_timeout", 30, time.Second)
	for _, si := range c.GetSimInfos() {
		if len(si.GetProfileInfos()) != 1 {
			// Only support SIMs with 1 profile for now since we don't support profile activation.
			log.Debugf(ctx, "Skipping slot: %d, more than 1 profile detected, only 1 supported", si.GetSlotId())
			continue
		}

		pi := si.GetProfileInfos()[0]
		if pi.GetIccid() == "" {
			// Don't try to connect to any profiles with empty ICCIDs.
			log.Debugf(ctx, "Skipping profile: empty ICCID in slot: %q", si.GetSlotId())
			continue
		}

		if err := cellular.SwitchSIMSlot(ctx, runner, si.GetSlotId()); err != nil {
			return errors.Annotate(err, "audit cellular connection").Err()
		}
		time.Sleep(5 * time.Second)

		// Optional, cache some of the SIM info that we found.
		if detectedSi, err := cellular.GetSIMInfo(ctx, runner); err != nil {
			log.Errorf(ctx, "Failed to detect SIM info after switching slogs, skipping ICCID verification: %w", err)
		} else if detectedSi != nil && len(detectedSi.GetProfileInfos()) == 1 {
			pi.DetectedIccid = detectedSi.GetProfileInfos()[0].GetIccid()
			pi.DetectedOwnNumber = detectedSi.GetProfileInfos()[0].GetOwnNumber()
		}

		if err := retry.LimitCount(ctx, connectAttempts, attemptDelay, func() error {
			if err := cellular.ConnectToDefaultService(ctx, runner, waitTimeout); err == nil {
				return nil
			} else {
				log.Errorf(ctx, "Failed connection attempt, resetting modem: %w", err)
			}

			if err := cellular.ResetModem(ctx, runner, resetCmdTimeout, resetDelay, resetWaitTimeout); err != nil {
				return errors.Annotate(err, "failed to restart modem after connection failure").Err()
			}
			return errors.Reason("connect to default service").Err()
		}, "wait for modemmanager"); err != nil {
			log.Debugf(ctx, "Failed to connect to SIM profile: %q", pi.GetIccid())
		}

		if err := reportCellularConnectionInfo(ctx, info, 15*time.Second, pi); err != nil {
			return errors.Annotate(err, "audit cellular connection").Err()
		}
	}

	return nil
}

// modemIMEImptyExec verifies that the modem IMEI has not been populated yet.
func modemIMEImptyExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("imei empty: cellular data is not present in dut info").Err()
	}

	if c.GetModemInfo().GetImei() != "" {
		return errors.Reason("imei empty: cellular modem info already populated").Err()
	}
	return nil
}

// updateCellularModemLabelsExec sets the cellular modem labels in swarming to match those available on the DUT.
func updateCellularModemLabelsExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("audit cellular modem labels: cellular data is not present in dut info").Err()
	}

	// Get labels directly from modem hardware.
	argsMap := info.GetActionArgs(ctx)
	timeout := argsMap.AsDuration(ctx, "wait_modem_timeout", 15, time.Second)
	modemInfo, err := cellular.WaitForModemInfo(ctx, info.DefaultRunner(), timeout)
	if err != nil {
		return errors.Reason("audit cellular modem labels: no modem exported by ModemManager").Err()
	}

	if modemInfo.GetImei() == "" {
		return errors.Reason("audit cellular modem labels: failed to get modem imei").Err()
	}

	// Get labels from cros_config.
	variant := cellular.GetModelVariant(ctx, info.DefaultRunner())
	if variant == "" {
		return errors.Reason("audit cellular modem labels: cellular variant not present on device").Err()
	}

	// First try to get modem type from config otherwise fall-back to inferring from variant.
	modemType := cellular.GetModemTypeFromConfig(ctx, info.DefaultRunner())
	if modemType == tlw.Cellular_MODEM_TYPE_UNSPECIFIED || modemType == tlw.Cellular_MODEM_TYPE_UNSUPPORTED {
		log.Infof(ctx, "audit cellular modem labels: failed to get modem type from config, falling back to variant")
		modemType = cellular.GetModemTypeFromVariant(variant)
	}

	if modemType == tlw.Cellular_MODEM_TYPE_UNSUPPORTED && (c.ModemInfo.Type == tlw.Cellular_MODEM_TYPE_UNSPECIFIED || c.ModemInfo.Type == tlw.Cellular_MODEM_TYPE_UNSUPPORTED) {
		// If unknown modem type and no modem was previously specified then just log as its a new device.
		log.Errorf(ctx, "audit cellular modem labels: unknown modem type for variant: %q", variant)
	} else if modemType == tlw.Cellular_MODEM_TYPE_UNSUPPORTED {
		// If unknown modem type and modem was previously specified, then we should error out
		// without updating anything as something has gone wrong and the device can't be trusted.
		return errors.Reason("audit cellular modem labels: unknown modem type for variant: %q", variant).Err()
	}

	// Update properties at end once everything has been verified.
	c.ModemInfo.Type = modemType
	c.ModelVariant = variant
	c.ModemInfo.Imei = modemInfo.GetImei()
	return nil
}

// Ensures that the SIM labels only contain references to at most one profile.
func hasOnlyOneSIMProfileExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("audit cellular sim labels: cellular data is not present in dut info").Err()
	}

	// It is possible that some SIMs have more than one profile installed on them.
	// If this happens, then we should fail and require the SIM labels to be manually
	// added or wiped first. As updating these would require activating/deactivating
	// SIM profiles, which could have unintended consequences if there is an inactive
	// profile installed on the DUT.
	for _, s := range c.GetSimInfos() {
		if len(s.GetProfileInfos()) > 1 {
			return errors.Reason("audit cellular sim labels: expected <= 1 profiles for each SIM, got %d", len(s.ProfileInfos)).Err()
		}
	}
	return nil
}

// simInfoEmptyExec verifies that the device SIM info has not been populated.
func simInfoEmptyExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("sim info empty: cellular data is not present in dut info").Err()
	}

	if len(c.GetSimInfos()) != 0 {
		return errors.Reason("sim info empty: SIM info already populated, found %d SIMs", len(c.GetSimInfos())).Err()
	}
	return nil
}

// updateCellularSIMLabelsExec queries the available SIM cards on the DUT and updates their information.
func updateCellularSIMLabelsExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("audit cellular sim labels: cellular data is not present in dut info").Err()
	}

	// Fetch available SIMs on device, if we fail to fetch any required information for a SIM then
	// we should fail without updating as we would still want to know which SIM are not being properly
	// populated.
	var simInfos []*tlw.Cellular_SIMInfo
	var err error
	if strings.Contains(c.GetCarrier(), "STARFISH") {
		simInfos, err = starfish.GetAllSIMInfo(ctx, info.DefaultRunner())
		if err != nil {
			return errors.Annotate(err, "audit cellular sim labels: failed to query sim info").Err()
		}
	} else {
		simInfos, err = cellular.GetAllSIMInfo(ctx, info.DefaultRunner())
		if err != nil {
			return errors.Annotate(err, "audit cellular sim labels: failed to query sim info").Err()
		}
	}

	simInfosBySlot := make(map[int32]*tlw.Cellular_SIMInfo)
	for _, si := range c.SimInfos {
		if _, ok := simInfosBySlot[si.SlotId]; ok {
			return errors.Reason("audit cellular sim labels: duplicate SIM slot ID found: %d", si.SlotId).Err()
		}
		simInfosBySlot[si.SlotId] = si
	}

	for _, newSI := range simInfos {
		if oldSI, ok := simInfosBySlot[newSI.SlotId]; ok {
			oldSI.Type = newSI.Type
			oldSI.Eid = newSI.Eid

			// Technically it's possible that old SimInfo has no profiles.
			var oldProfile *tlw.Cellular_SIMProfileInfo
			if len(oldSI.ProfileInfos) > 0 {
				oldProfile = oldSI.ProfileInfos[0]
			} else {
				oldProfile = &tlw.Cellular_SIMProfileInfo{}
				oldSI.ProfileInfos = append(oldSI.ProfileInfos, oldProfile)
			}

			// newSI always has exactly 1 profile since we just created it.
			newProfile := newSI.ProfileInfos[0]
			oldProfile.Iccid = newProfile.Iccid
			oldProfile.CarrierName = newProfile.CarrierName

			// OwnNumber may not be available from the DUT directly, and may instead
			// have been added directly to swarming via shivas, so don't overwrite it if it's missing.
			if newProfile.OwnNumber != "" {
				oldProfile.OwnNumber = newProfile.OwnNumber
			}
			delete(simInfosBySlot, newSI.SlotId)
		} else {
			c.SimInfos = append(c.SimInfos, newSI)
		}
	}

	// Check that we are not failing to detect any previously detected SIMs, if
	// we are, then fail the action after updating the SIMs that we did manage to detect.
	// We don't want to clear the missing SIMs since it may be helpful to know which SIMs
	// we are failing to find.
	if len(simInfosBySlot) != 0 {
		return errors.Reason("audit cellular sim labels: failed to find SIM info for %d slots", len(simInfosBySlot)).Err()
	}

	return nil
}

// auditCellularModem will validate cellular modem hardware state.
func auditCellularModemExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("audit cellular modem: cellular data is not present in dut info").Err()
	}
	expected := cellular.HasCellularVariant(ctx, info.DefaultRunner())

	// if no cellular is expected then set total timeout to be much lower otherwise we will add
	// ~2 minutes to every repair even ones that don't require a modem.
	argsMap := info.GetActionArgs(ctx)
	timeout := argsMap.AsDuration(ctx, "wait_manager_when_not_expected", 120, time.Second)
	if !expected {
		timeout = argsMap.AsDuration(ctx, "wait_manager_when_expected", 15, time.Second)
	}

	modemInfo, err := cellular.WaitForModemInfo(ctx, info.DefaultRunner(), timeout)
	if err == nil {
		// found modem, try to get connection status.
		connectionState := "UNKNOWN"
		if modemInfo.Modem.Generic != nil && modemInfo.Modem.Generic.State != "" {
			connectionState = strings.ToUpper(modemInfo.Modem.Generic.State)
		}

		// only report connection state for devices where modem was found.
		info.AddObservation(metrics.NewStringObservation("cellularConnectionState", connectionState))
		c.ModemState = tlw.HardwareState_HARDWARE_NORMAL
		return nil
	}

	err = errors.Annotate(err, "audit cellular modem").Err()
	if execs.SSHErrorInternal.In(err) {
		c.ModemState = tlw.HardwareState_HARDWARE_UNSPECIFIED
		return err
	}

	if expected {
		// no modem detected but was expected by setup info
		// then we set needs_replacement as it is probably a hardware issue.
		c.ModemState = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
		return err
	}

	// not found and not expected, don't report an error, instead just log it
	log.Errorf(ctx, err.Error())
	c.ModemState = tlw.HardwareState_HARDWARE_NOT_DETECTED
	return nil
}

// collectSupportedCarriersExec populates schedulable labels that are deduced based on other device labels.
func collectSupportedCarriersExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("collect supported carriers: cellular data is not present in dut info").Err()
	}

	// If the DUT is a "starfish" device then this DUT supports multiple carriers and its
	// supported_carriers just a list of all the SIM network operators on the device.
	// Otherwise, we only support 1 carrier and it is the default one on the device.
	var carriers []string
	if strings.EqualFold(c.GetCarrier(), "STARFISH") || strings.EqualFold(c.GetCarrier(), "STARFISHPLUS") {
		seen := make(map[string]bool)
		for _, s := range c.GetSimInfos() {
			for _, p := range s.GetProfileInfos() {
				name := strings.TrimPrefix(p.GetCarrierName().String(), "NETWORK_")
				if seen[name] {
					log.Infof(ctx, "collect supported carriers: skipping duplicate carrier: %q", name)
					continue
				}
				seen[name] = true
				carriers = append(carriers, name)
			}
		}
	} else if c.GetCarrier() != "" {
		carriers = []string{c.GetCarrier()}
	} else {
		log.Infof(ctx, "collect supported carriers: carrier is empty")
	}
	c.SupportedCarriers = carriers

	// Check if the device's "carrier" label is in a special group that means that it's not readily
	// connectable. If this is the case, we should ignore what the actual carrier reported by the SIM is.
	argsMap := info.GetActionArgs(ctx)
	isLiveCarrier := true
	for _, carrier := range argsMap.AsStringSlice(ctx, "offline_carriers", []string{}) {
		if strings.EqualFold(carrier, c.GetCarrier()) {
			isLiveCarrier = false
			break
		}
	}

	// Get the features for the individual carriers.
	for _, s := range c.GetSimInfos() {
		for _, p := range s.GetProfileInfos() {
			if isLiveCarrier {
				p.Features = cellular.SupportedFeaturesForCarrier(p.GetCarrierName())
			} else {
				p.Features = []tlw.Cellular_SIMProfileInfo_Feature{}
			}
		}
	}

	return nil
}
