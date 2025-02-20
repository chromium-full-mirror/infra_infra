// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package labels

import (
	"strconv"
	"strings"

	"go.chromium.org/infra/libs/skylab/inventory"
)

func init() {
	converters = append(converters, simInfoConverter)
	reverters = append(reverters, simInfoReverter)
}

// simInfoConverter converts Skylab inventory SIMInfo labels to Autotest labels
func simInfoConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	for _, s := range ls.GetSiminfo() {
		sim_id := ""
		if v := s.GetSlotId(); v != 0 {
			sim_id = strconv.Itoa(int(v))
			lv := "sim_slot_id:" + sim_id
			labels = append(labels, lv)
		}
		if v := s.GetType(); v != inventory.SIMType_SIM_UNKNOWN {
			lv := "sim_" + sim_id + "_type:" + v.String()
			labels = append(labels, lv)
		}
		if eid := s.GetEid(); eid != "" {
			lv := "sim_" + sim_id + "_eid:" + eid
			labels = append(labels, lv)
		}
		if s.GetTestEsim() {
			lv := "sim_" + sim_id + "_test_esim:True"
			labels = append(labels, lv)
		}
		lv := "sim_" + sim_id + "_num_profiles:" + strconv.Itoa(len(s.GetProfileInfo()))
		labels = append(labels, lv)
		for j, p := range s.GetProfileInfo() {
			profile_id := strconv.Itoa(j)
			if k := p.GetIccid(); k != "" {
				lv := "sim_" + sim_id + "_" + profile_id + "_iccid:" + k
				labels = append(labels, lv)
			}
			if k := p.GetSimPin(); k != "" {
				lv := "sim_" + sim_id + "_" + profile_id + "_pin:" + k
				labels = append(labels, lv)
			}
			if k := p.GetSimPuk(); k != "" {
				lv := "sim_" + sim_id + "_" + profile_id + "_puk:" + k
				labels = append(labels, lv)
			}
			if k := p.GetCarrierName(); k != inventory.NetworkProvider_NETWORK_OTHER {
				lv := "sim_" + sim_id + "_" + profile_id + "_carrier_name:" + k.String()
				labels = append(labels, lv)
			}
			if k := p.GetOwnNumber(); k != "" {
				lv := "sim_" + sim_id + "_" + profile_id + "_own_number:" + k
				labels = append(labels, lv)
			}
		}
	}
	return labels
}

// siminfoReverter converts Autotest SIMInfo labels back to Skylab inventory labels
func simInfoReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	d := make(map[string][]string)
	for i := 0; i < len(labels); i++ {
		if strings.HasPrefix(labels[i], "sim_") {
			k, v := splitLabel(labels[i])
			d[k] = append(d[k], v)
			labels = removeLabel(labels, i)
			i--
		}
	}
	num_sim := len(d["sim_slot_id"])
	ls.Siminfo = make([]*inventory.SIMInfo, num_sim)
	for i, v := range d["sim_slot_id"] {
		sim_id := v
		s := inventory.NewSiminfo()
		if j, err := strconv.ParseInt(v, 10, 32); err == nil {
			id := int32(j)
			s.SlotId = &id
		}
		lv := "sim_" + sim_id + "_type"
		if v, ok := getLastStringValue(d, lv); ok {
			if p, ok := inventory.SIMType_value[v]; ok {
				stype := inventory.SIMType(p)
				s.Type = &stype
			}
			delete(d, lv)
		}
		lv = "sim_" + sim_id + "_eid"
		d = assignLastStringValueAndDropKey(d, s.Eid, lv)
		lv = "sim_" + sim_id + "_test_esim"
		d = assignLastBoolValueAndDropKey(d, s.TestEsim, lv)
		lv = "sim_" + sim_id + "_num_profiles"
		num_profiles := 0
		d = assignLastIntValueAndDropKey(d, &num_profiles, lv)
		s.ProfileInfo = make([]*inventory.SIMProfileInfo, num_profiles)
		for j := 0; j < num_profiles; j++ {
			s.ProfileInfo[j] = inventory.NewSimprofileinfo()
			profile_id := strconv.Itoa(j)
			lv = "sim_" + sim_id + "_" + profile_id + "_iccid"
			d = assignLastStringValueAndDropKey(d, s.ProfileInfo[j].Iccid, lv)
			lv = "sim_" + sim_id + "_" + profile_id + "_pin"
			d = assignLastStringValueAndDropKey(d, s.ProfileInfo[j].SimPin, lv)
			lv = "sim_" + sim_id + "_" + profile_id + "_puk"
			d = assignLastStringValueAndDropKey(d, s.ProfileInfo[j].SimPuk, lv)
			lv = "sim_" + sim_id + "_" + profile_id + "_carrier_name"
			if v, ok := getLastStringValue(d, lv); ok {
				if c, ok := inventory.NetworkProvider_value[v]; ok {
					pt := inventory.NetworkProvider(c)
					s.ProfileInfo[j].CarrierName = &pt
				}
				delete(d, lv)
			}
			lv = "sim_" + sim_id + "_" + profile_id + "_own_number"
			d = assignLastStringValueAndDropKey(d, s.ProfileInfo[j].OwnNumber, lv)
		}
		ls.Siminfo[i] = s
	}
	delete(d, "sim_slot_id")
	return labels
}
