// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/vpd"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// moSysSkuCmd will retrieve the SKU label of the DUT.
	moSysSkuCmd                    = "mosys platform sku"
	crosIDSkuCmd                   = "crosid -f SKU"
	cmdAudioLatencyToolkitCheck    = "lsusb -vv -d 16c0: | grep \"Teensyduino\""
	cmdAudioBeamformingCheckLegacy = "cros_config /audio/main cras-config-dir"
	cmdAudioBeamformingCheck       = "cras_server_tool label-audio_beamforming"
)

// updateDlmSkuIdExec updates device's SKU label if not present in inventory
// or keep it the same if the info.GetDut() already has the value for
// dlm_sku_id label.
func updateDlmSkuIDInvExec(ctx context.Context, info *execs.ExecInfo) error {
	// If sku is present, skip
	if info.GetChromeos().GetDlmSkuId() != "" {
		log.Debugf(ctx, "DlmSkuId already present. Skipping update.")
		return nil
	}

	const dlmSkuVpdKey = "dlm_sku_id"
	dlmSkuIDLabelOutput, err := vpd.Read(ctx, info.DefaultHostAccess(), info.GetExecTimeout(), dlmSkuVpdKey)
	if err != nil {
		return errors.Annotate(err, "update dlm_sku_id label").Err()
	}
	if dlmSkuIDLabelOutput == "" {
		return errors.Reason("vpd key %q: has empty value", dlmSkuVpdKey).Err()
	}
	log.Debugf(ctx, "DlmSkuId found: %q.", dlmSkuIDLabelOutput)
	info.GetChromeos().DlmSkuId = dlmSkuIDLabelOutput
	return nil
}

// updateDeviceSKUExec updates device's SKU label if not present in inventory
// or keep it the same if the info.GetDut() already has the value for SKU label.
func updateDeviceSKUExec(ctx context.Context, info *execs.ExecInfo) error {
	// If sku is present, skip
	if info.GetChromeos().DeviceSku != "" {
		log.Debugf(ctx, "Device sku already present. Skipping update.")
		return nil
	}

	r := info.DefaultRunner()
	// Try crosid
	skuLabelOutput, err := r(ctx, time.Minute, crosIDSkuCmd)
	if err == nil {
		log.Debugf(ctx, "Device sku found with crosid.")
		info.GetChromeos().DeviceSku = skuLabelOutput
		return nil
	}
	log.Debugf(ctx, "Error when reading device sku with crosid: %s", err)

	// Else, try mosys
	skuLabelOutput, err = r(ctx, time.Minute, moSysSkuCmd)
	if err == nil {
		log.Debugf(ctx, "Device sku found with mosys.")
		info.GetChromeos().DeviceSku = skuLabelOutput
		return nil
	}
	log.Debugf(ctx, "Error when reading device sku with mosys: %s", err)

	log.Debugf(ctx, "Device sku label not found in the DUT.")
	return errors.Annotate(err, "update device sku label").Err()
}

// isAudioLoopBackStateWorkingExec checks if the DUT's audio loop back state has already been in the working state.
func isAudioLoopBackStateWorkingExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetAudio().GetLoopbackState() != tlw.DUTAudio_LOOPBACK_WORKING {
		return errors.Reason("audio loop back state working: not working").Err()
	}
	return nil
}

// updateAudioLoopbackLabelExec updates the DUT's audio loop back state to the correct state
// based on the condition as follows:
// if both the Headphone and Mic exists on the DUT, then the state is working.
// For all other cases, state is unspecified.
func updateAudioLoopbackLabelExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetChromeos().GetAudio().LoopbackState = tlw.DUTAudio_LOOPBACK_UNSPECIFIED
	defer log.Infof(ctx, "Setting DUT's Audio Loopback State to be %s", info.GetChromeos().GetAudio().GetLoopbackState())
	r := info.DefaultRunner()
	// check if the Headphone cras audio type exists on the DUT.
	isAudioHeadPhoneExist, err := CrasAudioNodeTypeIsPlugged(ctx, r, CrasAudioHeadphone)
	if err != nil {
		return errors.Annotate(err, "update audio loop back label").Err()
	}
	// check if the Mic cras audio type exists on the DUT.
	isAudioMicExist, err := CrasAudioNodeTypeIsPlugged(ctx, r, CrasAudioMic)
	if err != nil {
		return errors.Annotate(err, "update audio loop back label").Err()
	}
	if isAudioHeadPhoneExist && isAudioMicExist {
		info.GetChromeos().GetAudio().LoopbackState = tlw.DUTAudio_LOOPBACK_WORKING
	}
	return nil
}

// updateAudioLatencyToolkitStateExec updates the DUT's audio latency toolkit state to the correct state
// based on the condition as follows:
// if audio latency toolkit not exists set as not applicable.
// if lsusb doesn't has audio latency toolkit set as broken.
// else set as working
func updateAudioLatencyToolkitStateExec(ctx context.Context, info *execs.ExecInfo) error {
	if version := info.GetChromeos().GetAudioLatencyToolkit().GetVersion(); version == "" {
		info.GetChromeos().GetAudioLatencyToolkit().State = tlw.AudioLatencyToolkit_NOT_APPLICABLE
	} else {
		log.Debugf(ctx, "Audio Latency Toolkit exists, version: %s", version)
		cmd := cmdAudioLatencyToolkitCheck
		res, err := info.DefaultRunner()(ctx, info.GetExecTimeout(), cmd)
		log.Debugf(ctx, "command \"%s\" shows: %s", cmd, res)
		if err != nil {
			log.Debugf(ctx, "command \"%s\" got error: %s", cmd, err)
			info.GetChromeos().GetAudioLatencyToolkit().State = tlw.AudioLatencyToolkit_BROKEN
			return errors.Annotate(err, "unable to find audio latency toolkit").Err()
		} else {
			info.GetChromeos().GetAudioLatencyToolkit().State = tlw.AudioLatencyToolkit_WORKING
		}
	}

	return nil
}

// updateAudioBeamformingTypeLegacy updates the DUT's Audio Beamforming type
// based on the legacy condition as follows:
// if ".3mic" suffix exists: set as intelligo
// else set as "none"
func updateAudioBeamformingTypeLegacy(ctx context.Context, info *execs.ExecInfo) error {
	res, err := info.DefaultRunner()(ctx, info.GetExecTimeout(), cmdAudioBeamformingCheckLegacy)
	log.Debugf(ctx, "command \"%s\" shows: %s", cmdAudioBeamformingCheckLegacy, res)
	if err != nil {
		log.Debugf(ctx, "command \"%s\" got error: %s", cmdAudioBeamformingCheckLegacy, err)
		return errors.Annotate(err, "unable to find the type of audio beamforming on dut.").Err()
	}
	resTrimmed := strings.TrimSpace(res)
	suffixRegex := regexp.MustCompile(`\.(.*)`)
	resSuffix := suffixRegex.FindString(resTrimmed)

	log.Debugf(ctx, "suffix in output: \"%s\"", resSuffix)

	switch resSuffix {
	case ".3mic":
		info.GetChromeos().GetAudio().Beamforming = "intelligo"
	default:
		info.GetChromeos().GetAudio().Beamforming = "none"
	}
	return nil
}

// updateAudioBeamformingTypeExec updates the DUT's Audio Beamforming type
// based on cras_server_tool.
func updateAudioBeamformingTypeExec(ctx context.Context, info *execs.ExecInfo) error {
	res, err := info.DefaultRunner()(ctx, info.GetExecTimeout(), cmdAudioBeamformingCheck)
	if err != nil {
		log.Debugf(ctx, "command %q got error: %s", cmdAudioBeamformingCheck, err)
		// Fall back to legacy check.
		return updateAudioBeamformingTypeLegacy(ctx, info)
	}

	log.Debugf(ctx, "command %q: %q", cmdAudioBeamformingCheck, res)
	info.GetChromeos().GetAudio().Beamforming = strings.TrimSpace(res)
	return nil
}

func init() {
	execs.Register("cros_update_device_sku", updateDeviceSKUExec)
	execs.Register("cros_update_dlm_sku_id", updateDlmSkuIDInvExec)
	execs.Register("cros_is_audio_loopback_state_working", isAudioLoopBackStateWorkingExec)
	execs.Register("cros_update_audio_loopback_state_label", updateAudioLoopbackLabelExec)
	execs.Register("cros_update_audio_latency_toolkit_state", updateAudioLatencyToolkitStateExec)
	execs.Register("cros_update_audio_beamforming_type", updateAudioBeamformingTypeExec)
}
