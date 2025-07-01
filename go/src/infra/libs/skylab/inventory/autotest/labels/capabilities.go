// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package labels

import (
	"strings"

	"go.chromium.org/infra/libs/skylab/inventory"
)

func init() {
	converters = append(converters, boolCapabilitiesConverter)
	converters = append(converters, stringCapabilitiesConverter)
	converters = append(converters, otherCapabilitiesConverter)

	reverters = append(reverters, boolCapabilitiesReverter)
	reverters = append(reverters, stringCapabilitiesReverter)
	reverters = append(reverters, otherCapabilitiesReverter)
}

func boolCapabilitiesConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	c := ls.GetCapabilities()
	if c.GetAtrus() {
		labels = append(labels, "atrus")
	}
	if c.GetBluetooth() {
		labels = append(labels, "bluetooth")
	}
	if c.GetDetachablebase() {
		labels = append(labels, "detachablebase")
	}
	if c.GetFingerprint() {
		labels = append(labels, "fingerprint")
	}
	if c.GetFlashrom() {
		labels = append(labels, "flashrom")
	}
	if c.GetHotwording() {
		labels = append(labels, "hotwording")
	}
	if c.GetInternalDisplay() {
		labels = append(labels, "internal_display")
	}
	if c.GetLucidsleep() {
		labels = append(labels, "lucidsleep")
	}
	if c.GetTouchpad() {
		labels = append(labels, "touchpad")
	}
	if c.GetTouchscreen() {
		labels = append(labels, "touchscreen")
	}
	if c.GetWebcam() {
		labels = append(labels, "webcam")
	}
	return labels
}

func stringCapabilitiesConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	c := ls.GetCapabilities()
	if v := c.GetGpuFamily(); v != "" {
		lv := "gpu_family:" + v
		labels = append(labels, lv)
	}
	if v := c.GetGraphics(); v != "" {
		lv := "graphics:" + v
		labels = append(labels, lv)
	}
	if v := c.GetModem(); v != "" {
		lv := "modem:" + v
		labels = append(labels, lv)
	}
	if v := c.GetPower(); v != "" {
		lv := "power:" + v
		labels = append(labels, lv)
	}
	if v := c.GetStorage(); v != "" {
		lv := "storage:" + v
		labels = append(labels, lv)
	}
	if v := c.GetTelephony(); v != "" {
		lv := "telephony:" + v
		labels = append(labels, lv)
	}
	if v := c.GetStarfishSlotMapping(); v != "" {
		lv := "starfish_slot_mapping:" + v
		labels = append(labels, lv)
		// Starfish slot mapping is stored as a comma deliminated list containing slotID_carrier.
		// e.g. 0_att,1_tmobile,2_verizon.
		// We need to split this list and determine each carrier that the testbed supports.
		for _, slot := range strings.Split(v, ",") {
			slotArr := strings.Split(slot, "_")
			if len(slotArr) != 2 {
				continue
			}
			lv := "supported_carrier:" + slotArr[1]
			labels = append(labels, lv)
		}
	}
	return labels
}

func otherCapabilitiesConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	c := ls.GetCapabilities()
	if v := c.GetCarrier(); v != inventory.HardwareCapabilities_CARRIER_INVALID {
		const plen = 8 // len("CARRIER_")
		carrier := strings.ToLower(v.String()[plen:])
		labels = append(labels, "carrier:"+carrier)
		// Carrier should be included in "supported_carrier" along with starfish carriers.
		labels = append(labels, "supported_carrier:"+carrier)
	}
	for _, v := range c.GetVideoAcceleration() {
		const plen = 19 // len("VIDEO_ACCELERATION_")
		lv := "hw_video_acc_" + strings.ToLower(v.String()[plen:])
		labels = append(labels, lv)
	}
	if v := c.GetCbx(); v != inventory.HardwareCapabilities_CBX_STATE_UNSPECIFIED {
		if v == inventory.HardwareCapabilities_CBX_STATE_TRUE {
			labels = append(labels, "cbx:True")
		} else {
			labels = append(labels, "cbx:False")
		}
	}
	if v := c.GetCbxBranding(); v != inventory.HardwareCapabilities_CBX_BRANDING_UNSPECIFIED {
		if v == inventory.HardwareCapabilities_CBX_BRANDING_SOFT_BRANDING {
			labels = append(labels, "cbx_branding:soft")
		} else if v == inventory.HardwareCapabilities_CBX_BRANDING_HARD_BRANDING {
			labels = append(labels, "cbx_branding:hard")
		}
	}
	if v := c.GetGscChip(); v != inventory.HardwareCapabilities_GSC_CHIP_UNKNOWN {
		const plen = 9 // len("GSC_CHIP_")
		chip := strings.ToLower(v.String()[plen:])
		labels = append(labels, "gsc_chip:"+chip)
	}
	return labels
}

func boolCapabilitiesReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	c := ls.GetCapabilities()
	for i := 0; i < len(labels); i++ {
		v := labels[i]
		switch v {
		case "atrus":
			*c.Atrus = true
		case "bluetooth":
			*c.Bluetooth = true
		case "detachablebase":
			*c.Detachablebase = true
		case "fingerprint":
			*c.Fingerprint = true
		case "flashrom":
			*c.Flashrom = true
		case "hotwording":
			*c.Hotwording = true
		case "internal_display":
			*c.InternalDisplay = true
		case "lucidsleep":
			*c.Lucidsleep = true
		case "touchpad":
			*c.Touchpad = true
		case "touchscreen":
			*c.Touchscreen = true
		case "webcam":
			*c.Webcam = true
		default:
			continue
		}
		labels = removeLabel(labels, i)
		i--
	}
	return labels
}

func stringCapabilitiesReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	c := ls.GetCapabilities()
	for i := 0; i < len(labels); i++ {
		k, v := splitLabel(labels[i])
		switch k {
		case "gpu_family":
			*c.GpuFamily = v
		case "graphics":
			*c.Graphics = v
		case "modem":
			*c.Modem = v
		case "power":
			*c.Power = v
		case "storage":
			*c.Storage = v
		case "telephony":
			*c.Telephony = v
		case "starfish_slot_mapping":
			*c.StarfishSlotMapping = v
		default:
			continue
		}
		labels = removeLabel(labels, i)
		i--
	}
	return labels
}

func otherCapabilitiesReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	c := ls.GetCapabilities()
	for i := 0; i < len(labels); i++ {
		k, v := splitLabel(labels[i])
		switch k {
		case "carrier":
			vn := "CARRIER_" + strings.ToUpper(v)
			type t = inventory.HardwareCapabilities_Carrier
			vals := inventory.HardwareCapabilities_Carrier_value
			*c.Carrier = t(vals[vn])
		case "supported_carrier:":
			// Nothing to do, carriers is already preserved by either
			// "carrier" or "starfish_slot_mapping"
		case "cbx":
			if v == "True" {
				*c.Cbx = inventory.HardwareCapabilities_CBX_STATE_TRUE
			} else {
				*c.Cbx = inventory.HardwareCapabilities_CBX_STATE_FALSE
			}
		case "cbx_branding":
			if v == "soft" {
				*c.CbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_SOFT_BRANDING
			} else if v == "hard" {
				*c.CbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_HARD_BRANDING
			}
		case "gsc_chip":
			vn := "GSC_CHIP_" + strings.ToUpper(v)
			type t = inventory.HardwareCapabilities_GscChip
			vals := inventory.HardwareCapabilities_GscChip_value
			if c.GscChip == nil {
				c.GscChip = new(inventory.HardwareCapabilities_GscChip)
			}
			*c.GscChip = t(vals[vn])
		default:
			switch {
			case strings.HasPrefix(k, "hw_video_acc_"):
				const plen = 13 // len("hw_video_acc_")
				vn := "VIDEO_ACCELERATION_" + strings.ToUpper(k[plen:])
				type t = inventory.HardwareCapabilities_VideoAcceleration
				vals := inventory.HardwareCapabilities_VideoAcceleration_value
				c.VideoAcceleration = append(c.VideoAcceleration, t(vals[vn]))
			default:
				continue
			}
		}
		labels = removeLabel(labels, i)
		i--
	}
	return labels
}
