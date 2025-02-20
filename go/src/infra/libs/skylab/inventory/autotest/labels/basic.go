// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package labels

import (
	"strconv"
	"strings"

	"go.chromium.org/infra/libs/skylab/inventory"
)

func init() {
	reverters = append(reverters, basicReverter)
	converters = append(converters, basicConverter)
}

func basicConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	if v := ls.GetBoard(); v != "" {
		lv := "board:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetModel(); v != "" {
		lv := "model:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetSku(); v != "" {
		lv := "device-sku:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetHwidSku(); v != "" {
		lv := "sku:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetBrand(); v != "" {
		lv := "brand-code:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetPlatform(); v != "" {
		lv := "platform:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetReferenceDesign(); v != "" {
		lv := "reference_design:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetWifiChip(); v != "" {
		lv := "wifi_chip:" + v
		labels = append(labels, lv)
	}
	switch v := ls.GetEcType(); v {
	case inventory.SchedulableLabels_EC_TYPE_CHROME_OS:
		labels = append(labels, "ec:cros")
	}
	if v := ls.GetOsType(); v != inventory.SchedulableLabels_OS_TYPE_INVALID {
		const plen = 8 // len("OS_TYPE_")
		lv := "os:" + strings.ToLower(v.String()[plen:])
		labels = append(labels, lv)
	}
	if v := ls.GetPhase(); v != inventory.SchedulableLabels_PHASE_INVALID {
		const plen = 6 // len("PHASE_")
		lv := "phase:" + v.String()[plen:]
		labels = append(labels, lv)
	}
	for _, v := range ls.GetVariant() {
		lv := "variant:" + v
		labels = append(labels, lv)
	}
	if v := ls.GetDlmSkuId(); v != "" {
		lv := "dlm-sku-id:" + v
		labels = append(labels, lv)
	}
	return labels
}

func basicReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	for i := 0; i < len(labels); i++ {
		k, v := splitLabel(labels[i])
		switch k {
		case "board":
			*ls.Board = v
		case "model":
			*ls.Model = v
		case "device-sku":
			*ls.Sku = v
		case "dlm-sku-id":
			*ls.DlmSkuId = v
		case "sku":
			*ls.HwidSku = v
		case "brand-code":
			*ls.Brand = v
		case "platform":
			*ls.Platform = v
		case "ec":
			switch v {
			case "cros":
				*ls.EcType = inventory.SchedulableLabels_EC_TYPE_CHROME_OS
			default:
				continue
			}
		case "os":
			vn := "OS_TYPE_" + strings.ToUpper(v)
			type t = inventory.SchedulableLabels_OSType
			vals := inventory.SchedulableLabels_OSType_value
			*ls.OsType = t(vals[vn])
		case "phase":
			vn := "PHASE_" + strings.ToUpper(v)
			type t = inventory.SchedulableLabels_Phase
			vals := inventory.SchedulableLabels_Phase_value
			*ls.Phase = t(vals[vn])
		case "reference_design":
			*ls.ReferenceDesign = v
		case "wifi_chip":
			*ls.WifiChip = v
		case "variant":
			if v != "" {
				ls.Variant = append(ls.Variant, v)
			}
		default:
			continue
		}
		labels = removeLabel(labels, i)
		i--
	}
	return labels
}

// assignLastStringValueAndDropKey assign the last string value matching the key to `to`
// and drop the key
func assignLastStringValueAndDropKey(d map[string][]string, to *string, key string) map[string][]string {
	if v, ok := getLastStringValue(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

// getLastStringValue return the last string value matching the key
func getLastStringValue(d map[string][]string, key string) (string, bool) {
	if vs, ok := d[key]; ok {
		if len(vs) > 0 {
			return vs[len(vs)-1], true
		}
		return "", false
	}
	return "", false
}

// assignLastBoolValueAndDropKey assign the last bool value matching the key to `to`
// and drop the key
func assignLastBoolValueAndDropKey(d map[string][]string, to *bool, key string) map[string][]string {
	if v, ok := getLastBoolValue(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

// getLastBoolValue return the last bool value matching the key
func getLastBoolValue(d map[string][]string, key string) (bool, bool) {
	if s, ok := getLastStringValue(d, key); ok {
		return strings.ToLower(s) == "true", true
	}
	return false, false
}

// assignLastInt32ValueAndDropKey assign the last int32 value matching the key to `to`
// and drop the key
func assignLastInt32ValueAndDropKey(d map[string][]string, to *int32, key string) map[string][]string {
	if v, ok := getLastInt32Value(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

// getLastInt32Value return the last int32 value matching the key
func getLastInt32Value(d map[string][]string, key string) (int32, bool) {
	if s, ok := getLastStringValue(d, key); ok {
		if c, err := strconv.ParseInt(s, 10, 32); err == nil {
			return int32(c), true
		}
		return int32(-1), false
	}
	return int32(-1), false
}

// assignLastIntValueAndDropKey assign the last int value matching the key to `to`
// and drop the key
func assignLastIntValueAndDropKey(d map[string][]string, to *int, key string) map[string][]string {
	if v, ok := getLastIntValue(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

// getLastIntValue return the last int value matching the key
func getLastIntValue(d map[string][]string, key string) (int, bool) {
	if s, ok := getLastStringValue(d, key); ok {
		if c, err := strconv.Atoi(s); err == nil {
			return c, true
		}
	}
	return -1, false
}
