// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package osutil

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/golang/protobuf/proto"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"

	"infra/cros/lab_inventory/deviceconfig"
	"infra/libs/skylab/inventory"
	ufspb "infra/unifiedfleet/api/v1/models"
	device "infra/unifiedfleet/api/v1/models/chromeos/device"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsmanufacturing "infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
)

var (
	trueValue            bool   = true
	falseValue           bool   = false
	emptyString          string = ""
	invServoStateUnknown        = inventory.PeripheralState_UNKNOWN
)

var noArcBoardMap = map[string]bool{
	"banjo":            true,
	"buddy":            true,
	"candy":            true,
	"dell":             true,
	"enguarde":         true,
	"expresso":         true,
	"falco":            true,
	"fizz-labstation":  true,
	"fizz-moblab":      true,
	"gale":             true,
	"gnawty":           true,
	"guado":            true,
	"guado_labstation": true,
	"heli":             true,
	"kip":              true,
	"link":             true,
	"monroe":           true,
	"ninja":            true,
	"nyan_big":         true,
	"nyan_blaze":       true,
	"nyan_kitty":       true,
	"orco":             true,
	"panther":          true,
	"peach_pit":        true,
	"peppy":            true,
	"quawks":           true,
	"rikku":            true,
	"sumo":             true,
	"swanky":           true,
	"tidus":            true,
	"tricky":           true,
	"veyron_jack":      true,
	"veyron_mickey":    true,
	"veyron_rialto":    true,
	"veyron_speedy":    true,
	"whirlwind":        true,
	"winky":            true,
	"zako":             true,
}

var appMap = map[string]bool{
	"hotrod": true,
}

// pasitComponentsMap are the components allowed to be included as swarming labels in PASIT testbeds.
var pasitComponentsMap = map[labapi.PasitHost_Device_Type]bool{
	labapi.PasitHost_Device_DOCKING_STATION: true,
	labapi.PasitHost_Device_MONITOR:         true,
	labapi.PasitHost_Device_CAMERA:          true,
	labapi.PasitHost_Device_STORAGE:         true,
	labapi.PasitHost_Device_HID:             true,
	labapi.PasitHost_Device_NETWORK:         true,
	labapi.PasitHost_Device_HEADPHONE:       true,
	labapi.PasitHost_Device_SPEAKER:         true,
}

type attributes []*inventory.KeyValue

func (a *attributes) append(key string, value string) *attributes {
	if value == "" {
		return a
	}
	*a = append(*a, &inventory.KeyValue{
		Key:   &key,
		Value: &value,
	})
	return a
}

func setDutPeripherals(labels *inventory.SchedulableLabels, d *chromeosLab.Peripherals) {
	if d == nil {
		return
	}

	p := labels.Peripherals
	c := labels.Capabilities
	hint := labels.TestCoverageHints

	p.AudioBoard = &falseValue
	if chameleon := d.GetChameleon(); chameleon != nil {
		if chameleon.GetHostname() != "" {
			p.Chameleon = &trueValue
		}
		for _, c := range chameleon.GetChameleonPeripherals() {
			cType := inventory.Peripherals_ChameleonType(c)
			if cType != inventory.Peripherals_CHAMELEON_TYPE_INVALID {
				p.ChameleonType = append(p.ChameleonType, cType)
			}
		}
		for _, c := range chameleon.GetChameleonConnectionTypes() {
			cType := inventory.Peripherals_ChameleonConnectionType(c)
			if cType != inventory.Peripherals_CHAMELEON_CONNECTION_TYPE_INVALID {
				p.ChameleonConnectionTypes = append(p.ChameleonConnectionTypes, cType)
			}
		}
		p.AudioBoard = &chameleon.AudioBoard
		p.AudioboxJackpluggerState = setAudioboxJackpluggerState(chameleon.GetAudioboxJackplugger())
	}
	p.TrrsType = setTrrsType(d.GetChameleon().GetTrrsType())

	p.Huddly = &falseValue
	if cameras := d.GetConnectedCamera(); cameras != nil {
		for _, c := range cameras {
			switch c.GetCameraType() {
			case chromeosLab.CameraType_CAMERA_HUDDLY:
				p.Huddly = &trueValue
			case chromeosLab.CameraType_CAMERA_PTZPRO2:
				p.Ptzpro2 = &trueValue
			}
		}
	}

	if audio := d.GetAudio(); audio != nil {
		p.AudioBox = &(audio.AudioBox)
		p.AudioCable = &(audio.AudioCable)
		c.Atrus = &(audio.Atrus)
	}

	if wifi := d.GetWifi(); wifi != nil {
		p.Wificell = &(wifi.Wificell)
		if wifi.GetAntennaConn() == chromeosLab.Wifi_CONN_CONDUCTIVE {
			p.Conductive = &trueValue
		} else {
			p.Conductive = &falseValue
		}
		if wifi.GetRouter() == chromeosLab.Wifi_ROUTER_802_11AX {
			p.Router_802_11Ax = &trueValue
		} else {
			p.Router_802_11Ax = &falseValue
		}
		p.WifiRouterFeatures = nil
		for _, feature := range wifi.GetWifiRouterFeatures() {
			p.WifiRouterFeatures = append(p.WifiRouterFeatures, inventory.Peripherals_WifiRouterFeature(feature.Number()))
		}
		p.WifiRouterModels = nil
		for _, wifiRouter := range wifi.GetWifiRouters() {
			routerModelForLabel := wifiRouter.GetModel()
			if routerModelForLabel == "" {
				routerModelForLabel = "UNKNOWN"
			}
			p.WifiRouterModels = append(p.WifiRouterModels, routerModelForLabel)
		}
	}

	if touch := d.GetTouch(); touch != nil {
		p.Mimo = &(touch.Mimo)
	}

	carrierKey := fmt.Sprintf("CARRIER_%s", strings.ToUpper(d.GetCarrier()))
	carrier := inventory.HardwareCapabilities_Carrier(inventory.HardwareCapabilities_Carrier_value[carrierKey])
	c.Carrier = &carrier

	c.SupportedCarriers = make([]inventory.HardwareCapabilities_Carrier, len(d.GetSupportedCarriers()))
	for i, car := range d.GetSupportedCarriers() {
		carrierKey := fmt.Sprintf("CARRIER_%s", strings.ToUpper(car))
		carrier := inventory.HardwareCapabilities_Carrier(inventory.HardwareCapabilities_Carrier_value[carrierKey])
		c.SupportedCarriers[i] = carrier
	}

	p.Camerabox = &(d.Camerabox)

	hint.ChaosDut = &(d.Chaos)
	for _, c := range d.GetCable() {
		switch c.GetType() {
		case chromeosLab.CableType_CABLE_AUDIOJACK:
			hint.TestAudiojack = &trueValue
		case chromeosLab.CableType_CABLE_USBAUDIO:
			hint.TestUsbaudio = &trueValue
		case chromeosLab.CableType_CABLE_USBPRINTING:
			hint.TestUsbprinting = &trueValue
		case chromeosLab.CableType_CABLE_HDMIAUDIO:
			hint.TestHdmiaudio = &trueValue
		}
	}

	if servo := d.GetServo(); servo != nil {
		servoType := servo.GetServoType()
		p.ServoType = &servoType
		p.ServoComponent = servo.GetServoComponent()
		setServoTopology(p, servo.GetServoTopology())
	}

	if facing := d.GetCameraboxInfo().GetFacing(); facing != chromeosLab.Camerabox_FACING_UNKNOWN {
		v1Facing := inventory.Peripherals_CameraboxFacing(facing)
		p.CameraboxFacing = &v1Facing
	}
	if light := d.GetCameraboxInfo().GetLight(); light != chromeosLab.Camerabox_LIGHT_UNKNOWN {
		v1Light := inventory.Peripherals_CameraboxLight(light)
		p.CameraboxLight = &v1Light
	}

	p.SmartUsbhub = &(d.SmartUsbhub)
	c.StarfishSlotMapping = &(d.StarfishSlotMapping)

	if h := d.GetPasitHost2(); h != nil {
		count := make(map[labapi.PasitHost_Device_Type]int)
		p.PasitComponents = []string{}
		for _, dev := range h.GetDevices() {
			dType := dev.GetType()
			// Skip not allowlisted components.
			if !pasitComponentsMap[dType] {
				continue
			}

			count[dType]++
			p.PasitComponents = append(p.PasitComponents, fmt.Sprintf("%v-%d", dType, count[dType]))
		}
	}
}

func setServoTopology(p *inventory.Peripherals, st *chromeosLab.ServoTopology) {
	var t *inventory.ServoTopology
	if st != nil {
		stString := proto.MarshalTextString(st)
		t = &inventory.ServoTopology{}
		proto.UnmarshalText(stString, t)
	}
	p.ServoTopology = t
}

func setDutPools(labels *inventory.SchedulableLabels, inputPools []string) {
	for _, p := range inputPools {
		v, ok := inventory.SchedulableLabels_DUTPool_value[p]
		if ok {
			labels.CriticalPools = append(labels.CriticalPools, inventory.SchedulableLabels_DUTPool(v))
		} else {
			labels.SelfServePools = append(labels.SelfServePools, p)
		}

		if _, ok := appMap[p]; ok {
			labels.TestCoverageHints.HangoutApp = &trueValue
			labels.TestCoverageHints.MeetApp = &trueValue
		}
	}
}

func setManufacturingConfig(l *inventory.SchedulableLabels, m *ufsmanufacturing.ManufacturingConfig) {
	if m == nil {
		return
	}
	l.Phase = (*inventory.SchedulableLabels_Phase)(&(m.DevicePhase))
	wifiChip := m.GetWifiChip()
	l.WifiChip = &wifiChip
	hwidComponent := m.GetHwidComponent()
	l.HwidComponent = hwidComponent
}

func setDeviceConfig(labels *inventory.SchedulableLabels, d *device.Config) {
	c := labels.GetCapabilities()
	if d == nil {
		return
	}
	c.GpuFamily = &(d.GpuFamily)
	var graphics string
	switch d.Graphics {
	case device.Config_GRAPHICS_GL:
		graphics = "gl"
	case device.Config_GRAPHICS_GLE:
		graphics = "gles"
	}
	c.Graphics = &graphics

	for _, f := range d.GetHardwareFeatures() {
		switch f {
		case device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD:
			c.Detachablebase = &trueValue
		case device.Config_HARDWARE_FEATURE_FINGERPRINT:
			c.Fingerprint = &trueValue
		case device.Config_HARDWARE_FEATURE_FLASHROM:
			c.Flashrom = &trueValue
		case device.Config_HARDWARE_FEATURE_HOTWORDING:
			c.Hotwording = &trueValue
		case device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY:
			c.InternalDisplay = &trueValue
		case device.Config_HARDWARE_FEATURE_LUCID_SLEEP:
			c.Lucidsleep = &trueValue
		case device.Config_HARDWARE_FEATURE_WEBCAM:
			c.Webcam = &trueValue
		}
	}

	if st := d.GetStorage(); st != device.Config_STORAGE_UNSPECIFIED {
		// Extract the storge type, e.g. "STORAGE_SSD" -> "ssd".
		storage := strings.ToLower(strings.SplitAfterN(st.String(), "_", 2)[1])
		c.Storage = &storage
	}

	if videoAcc := d.GetVideoAccelerationSupports(); videoAcc != nil {
		var acc []inventory.HardwareCapabilities_VideoAcceleration
		for _, v := range videoAcc {
			acc = append(acc, inventory.HardwareCapabilities_VideoAcceleration(v))
		}
		c.VideoAcceleration = acc
	}

	// Set CTS_ABI & CTS_CPU.
	switch d.GetCpu() {
	case device.Config_X86, device.Config_X86_64:
		labels.CtsAbi = []inventory.SchedulableLabels_CTSABI{
			inventory.SchedulableLabels_CTS_ABI_X86,
		}
		labels.CtsCpu = []inventory.SchedulableLabels_CTSCPU{
			inventory.SchedulableLabels_CTS_CPU_X86,
		}
	case device.Config_ARM, device.Config_ARM64:
		labels.CtsAbi = []inventory.SchedulableLabels_CTSABI{
			inventory.SchedulableLabels_CTS_ABI_ARM,
		}
		labels.CtsCpu = []inventory.SchedulableLabels_CTSCPU{
			inventory.SchedulableLabels_CTS_CPU_ARM,
		}
	}

	// Set Form_Factor
	switch d.GetFormFactor() {
	case device.Config_FORM_FACTOR_CLAMSHELL:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_CLAMSHELL.Enum()
	case device.Config_FORM_FACTOR_CONVERTIBLE:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_CONVERTIBLE.Enum()
	case device.Config_FORM_FACTOR_DETACHABLE:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_DETACHABLE.Enum()
	case device.Config_FORM_FACTOR_CHROMEBASE:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_CHROMEBASE.Enum()
	case device.Config_FORM_FACTOR_CHROMEBOX:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_CHROMEBOX.Enum()
	case device.Config_FORM_FACTOR_CHROMEBIT:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_CHROMEBIT.Enum()
	case device.Config_FORM_FACTOR_CHROMESLATE:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_CHROMESLATE.Enum()
	default:
		c.FormFactor = inventory.HardwareCapabilities_FORM_FACTOR_UNSPECIFIED.Enum()
	}
}

func setConfigsFromMachine(l *inventory.SchedulableLabels, machine *ufspb.Machine) {
	// Setup sku from DLM
	dlmSkuID := machine.GetChromeosMachine().GetDlmSkuId()
	l.DlmSkuId = &dlmSkuID

	c := l.GetCapabilities()
	// Setup bluetooth
	if machine.GetChromeosMachine().GetHasWifiBt() {
		c.Bluetooth = &trueValue
	}
}

func setHwidData(l *inventory.SchedulableLabels, h *ufspb.HwidData) {
	sku := h.GetSku()
	l.HwidSku = &sku
	l.Variant = []string{
		h.GetVariant(),
	}

	p := l.GetPeripherals()
	if h.GetStylus() {
		p.Stylus = &trueValue
	}
	c := l.GetCapabilities()
	if h.GetTouchpad() {
		c.Touchpad = &trueValue
	}
	if h.GetTouchscreen() {
		c.Touchscreen = &trueValue
	}
}

func setLicenses(l *inventory.SchedulableLabels, lic []*chromeosLab.License) {
	l.Licenses = make([]*inventory.License, len(lic))
	for i, v := range lic {
		var t inventory.LicenseType
		switch v.Type {
		case chromeosLab.LicenseType_LICENSE_TYPE_MS_OFFICE_STANDARD:
			t = inventory.LicenseType_LICENSE_TYPE_MS_OFFICE_STANDARD
		case chromeosLab.LicenseType_LICENSE_TYPE_WINDOWS_10_PRO:
			t = inventory.LicenseType_LICENSE_TYPE_WINDOWS_10_PRO
		default:
			t = inventory.LicenseType_LICENSE_TYPE_UNSPECIFIED
		}
		l.Licenses[i] = &inventory.License{
			Type:       &t,
			Identifier: &v.Identifier,
		}
	}
}

func setModemInfo(l *inventory.SchedulableLabels, m *chromeosLab.ModemInfo) {
	p := inventory.NewModeminfo()
	imei := m.GetImei()
	p.Imei = &imei
	supported_bands := m.GetSupportedBands()
	p.SupportedBands = &supported_bands
	sim_count := m.GetSimCount()
	p.SimCount = &sim_count
	modelVariant := m.GetModelVariant()
	p.ModelVariant = &modelVariant
	var t inventory.ModemType
	mtype := m.GetType()
	switch mtype {
	case chromeosLab.ModemType_MODEM_TYPE_UNSUPPORTED:
		t = inventory.ModemType_MODEM_TYPE_UNSUPPORTED
	case chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180:
		t = inventory.ModemType_MODEM_TYPE_QUALCOMM_SC7180
	case chromeosLab.ModemType_MODEM_TYPE_FIBOCOMM_L850GL:
		t = inventory.ModemType_MODEM_TYPE_FIBOCOMM_L850GL
	case chromeosLab.ModemType_MODEM_TYPE_NL668:
		t = inventory.ModemType_MODEM_TYPE_NL668
	case chromeosLab.ModemType_MODEM_TYPE_FM350:
		t = inventory.ModemType_MODEM_TYPE_FM350
	case chromeosLab.ModemType_MODEM_TYPE_FM101:
		t = inventory.ModemType_MODEM_TYPE_FM101
	case chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7280:
		t = inventory.ModemType_MODEM_TYPE_QUALCOMM_SC7280
	case chromeosLab.ModemType_MODEM_TYPE_EM060:
		t = inventory.ModemType_MODEM_TYPE_EM060
	case chromeosLab.ModemType_MODEM_TYPE_RW101:
		t = inventory.ModemType_MODEM_TYPE_RW101
	case chromeosLab.ModemType_MODEM_TYPE_RW135:
		t = inventory.ModemType_MODEM_TYPE_RW135
	case chromeosLab.ModemType_MODEM_TYPE_LCUK54:
		t = inventory.ModemType_MODEM_TYPE_LCUK54
	default:
		t = inventory.ModemType_MODEM_TYPE_UNSPECIFIED
	}
	p.Type = &t
	l.Modeminfo = p
}

func setSimInfo(l *inventory.SchedulableLabels, sim []*chromeosLab.SIMInfo) {
	l.Siminfo = make([]*inventory.SIMInfo, len(sim))
	for i, v := range sim {
		s := inventory.NewSiminfo()
		sid := v.GetSlotId()
		s.SlotId = &sid
		var t inventory.SIMType
		stype := v.GetType()
		switch stype {
		case chromeosLab.SIMType_SIM_PHYSICAL:
			t = inventory.SIMType_SIM_PHYSICAL
		case chromeosLab.SIMType_SIM_DIGITAL:
			t = inventory.SIMType_SIM_DIGITAL
		default:
			t = inventory.SIMType_SIM_UNKNOWN
		}
		s.Type = &t
		eid := v.GetEid()
		s.Eid = &eid
		testesim := v.GetTestEsim()
		s.TestEsim = &testesim
		s.ProfileInfo = make([]*inventory.SIMProfileInfo, len(v.GetProfileInfo()))
		for j, p := range v.GetProfileInfo() {
			s.ProfileInfo[j] = inventory.NewSimprofileinfo()
			iccid := p.GetIccid()
			s.ProfileInfo[j].Iccid = &iccid
			pin := p.GetSimPin()
			s.ProfileInfo[j].SimPin = &pin
			puk := p.GetSimPuk()
			s.ProfileInfo[j].SimPuk = &puk
			var np inventory.NetworkProvider
			pname := p.GetCarrierName()
			switch pname {
			case chromeosLab.NetworkProvider_NETWORK_UNSUPPORTED:
				np = inventory.NetworkProvider_NETWORK_UNSUPPORTED
			case chromeosLab.NetworkProvider_NETWORK_TEST:
				np = inventory.NetworkProvider_NETWORK_TEST
			case chromeosLab.NetworkProvider_NETWORK_ATT:
				np = inventory.NetworkProvider_NETWORK_ATT
			case chromeosLab.NetworkProvider_NETWORK_TMOBILE:
				np = inventory.NetworkProvider_NETWORK_TMOBILE
			case chromeosLab.NetworkProvider_NETWORK_VERIZON:
				np = inventory.NetworkProvider_NETWORK_VERIZON
			case chromeosLab.NetworkProvider_NETWORK_SPRINT:
				np = inventory.NetworkProvider_NETWORK_SPRINT
			case chromeosLab.NetworkProvider_NETWORK_DOCOMO:
				np = inventory.NetworkProvider_NETWORK_DOCOMO
			case chromeosLab.NetworkProvider_NETWORK_SOFTBANK:
				np = inventory.NetworkProvider_NETWORK_SOFTBANK
			case chromeosLab.NetworkProvider_NETWORK_KDDI:
				np = inventory.NetworkProvider_NETWORK_KDDI
			case chromeosLab.NetworkProvider_NETWORK_RAKUTEN:
				np = inventory.NetworkProvider_NETWORK_RAKUTEN
			case chromeosLab.NetworkProvider_NETWORK_VODAFONE:
				np = inventory.NetworkProvider_NETWORK_VODAFONE
			case chromeosLab.NetworkProvider_NETWORK_EE:
				np = inventory.NetworkProvider_NETWORK_EE
			case chromeosLab.NetworkProvider_NETWORK_AMARISOFT:
				np = inventory.NetworkProvider_NETWORK_AMARISOFT
			case chromeosLab.NetworkProvider_NETWORK_ROGER:
				np = inventory.NetworkProvider_NETWORK_ROGER
			case chromeosLab.NetworkProvider_NETWORK_BELL:
				np = inventory.NetworkProvider_NETWORK_BELL
			case chromeosLab.NetworkProvider_NETWORK_TELUS:
				np = inventory.NetworkProvider_NETWORK_TELUS
			case chromeosLab.NetworkProvider_NETWORK_FI:
				np = inventory.NetworkProvider_NETWORK_FI
			default:
				np = inventory.NetworkProvider_NETWORK_OTHER
			}
			s.ProfileInfo[j].CarrierName = &np
			ownNumber := p.GetOwnNumber()
			s.ProfileInfo[j].OwnNumber = &ownNumber
		}
		l.Siminfo[i] = s
	}

	// Process additional state information
	p := l.GetPeripherals()
	var count int32
	simState := inventory.PeripheralState_UNKNOWN
	for _, si := range sim {
		for _, pi := range si.GetProfileInfo() {
			state := pi.GetState()
			if state == chromeosLab.SIMProfileInfo_UNSPECIFIED {
				continue
			}
			if state != chromeosLab.SIMProfileInfo_WORKING {
				simState = inventory.PeripheralState_BROKEN
				continue
			}
			count++
		}
	}

	// If we have at leasts one SIM and it is working then change from UNSPECIFIED to WORKING.
	if count > 0 && simState != inventory.PeripheralState_BROKEN {
		simState = inventory.PeripheralState_WORKING
	}
	p.WorkingSims = &count
	p.SimState = &simState
}

func setDutStateHelper(s chromeosLab.PeripheralState) *bool {
	var val bool
	if s == chromeosLab.PeripheralState_UNKNOWN || s == chromeosLab.PeripheralState_NOT_CONNECTED {
		val = false
	} else {
		val = true
	}
	return &val
}

func setPeripheralState(s chromeosLab.PeripheralState) *inventory.PeripheralState {
	target := inventory.PeripheralState_UNKNOWN
	if s != chromeosLab.PeripheralState_UNKNOWN {
		target = inventory.PeripheralState(s)
	}
	return &target
}

func setCr50Configs(l *inventory.SchedulableLabels, s *chromeosLab.DutState) {
	switch s.GetCr50Phase() {
	case chromeosLab.DutState_CR50_PHASE_PVT:
		l.Cr50Phase = inventory.SchedulableLabels_CR50_PHASE_PVT.Enum()
	case chromeosLab.DutState_CR50_PHASE_PREPVT:
		l.Cr50Phase = inventory.SchedulableLabels_CR50_PHASE_PREPVT.Enum()
	default:
		l.Cr50Phase = inventory.SchedulableLabels_CR50_PHASE_INVALID.Enum()
	}

	cr50Env := ""
	switch s.GetCr50KeyEnv() {
	case chromeosLab.DutState_CR50_KEYENV_PROD:
		cr50Env = "prod"
	case chromeosLab.DutState_CR50_KEYENV_DEV:
		cr50Env = "dev"
	}
	l.Cr50RoKeyid = &cr50Env
}

func setGpuId(l *inventory.SchedulableLabels, s *chromeosLab.DutState) {
	c := l.GetCapabilities()
	c.GpuId = &(s.GpuId)
}

func setHardwareState(s chromeosLab.HardwareState) *inventory.HardwareState {
	target := inventory.HardwareState_HARDWARE_UNKNOWN
	if s != chromeosLab.HardwareState_HARDWARE_UNKNOWN {
		target = inventory.HardwareState(s)
	}
	return &target
}

func setDutState(l *inventory.SchedulableLabels, s *chromeosLab.DutState) {
	if s == nil || l == nil || l.GetPeripherals() == nil {
		return
	}
	p := l.Peripherals
	p.ServoState = setPeripheralState(s.GetServo())
	p.Servo = setDutStateHelper(s.GetServo())
	p.ChameleonState = setPeripheralState(s.GetChameleon())
	p.AudioLoopbackDongle = setDutStateHelper(s.GetAudioLoopbackDongle())
	p.ServoUsbState = setHardwareState(s.GetServoUsbState())
	p.StorageState = setHardwareState(s.GetStorageState())
	p.BatteryState = setHardwareState(s.GetBatteryState())
	p.WifiState = setHardwareState(s.GetWifiState())
	p.BluetoothState = setHardwareState(s.GetBluetoothState())
	p.CellularModemState = setHardwareState(s.GetCellularModemState())
	p.StarfishState = setPeripheralState(s.GetStarfishState())
	p.RpmState = setPeripheralState(s.GetRpmState())
	p.PeripheralWifiState = setPeripheralState(s.GetWifiPeripheralState())
	p.PeripheralBtpeerState = setPeripheralState(s.GetPeripheralBtpeerState())
	p.HmrState = setPeripheralState(s.GetHmrState())
	p.AudioLatencyToolkitState = setPeripheralState(s.GetAudioLatencyToolkitState())
	p.AmtManagerState = setPeripheralState(s.GetAmtManagerState())

	if n := s.GetWorkingBluetoothBtpeer(); n > 0 {
		p.WorkingBluetoothBtpeer = &n
	}
	setCr50Configs(l, s)
	setGpuId(l, s)
}

// TODO(echoyang@): Add CBX branding
func setCbx(l *inventory.SchedulableLabels, machine *ufspb.Machine) {
	c := l.Capabilities

	// Cbx State
	var cbx inventory.HardwareCapabilities_CbxState
	if machine.GetChromeosMachine().GetIsCbx() {
		cbx = inventory.HardwareCapabilities_CBX_STATE_TRUE
	} else {
		cbx = inventory.HardwareCapabilities_CBX_STATE_FALSE
	}
	c.Cbx = &cbx

	// Cbx Branding
	var cbxBranding inventory.HardwareCapabilities_CbxBranding
	if machine.GetChromeosMachine().GetCbxFeatureType() == ufspb.ChassisXBrandType_HARD_BRANDED {
		cbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_HARD_BRANDING
	} else if machine.GetChromeosMachine().GetCbxFeatureType() == ufspb.ChassisXBrandType_SOFT_BRANDED_LEGACY || machine.GetChromeosMachine().GetCbxFeatureType() == ufspb.ChassisXBrandType_SOFT_BRANDED_WAIVER {
		cbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_SOFT_BRANDING
	} else {
		cbxBranding = inventory.HardwareCapabilities_CBX_BRANDING_UNSPECIFIED
	}
	c.CbxBranding = &cbxBranding
}

func setAudioboxJackpluggerState(s chromeosLab.Chameleon_AudioBoxJackPlugger) *inventory.Peripherals_AudioBoxJackPlugger {
	target := inventory.Peripherals_AUDIOBOX_JACKPLUGGER_UNSPECIFIED
	if s != chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_UNSPECIFIED {
		target = inventory.Peripherals_AudioBoxJackPlugger(s)
	}
	return &target
}

func setTrrsType(s chromeosLab.Chameleon_TRRSType) *inventory.Peripherals_TRRSType {
	target := inventory.Peripherals_TRRS_TYPE_UNSPECIFIED
	if s != chromeosLab.Chameleon_TRRS_TYPE_UNSPECIFIED {
		target = inventory.Peripherals_TRRSType(s)
	}
	return &target
}

func setPower(labels *inventory.SchedulableLabels, p *chromeosLab.Peripherals, d *device.Config) {
	c := labels.GetCapabilities()
	var power string
	if p.GetDolos().GetHostname() != "" {
		power = "dolos"
	} else {
		switch pr := d.GetPower(); pr {
		case device.Config_POWER_SUPPLY_AC_ONLY:
			power = "AC_only"
		case device.Config_POWER_SUPPLY_BATTERY:
			power = "battery"
		}
	}
	c.Power = &power
}

func createDutLabels(machine *ufspb.Machine, devConfig *device.Config, osType *inventory.SchedulableLabels_OSType) *inventory.SchedulableLabels {
	// Use GetXXX in case any object is nil.
	platform := machine.GetChromeosMachine().GetBuildTarget()
	brand := strings.ToLower(devConfig.GetId().GetBrandId().GetValue())
	model := machine.GetChromeosMachine().GetModel()
	variant := machine.GetChromeosMachine().GetSku()

	_, ok := noArcBoardMap[platform]
	arc := !ok

	labels := inventory.SchedulableLabels{
		Arc:               &arc,
		OsType:            osType,
		Platform:          &platform,
		Board:             &platform,
		Brand:             &brand,
		Model:             &model,
		Sku:               &variant,
		Capabilities:      &inventory.HardwareCapabilities{},
		Peripherals:       &inventory.Peripherals{},
		TestCoverageHints: &inventory.TestCoverageHints{},
	}

	ecTypeCros := inventory.SchedulableLabels_EC_TYPE_CHROME_OS
	mappedPlatform := deviceconfig.BoardToPlatformMap[platform]

	boardsHasCrosEc := stringset.NewFromSlice(crosEcTypeBoards...)
	if boardsHasCrosEc.Has(platform) || boardsHasCrosEc.Has(mappedPlatform) {
		labels.EcType = &ecTypeCros
	}
	// See b/343614317, IAD65 doesn't have wifi network setup for test yet.
	if machine.GetLocation().GetZone() != ufspb.Zone_ZONE_IAD65_OS {
		labels.WifiOnSite = &trueValue
	}
	return &labels
}

// AdaptToV1DutSpec adapts ChromeOSDeviceData to inventory.DeviceUnderTest of
// inventory v1 defined in
// https://chromium.googlesource.com/infra/infra/+/refs/heads/master/go/src/infra/libs/skylab/inventory/device.proto
func AdaptToV1DutSpec(data *ufspb.ChromeOSDeviceData) (dut *inventory.DeviceUnderTest, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Reason("Recovered from %v\n%s", r, debug.Stack()).Err()
		}
	}()

	if data.GetLabConfig() == nil {
		return nil, errors.Reason("chromeosdevicedata is nil to adapt").Err()
	}
	if data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut() != nil {
		return adaptV2DutToV1DutSpec(data)
	}
	if data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetLabstation() != nil {
		return adaptV2LabstationToV1DutSpec(data)
	}
	if data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDevboard() != nil {
		return adaptV2DevboardToV1DutSpec(data)
	}
	panic("We should never reach here!")
}

func adaptV2DutToV1DutSpec(data *ufspb.ChromeOSDeviceData) (*inventory.DeviceUnderTest, error) {
	lse := data.GetLabConfig()
	machine := data.GetMachine()
	devConfig := data.GetDeviceConfig()
	dut := lse.GetChromeosMachineLse().GetDeviceLse().GetDut()
	p := dut.GetPeripherals()
	sn := machine.GetSerialNumber()
	var attrs attributes
	attrs.
		append("HWID", machine.GetChromeosMachine().GetHwid()).
		append("powerunit_hostname", p.GetRpm().GetPowerunitName()).
		append("powerunit_outlet", p.GetRpm().GetPowerunitOutlet()).
		append("serial_number", sn).
		append("servo_host", p.GetServo().GetServoHostname()).
		append("servod_docker", p.GetServo().GetDockerContainerName()).
		append("servo_port", fmt.Sprintf("%v", p.GetServo().GetServoPort())).
		append("servo_serial", p.GetServo().GetServoSerial()).
		append("servo_type", p.GetServo().GetServoType()).
		append("servo_setup", p.GetServo().GetServoSetup().String()[len("SERVO_SETUP_"):]).
		append("servo_fw_channel", p.GetServo().GetServoFwChannel().String()[len("SERVO_FW_"):])

	osType := inventory.SchedulableLabels_OS_TYPE_INVALID
	if board := machine.GetChromeosMachine().GetBuildTarget(); board != "" {
		var found bool
		if osType, found = boardToOsTypeMapping[board]; !found {
			osType = inventory.SchedulableLabels_OS_TYPE_CROS
		}
	}

	labels := createDutLabels(machine, devConfig, &osType)

	setDutPools(labels, dut.GetPools())
	setLicenses(labels, dut.GetLicenses())
	setModemInfo(labels, dut.GetModeminfo())
	setSimInfo(labels, dut.GetSiminfo())
	setDutPeripherals(labels, p)
	setDutState(labels, data.GetDutState())
	setDeviceConfig(labels, devConfig)
	setPower(labels, p, devConfig)
	setManufacturingConfig(labels, data.GetManufacturingConfig())
	setHwidData(labels, data.GetHwidData())
	setCbx(labels, machine)
	// Bluetooth config will be overwritten here by DLM configs
	setConfigsFromMachine(labels, machine)

	if dut.GetHive() != "cloudbots" {
		// All non cloudbots are running on drone which is large by default.
		large := inventory.SchedulableLabels_BOT_SIZE_LARGE
		labels.BotSize = &large
	}

	id := machine.GetName()
	hostname := lse.GetName()
	hwid := machine.GetChromeosMachine().GetHwid()
	deviceUnderTest := &inventory.DeviceUnderTest{
		Common: &inventory.CommonDeviceSpecs{
			Id:           &id,
			SerialNumber: &sn,
			Hostname:     &hostname,
			Attributes:   attrs,
			Labels:       labels,
			// Duplicating hwid here for populating hwid to swarming dimensions in internal-print-bot-info
			Hwid: &hwid,
		},
	}
	return deviceUnderTest, nil
}

func adaptV2LabstationToV1DutSpec(data *ufspb.ChromeOSDeviceData) (*inventory.DeviceUnderTest, error) {
	lse := data.GetLabConfig()
	machine := data.GetMachine()
	devConfig := data.GetDeviceConfig()
	l := lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation()
	sn := machine.GetSerialNumber()

	var attrs attributes
	attrs.
		append("HWID", machine.GetChromeosMachine().GetHwid()).
		append("powerunit_hostname", l.GetRpm().GetPowerunitName()).
		append("powerunit_outlet", l.GetRpm().GetPowerunitOutlet()).
		append("serial_number", sn)
	osType := inventory.SchedulableLabels_OS_TYPE_LABSTATION
	labels := createDutLabels(machine, devConfig, &osType)
	// Hardcode labstation labels.
	labels.Platform = &emptyString
	acOnly := "AC_only"
	carrierInvalid := inventory.HardwareCapabilities_CARRIER_INVALID
	labels.Capabilities = &inventory.HardwareCapabilities{
		Atrus:               &falseValue,
		Bluetooth:           &falseValue,
		Carrier:             &carrierInvalid,
		Detachablebase:      &falseValue,
		Fingerprint:         &falseValue,
		Flashrom:            &falseValue,
		GpuFamily:           &emptyString,
		GpuId:               &emptyString,
		Graphics:            &emptyString,
		Hotwording:          &falseValue,
		InternalDisplay:     &falseValue,
		Lucidsleep:          &falseValue,
		Modem:               &emptyString,
		Power:               &acOnly,
		StarfishSlotMapping: &emptyString,
		Storage:             &emptyString,
		Telephony:           &emptyString,
		Webcam:              &falseValue,
		Touchpad:            &falseValue,
		Touchscreen:         &falseValue,
	}
	cr50PhaseInvalid := inventory.SchedulableLabels_CR50_PHASE_INVALID
	labels.Cr50Phase = &cr50PhaseInvalid
	labels.Cr50RoKeyid = &emptyString
	labels.Cr50RoVersion = &emptyString
	labels.Cr50RwKeyid = &emptyString
	labels.Cr50RwVersion = &emptyString
	ecTypeInvalid := inventory.SchedulableLabels_EC_TYPE_INVALID
	labels.EcType = &ecTypeInvalid
	labels.WifiChip = &emptyString

	labels.Peripherals = &inventory.Peripherals{
		AudioBoard:          &falseValue,
		AudioBox:            &falseValue,
		AudioLoopbackDongle: &falseValue,
		Chameleon:           &falseValue,
		ChameleonType:       []inventory.Peripherals_ChameleonType{inventory.Peripherals_CHAMELEON_TYPE_INVALID},
		Conductive:          &falseValue,
		Huddly:              &falseValue,
		Mimo:                &falseValue,
		Servo:               &falseValue,
		ServoState:          &invServoStateUnknown,
		SmartUsbhub:         &falseValue,
		Stylus:              &falseValue,
		Camerabox:           &falseValue,
		Wificell:            &falseValue,
		Router_802_11Ax:     &falseValue,
	}

	labels.TestCoverageHints = &inventory.TestCoverageHints{
		ChaosDut:        &falseValue,
		ChaosNightly:    &falseValue,
		Chromesign:      &falseValue,
		HangoutApp:      &falseValue,
		MeetApp:         &falseValue,
		RecoveryTest:    &falseValue,
		TestAudiojack:   &falseValue,
		TestHdmiaudio:   &falseValue,
		TestUsbaudio:    &falseValue,
		TestUsbprinting: &falseValue,
		UsbDetect:       &falseValue,
		UseLid:          &falseValue,
	}
	setHwidData(labels, data.GetHwidData())
	setDutState(labels, data.GetDutState())
	labels.Variant = nil
	setDutPools(labels, l.GetPools())
	id := machine.GetName()
	hostname := lse.GetName()
	deviceUnderTest := &inventory.DeviceUnderTest{
		Common: &inventory.CommonDeviceSpecs{
			Id:           &id,
			SerialNumber: &sn,
			Hostname:     &hostname,
			Attributes:   attrs,
			Labels:       labels,
		},
	}
	return deviceUnderTest, nil
}

func adaptV2DevboardToV1DutSpec(data *ufspb.ChromeOSDeviceData) (*inventory.DeviceUnderTest, error) {
	lse := data.GetLabConfig()
	machine := data.GetMachine()
	devboard := lse.GetChromeosMachineLse().GetDeviceLse().GetDevboard()
	servo := devboard.GetServo()
	sn := machine.GetSerialNumber()
	var attrs attributes
	attrs.append("serial_number", sn)

	var devboardType inventory.SchedulableLabels_DevboardType
	var b string
	if machine.GetDevboard() != nil {
		if andreiBoard := machine.GetDevboard().GetAndreiboard(); andreiBoard != nil {
			attrs.append("devboard_type", "andreiboard")
			attrs.append("ultradebug_serial", andreiBoard.GetUltradebugSerial())
			devboardType = inventory.SchedulableLabels_DEVBOARD_TYPE_ANDREIBOARD
			b = "andreiboard"
		}
		if icetower := machine.GetDevboard().GetIcetower(); icetower != nil {
			attrs.append("devboard_type", "icetower")
			attrs.append("fingerprint_id", icetower.GetFingerprintId())
			devboardType = inventory.SchedulableLabels_DEVBOARD_TYPE_ICETOWER
			b = "icetower"
		}
		if dragonclaw := machine.GetDevboard().GetDragonclaw(); dragonclaw != nil {
			attrs.append("devboard_type", "dragonclaw")
			attrs.append("fingerprint_id", dragonclaw.GetFingerprintId())
			devboardType = inventory.SchedulableLabels_DEVBOARD_TYPE_DRAGONCLAW
			b = "dragonclaw"
		}
		// All Devboards need the `-devboard` identifier in the label
		if len(b) > 0 {
			b = fmt.Sprintf("%s-devboard", b)
		}
	}
	labels := &inventory.SchedulableLabels{
		DevboardType: &devboardType,
		Board:        &b,
		// Model is the same as board for devboards
		Model: &b,
	}
	setDutPools(labels, devboard.GetPools())
	setDutState(labels, data.GetDutState())

	if servo != nil && servo.GetServoHostname() != "" {
		// handle servo attributes
		attrs.
			append("servo_host", servo.GetServoHostname()).
			append("servod_docker", servo.GetDockerContainerName()).
			append("servo_port", fmt.Sprintf("%v", servo.GetServoPort())).
			append("servo_serial", servo.GetServoSerial()).
			append("servo_type", servo.GetServoType()).
			append("servo_setup", servo.GetServoSetup().String()[len("SERVO_SETUP_"):]).
			append("servo_fw_channel", servo.GetServoFwChannel().String()[len("SERVO_FW_"):])

		// handle servo labels
		labels.Peripherals = &inventory.Peripherals{}
		p := labels.Peripherals
		servoType := servo.GetServoType()
		p.ServoType = &servoType
		p.ServoComponent = servo.GetServoComponent()
		setServoTopology(p, servo.GetServoTopology())
	}
	id := machine.GetName()
	hostname := lse.GetName()
	deviceUnderTest := &inventory.DeviceUnderTest{
		Common: &inventory.CommonDeviceSpecs{
			Id:           &id,
			SerialNumber: &sn,
			Hostname:     &hostname,
			Attributes:   attrs,
			Labels:       labels,
		},
	}
	return deviceUnderTest, nil
}
