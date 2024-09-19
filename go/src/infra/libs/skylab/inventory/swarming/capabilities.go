// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarming

import (
	"infra/libs/skylab/inventory"
)

func init() {
	converters = append(converters, boolCapabilitiesConverter)
	reverters = append(reverters, boolCapabilitiesReverter)
	converters = append(converters, stringCapabilitiesConverter)
	reverters = append(reverters, stringCapabilitiesReverter)
	converters = append(converters, otherCapabilitiesConverter)
	reverters = append(reverters, otherCapabilitiesReverter)

}

func boolCapabilitiesConverter(dims Dimensions, ls *inventory.SchedulableLabels) {
	c := ls.GetCapabilities()
	if c.GetAtrus() {
		dims["label-atrus"] = []string{"True"}
	}
	if c.GetBluetooth() {
		dims["label-bluetooth"] = []string{"True"}
	}
	if c.GetDetachablebase() {
		dims["label-detachablebase"] = []string{"True"}
	}
	if c.GetFingerprint() {
		dims["label-fingerprint"] = []string{"True"}
	}
	if c.GetFlashrom() {
		dims["label-flashrom"] = []string{"True"}
	}
	if c.GetHotwording() {
		dims["label-hotwording"] = []string{"True"}
	}
	if c.GetInternalDisplay() {
		dims["label-internal_display"] = []string{"True"}
	}
	if c.GetLucidsleep() {
		dims["label-lucidsleep"] = []string{"True"}
	}
	if c.GetTouchpad() {
		dims["label-touchpad"] = []string{"True"}
	}
	if c.GetTouchscreen() {
		dims["label-touchscreen"] = []string{"True"}
	}
	if c.GetWebcam() {
		dims["label-webcam"] = []string{"True"}
	}
}

func boolCapabilitiesReverter(ls *inventory.SchedulableLabels, d Dimensions) Dimensions {
	c := ls.Capabilities
	d = assignLastBoolValueAndDropKey(d, c.Atrus, "label-atrus")
	d = assignLastBoolValueAndDropKey(d, c.Bluetooth, "label-bluetooth")
	d = assignLastBoolValueAndDropKey(d, c.Detachablebase, "label-detachablebase")
	d = assignLastBoolValueAndDropKey(d, c.Fingerprint, "label-fingerprint")
	d = assignLastBoolValueAndDropKey(d, c.Flashrom, "label-flashrom")
	d = assignLastBoolValueAndDropKey(d, c.Hotwording, "label-hotwording")
	d = assignLastBoolValueAndDropKey(d, c.InternalDisplay, "label-internal_display")
	d = assignLastBoolValueAndDropKey(d, c.Lucidsleep, "label-lucidsleep")
	d = assignLastBoolValueAndDropKey(d, c.Touchpad, "label-touchpad")
	d = assignLastBoolValueAndDropKey(d, c.Touchscreen, "label-touchscreen")
	d = assignLastBoolValueAndDropKey(d, c.Webcam, "label-webcam")
	return d
}

func stringCapabilitiesConverter(dims Dimensions, ls *inventory.SchedulableLabels) {
	c := ls.GetCapabilities()
	if v := c.GetFingerprintBoard(); v != "" {
		dims["label-fingerprint_board"] = []string{v}
	}
	if v := c.GetFingerprintMcu(); v != "" {
		dims["label-fingerprint_mcu"] = []string{v}
	}
	if v := c.GetFingerprintSensor(); v != "" {
		dims["label-fingerprint_sensor"] = []string{v}
	}
	if v := c.GetGpuFamily(); v != "" {
		dims["label-gpu_family"] = []string{v}
	}
	if v := c.GetGpuId(); v != "" {
		dims["label-gpu_id"] = []string{v}
	}
	if v := c.GetGraphics(); v != "" {
		dims["label-graphics"] = []string{v}
	}
	if v := c.GetModem(); v != "" {
		dims["label-modem"] = []string{v}
	}
	if v := c.GetPower(); v != "" {
		dims["label-power"] = []string{v}
	}
	if v := c.GetStorage(); v != "" {
		dims["label-storage"] = []string{v}
	}
	if v := c.GetTelephony(); v != "" {
		dims["label-telephony"] = []string{v}
	}
	if v := c.GetStarfishSlotMapping(); v != "" {
		dims["label-starfish_slot_mapping"] = []string{v}
	}
}

func stringCapabilitiesReverter(ls *inventory.SchedulableLabels, d Dimensions) Dimensions {
	c := ls.Capabilities
	d = assignLastStringValueAndDropKey(d, c.FingerprintBoard, "label-fingerprint_board")
	d = assignLastStringValueAndDropKey(d, c.FingerprintMcu, "label-fingerprint_mcu")
	d = assignLastStringValueAndDropKey(d, c.FingerprintSensor, "label-fingerprint_sensor")
	d = assignLastStringValueAndDropKey(d, c.GpuFamily, "label-gpu_family")
	d = assignLastStringValueAndDropKey(d, c.GpuId, "label-gpu_id")
	d = assignLastStringValueAndDropKey(d, c.Graphics, "label-graphics")
	d = assignLastStringValueAndDropKey(d, c.Modem, "label-modem")
	d = assignLastStringValueAndDropKey(d, c.Power, "label-power")
	d = assignLastStringValueAndDropKey(d, c.Storage, "label-storage")
	d = assignLastStringValueAndDropKey(d, c.Telephony, "label-telephony")
	d = assignLastStringValueAndDropKey(d, c.StarfishSlotMapping, "label-starfish_slot_mapping")
	return d
}

func otherCapabilitiesConverter(dims Dimensions, ls *inventory.SchedulableLabels) {
	c := ls.GetCapabilities()
	if v := c.GetCarrier(); v != inventory.HardwareCapabilities_CARRIER_INVALID {
		dims["label-carrier"] = []string{v.String()}
	}
	for _, v := range c.GetSupportedCarriers() {
		if v != inventory.HardwareCapabilities_CARRIER_INVALID {
			appendDim(dims, "label-supported_carriers", v.String())
		}
	}
	if v := c.GetCbx(); v != inventory.HardwareCapabilities_CBX_STATE_UNSPECIFIED {
		if v == inventory.HardwareCapabilities_CBX_STATE_TRUE {
			dims["label-cbx"] = []string{"True"}
		} else {
			dims["label-cbx"] = []string{"False"}
		}
	}
	if v := c.GetCbxBranding(); v != inventory.HardwareCapabilities_CBX_BRANDING_UNSPECIFIED {
		if v == inventory.HardwareCapabilities_CBX_BRANDING_SOFT_BRANDING {
			dims["label-cbx_branding"] = []string{"soft"}
		} else if v == inventory.HardwareCapabilities_CBX_BRANDING_HARD_BRANDING {
			dims["label-cbx_branding"] = []string{"hard"}
		}
	}
	for _, v := range c.GetVideoAcceleration() {
		appendDim(dims, "label-video_acceleration", v.String())
	}
	if v := c.GetFormFactor(); v != inventory.HardwareCapabilities_FORM_FACTOR_UNSPECIFIED {
		dims["label-form_factor"] = []string{v.String()}
	}
}

func otherCapabilitiesReverter(ls *inventory.SchedulableLabels, d Dimensions) Dimensions {
	c := ls.Capabilities
	if v, ok := getLastStringValue(d, "label-carrier"); ok {
		if p, ok := inventory.HardwareCapabilities_Carrier_value[v]; ok {
			*c.Carrier = inventory.HardwareCapabilities_Carrier(p)
		}
		delete(d, "label-carrier")
	}

	c.SupportedCarriers = make([]inventory.HardwareCapabilities_Carrier, len(d["label-supported_carriers"]))
	for i, v := range d["label-supported_carriers"] {
		if p, ok := inventory.HardwareCapabilities_Carrier_value[v]; ok {
			c.SupportedCarriers[i] = inventory.HardwareCapabilities_Carrier(p)
		}
	}
	delete(d, "label-supported_carrier")

	if v, ok := getLastStringValue(d, "label-cbx"); ok {
		if v == "True" {
			*c.Cbx = inventory.HardwareCapabilities_CBX_STATE_TRUE
		} else {
			*c.Cbx = inventory.HardwareCapabilities_CBX_STATE_FALSE
		}
		delete(d, "label-cbx")
	}
	if v, ok := getLastStringValue(d, "label-cbx_branding"); ok {
		if v == "soft" {
			*c.CbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_SOFT_BRANDING
		} else if v == "hard" {
			*c.CbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_HARD_BRANDING
		} else {
			*c.CbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_UNSPECIFIED
		}
		delete(d, "label-cbx")
	}
	c.VideoAcceleration = make([]inventory.HardwareCapabilities_VideoAcceleration, len(d["label-video_acceleration"]))
	for i, v := range d["label-video_acceleration"] {
		if p, ok := inventory.HardwareCapabilities_VideoAcceleration_value[v]; ok {
			c.VideoAcceleration[i] = inventory.HardwareCapabilities_VideoAcceleration(p)
		}
	}
	delete(d, "label-video_acceleration")
	if v, ok := getLastStringValue(d, "label-form_factor"); ok {
		if p, ok := inventory.HardwareCapabilities_FormFactor_value[v]; ok {
			*c.FormFactor = inventory.HardwareCapabilities_FormFactor(p)
		}
		delete(d, "label-form_factor")
	}
	return d
}
