// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

var featureLive = tlw.Cellular_SIMProfileInfo_FEATURE_LIVE_NETWORK
var featureSMS = tlw.Cellular_SIMProfileInfo_FEATURE_SMS

// carrierFeatures is a map of the carriers to the supported features.
// These features are used to determine what tests can be run against which SIMs
// in the lab, see go/cros-cellular-features for more information.
// File bugs against buganizer component: 979102.
var carrierFeatures = map[tlw.Cellular_NetworkProvider][]tlw.Cellular_SIMProfileInfo_Feature{
	tlw.Cellular_NETWORK_ATT:       {featureLive, featureSMS},
	tlw.Cellular_NETWORK_TMOBILE:   {featureLive, featureSMS},
	tlw.Cellular_NETWORK_TEST:      {featureLive},
	tlw.Cellular_NETWORK_VERIZON:   {featureLive},
	tlw.Cellular_NETWORK_SPRINT:    {featureLive},
	tlw.Cellular_NETWORK_DOCOMO:    {featureLive},
	tlw.Cellular_NETWORK_SOFTBANK:  {featureLive},
	tlw.Cellular_NETWORK_KDDI:      {featureLive},
	tlw.Cellular_NETWORK_RAKUTEN:   {featureLive},
	tlw.Cellular_NETWORK_VODAFONE:  {featureLive},
	tlw.Cellular_NETWORK_EE:        {featureLive},
	tlw.Cellular_NETWORK_AMARISOFT: {featureLive},
	tlw.Cellular_NETWORK_ROGER:     {featureLive},
	tlw.Cellular_NETWORK_BELL:      {featureLive},
	tlw.Cellular_NETWORK_TELUS:     {featureLive},
	tlw.Cellular_NETWORK_FI:        {featureLive},
	tlw.Cellular_NETWORK_CBRS:      {featureLive},
	tlw.Cellular_NETWORK_LINEMO:    {featureLive},
	tlw.Cellular_NETWORK_POVO:      {featureLive},
	tlw.Cellular_NETWORK_HANSHIN:   {featureLive},
}

// carrierOperatorIDs is a map of known operator_code to carrier mappings present in
// the lab, these are generally fixed and should only really need to be updated when
// adding new carriers to the lab..
var carrierOperatorIDs = map[string]tlw.Cellular_NetworkProvider{
	"00101":  tlw.Cellular_NETWORK_AMARISOFT,
	"001010": tlw.Cellular_NETWORK_AMARISOFT,
	"23415":  tlw.Cellular_NETWORK_VODAFONE,
	"23430":  tlw.Cellular_NETWORK_EE,
	"302220": tlw.Cellular_NETWORK_TELUS,
	"302720": tlw.Cellular_NETWORK_ROGER,
	"310260": tlw.Cellular_NETWORK_TMOBILE,
	"311882": tlw.Cellular_NETWORK_TMOBILE,
	"310280": tlw.Cellular_NETWORK_ATT,
	"310410": tlw.Cellular_NETWORK_ATT,
	"311480": tlw.Cellular_NETWORK_VERIZON,
	"44010":  tlw.Cellular_NETWORK_DOCOMO,
	"44011":  tlw.Cellular_NETWORK_RAKUTEN,
	"44020":  tlw.Cellular_NETWORK_SOFTBANK,
	"44051":  tlw.Cellular_NETWORK_KDDI,
}

// SupportedFeaturesForCarrier returns a list of the features supported for an
// individual cellular carrier.
func SupportedFeaturesForCarrier(carrier tlw.Cellular_NetworkProvider) []tlw.Cellular_SIMProfileInfo_Feature {
	if f, ok := carrierFeatures[carrier]; ok {
		return f
	}
	return []tlw.Cellular_SIMProfileInfo_Feature{}
}

// simInfo is a simplified version of the JSON output from ModemManager containing the SIM information.
type simInfo struct {
	SIM *struct {
		Properties *struct {
			ICCID        string `json:"iccid,omitempty"`
			EID          string `json:"eid,omitempty"`
			OperatorName string `json:"operator-name,omitempty"`
			OperatorCode string `json:"operator-code,omitempty"`
			TypeString   string `json:"sim-type,omitempty"`
		} `json:"properties,omitempty"`
	} `json:"sim,omitempty"`
}

// Type returns the type of SIM represented..
func (s *simInfo) Type() tlw.Cellular_SIMType {
	if s == nil || s.SIM == nil || s.SIM.Properties == nil {
		return tlw.Cellular_SIM_UNSPECIFIED
	}

	// If SIM has an EID or is explicitly eSIM then it is 'digital'
	if s.EID() != "" || strings.EqualFold(s.SIM.Properties.TypeString, "esim") {
		return tlw.Cellular_SIM_DIGITAL
	}
	return tlw.Cellular_SIM_PHYSICAL
}

// ICCID returns the SIMs ICCID.
func (s *simInfo) ICCID() string {
	if s == nil || s.SIM == nil || s.SIM.Properties == nil {
		return ""
	}

	// Modem manager may populate empty properties with "--"
	if s.SIM.Properties.ICCID == "--" {
		return ""
	}
	return s.SIM.Properties.ICCID
}

// EID returns the SIMs EID.
func (s *simInfo) EID() string {
	if s == nil || s.SIM == nil || s.SIM.Properties == nil {
		return ""
	}

	// Modem manager may populate empty properties with "--"
	if s.SIM.Properties.EID == "--" {
		return ""
	}
	return s.SIM.Properties.EID
}

// CarrierName returns the SIMs operator/carrier name.
func (s *simInfo) CarrierName() tlw.Cellular_NetworkProvider {
	if s == nil || s.SIM == nil || s.SIM.Properties == nil {
		return tlw.Cellular_NETWORK_UNSPECIFIED
	}

	// First try to determine Carrier Name from OperatorCode as it is more reliable,
	// if that fails then fallback to the OperatorName.
	oc := s.SIM.Properties.OperatorCode
	if _, ok := carrierOperatorIDs[oc]; ok {
		return carrierOperatorIDs[oc]
	}

	on := s.SIM.Properties.OperatorName
	switch {
	case strings.EqualFold(on, "AT&T"):
		return tlw.Cellular_NETWORK_ATT
	case strings.EqualFold(on, "VERIZON WIRELESS"):
		return tlw.Cellular_NETWORK_VERIZON
	case strings.EqualFold(on, "T-MOBILE"):
		return tlw.Cellular_NETWORK_TMOBILE
	case strings.EqualFold(on, "USIM"):
		return tlw.Cellular_NETWORK_AMARISOFT
	case strings.EqualFold(on, "NTT DOCOMO"):
		return tlw.Cellular_NETWORK_DOCOMO
	case strings.EqualFold(on, "EE"):
		return tlw.Cellular_NETWORK_EE
	case strings.EqualFold(on, "KDDI"):
		return tlw.Cellular_NETWORK_KDDI
	case strings.EqualFold(on, "RAKUTEN"):
		return tlw.Cellular_NETWORK_RAKUTEN
	case strings.EqualFold(on, "SOFTBANK"):
		return tlw.Cellular_NETWORK_SOFTBANK
	case strings.EqualFold(on, "GOOGLE FI"):
		return tlw.Cellular_NETWORK_FI
	case strings.EqualFold(on, "SPRINT"):
		return tlw.Cellular_NETWORK_SPRINT
	// vodafone may be 'vodafone UK' or some other variant.
	case strings.Contains(strings.ToLower(on), "vodafone"):
		return tlw.Cellular_NETWORK_VODAFONE
	// If empty, then return unspecified rather than unsupported.
	case on == "" || on == "--":
		return tlw.Cellular_NETWORK_UNSPECIFIED
	default:
		return tlw.Cellular_NETWORK_UNSUPPORTED
	}
}

// SwitchToMatchingSIMSlot switches to a SIM slot that matches the predicate.
func SwitchToMatchingSIMSlot(ctx context.Context, runner components.Runner, predicate func(*tlw.Cellular_SIMInfo) bool) error {
	// Check if the current SIM slot matches before moving on.
	simInfo, err := GetSIMInfo(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "get all sim info: failed to query info for current SIM slot").Err()
	}
	if predicate(simInfo) {
		return nil
	}

	modemInfo, err := WaitForModemInfo(ctx, runner, 15*time.Second)
	if err != nil {
		return errors.Annotate(err, "get all sim info: wait for ModemManager to export modem").Err()
	}
	for i := int32(0); i < modemInfo.SIMSlotCount(); i++ {
		if err := SwitchSIMSlot(ctx, runner, i+1); err != nil {
			return errors.Annotate(err, "get all sim info: switch to requested SIM slot").Err()
		}

		simInfo, err := GetSIMInfo(ctx, runner)
		if err != nil {
			return errors.Annotate(err, "get all sim info: failed to query info for sim slot: %d", i+1).Err()
		}

		if predicate(simInfo) {
			return nil
		}
	}
	return errors.Reason("switch to sim slot matching: failed to find a SIM slot matching predicate").Err()
}

// GetAllSIMInfo queries all SIM cards on the DUT and populates their information.
func GetAllSIMInfo(ctx context.Context, runner components.Runner) ([]*tlw.Cellular_SIMInfo, error) {
	modemInfo, err := WaitForModemInfo(ctx, runner, 15*time.Second)
	if err != nil {
		return nil, errors.Annotate(err, "get all sim info: wait for ModemManager to export modem").Err()
	}

	oldSlot := modemInfo.ActiveSIMSlot()
	defer func() {
		if err := SwitchSIMSlot(ctx, runner, oldSlot); err != nil {
			log.Errorf(ctx, "get all sim info: failed to restore original SIM slot: ", err)
		}
	}()

	res := make([]*tlw.Cellular_SIMInfo, 0)
	for i := int32(0); i < modemInfo.SIMSlotCount(); i++ {
		if err := SwitchSIMSlot(ctx, runner, i+1); err != nil {
			return nil, errors.Annotate(err, "get all sim info: switch to requested SIM slot").Err()
		}

		simInfo, err := GetSIMInfo(ctx, runner)
		if err != nil {
			return nil, errors.Annotate(err, "get all sim info: failed to query info for sim slot: %d", i+1).Err()
		}

		if simInfo != nil {
			res = append(res, simInfo)
		}
	}

	return res, nil
}

// SwitchSIMSlot switches the active SIM slot to the requested index.
func SwitchSIMSlot(ctx context.Context, runner components.Runner, slotNumber int32) error {
	modemInfo, err := WaitForModemInfo(ctx, runner, 15*time.Second)
	if err != nil {
		return errors.Annotate(err, "switch sim slot: wait for ModemManager to export modem").Err()
	}
	if modemInfo.ActiveSIMSlot() == slotNumber {
		log.Debugf(ctx, "sim slot already on %d", slotNumber)
		return nil
	}

	if _, err := runner(ctx, 5*time.Second, fmt.Sprintf("mmcli -m a --set-primary-sim-slot=%d", slotNumber)); err != nil {
		return errors.Annotate(err, "call mmcli").Err()
	}

	predicate := func(m *ModemInfo) error {
		if m.ActiveSIMSlot() != slotNumber {
			return errors.Reason("active modem slot not equal to %d", slotNumber).Err()
		}
		return nil
	}

	// Wait for up to 45 seconds for Modem to re-appear after slot switch.
	if _, err := WaitForModemInfo(ctx, runner, 120*time.Second, predicate); err != nil {
		return errors.Annotate(err, "switch sim slot: wait for ModemManager to export modem").Err()
	}

	return nil
}

// GetSIMInfo queries the SIM information in the current slot or returns nil if there is no SIM in the slot.
func GetSIMInfo(ctx context.Context, runner components.Runner) (*tlw.Cellular_SIMInfo, error) {
	modemInfo, err := WaitForModemInfo(ctx, runner, 15*time.Second)
	if err != nil {
		return nil, errors.Annotate(err, "get sim info: wait for ModemManager to export modem").Err()
	}

	// If this is a pSIM slot with no SIM, then there will be no SIM dbus path.
	// No sim detected in the slot -> return with no error.
	simID := modemInfo.ActiveSIMID()
	if simID == "" {
		log.Infof(ctx, "get sim info: no SIM detected in slot: %d, empty sim dbus path", modemInfo.ActiveSIMSlot())
		return nil, nil
	}

	output, err := runner(ctx, 5*time.Second, "mmcli -J -i "+simID)
	if err != nil {
		return nil, errors.Annotate(err, "get sim info: failed to call mmcli").Err()
	}

	si, err := parseSIMInfo(ctx, output)
	if err != nil {
		return nil, errors.Annotate(err, "get sim info: failed parsing mmcli response").Err()
	}

	// If this is an eSIM slot with no SIM profile, then there will be an empty ICCID even if the SIM dbus exists.
	if si.ICCID() == "" {
		log.Infof(ctx, "get sim info: no SIM detected in slot: %d, empty sim ICCID", modemInfo.ActiveSIMSlot())
		return nil, nil
	}

	// Verify some required information before returning
	if modemInfo.ActiveSIMSlot() == 0 {
		return nil, errors.Reason("get sim info: SIM slot ID is empty.").Err()
	}

	if si.Type() == tlw.Cellular_SIM_UNSPECIFIED {
		return nil, errors.Reason("get sim info: SIM type is empty.").Err()
	}

	return &tlw.Cellular_SIMInfo{
		SlotId: modemInfo.ActiveSIMSlot(),
		Type:   si.Type(),
		Eid:    si.EID(),
		ProfileInfos: []*tlw.Cellular_SIMProfileInfo{
			{
				Iccid:       si.ICCID(),
				CarrierName: si.CarrierName(),
				OwnNumber:   modemInfo.OwnNumber(),
			},
		},
	}, nil
}

// parseSIMInfo unmarshals the SIM properties json output from mmcli.
func parseSIMInfo(ctx context.Context, output string) (*simInfo, error) {
	info := &simInfo{}
	if err := json.Unmarshal([]byte(output), info); err != nil {
		return nil, err
	}
	return info, nil
}
