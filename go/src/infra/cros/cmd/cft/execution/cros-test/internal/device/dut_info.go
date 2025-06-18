// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package device implements utilities to extract device information.
package device

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/tast/core/framework/protocol"
)

// DutInfo stores information of a ChromeOS DUT.
type DutInfo struct {
	Addr                string   // The address of the DUT.
	Role                string   // The role of the DUT.
	Servo               string   // The address of the servo.
	DutServer           string   // The address of the dutServer.
	LibsServer          string   // The address of the libsServer.
	ProvisionServer     string   // The address of the provision server
	DevboardServer      string   // The address of the devboard server.
	Board               string   // The board of the DUT
	Model               string   // The model of the DUT
	ServoHostname       string   // The hostname of the Servo paired with the DUT
	ServoPort           string   // The port of the Servo paired with the DUT
	ServoSerial         string   // The serial of the Servo paired with the DUT
	ChameleonAudio      bool     // If the DUT has a ChameleonAudio peripheral
	ChamelonPresent     bool     // If the DUT has a Chameleon peripheral
	ChamelonPeriphsList []string // The list of Chameleon peripherals
	AtrusAudio          bool     // If the DUT has a AtrusAudio label
	TouchMimo           bool     // If the DUT has a TouchMimo label
	CameraboxFacing     string   // The direction the camerabox is facing, ie "front" or "back"
	CableList           []string // The list of cables attached
	CarrierList         []string // the list of carriers
	HwIDList            []string // HwIDlist
	Sku                 string
	Phase               string
	BTPeers             int
	CacheServer         string
	HWID                string
	FrontendAddress     string
	PowerUnitHostName   string
	PowerUnitOutlet     string
	HydraHostName       string
	SimInfos            []string
	WifiSecretSsid      string
	WifiSecretSecurity  string
	WifiSecretPassword  string
	DUT                 *labapi.Dut
	IP                  string
}

// joinHostAndPort joins host and port to a single address.
// Example 1: "127.0.0.1" "" -> "127.0.0.1".
// Example 2: "0:0:0:0:0:ffff:7f00:1" "2" -> "[0:0:0:0:0:ffff:7f00:1]:2".
// Example 3: "0:0:0:0:0:ffff:7f00:1" "" -> 0:0:0:0:0:ffff:7f00:1"
func joinHostAndPort(endpoint *labapi.IpEndpoint) string {
	if endpoint == nil {
		return ""
	}
	if endpoint.Port == 0 {
		return endpoint.Address
	}
	return net.JoinHostPort(endpoint.Address, strconv.Itoa(int(endpoint.Port)))
}

// AndroidInfo stores information of a Android device.
type AndroidInfo struct {
	AssoicateAddr string      // A hostname of the device that the Android DUT is attached to.
	Serial        string      // A string created by adb to uniquely identify the device.
	ModelName     string      // The model name of the device.
	DUT           *labapi.Dut // The DUT topology.
}

// Address returns the address of a DUT.
// TODO: Remove this after no test drivers are using this.
func Address(device *api.CrosTestRequest_Device) (string, error) {
	if device == nil {
		return "", errors.New("requested device is nil")
	}
	dut := device.Dut
	if dut == nil {
		return "", errors.New("DUT is nil")
	}
	chromeOS := dut.GetChromeos()
	if chromeOS == nil {
		return "", fmt.Errorf("DUT does not have end point information: %v", dut)
	}
	return joinHostAndPort(chromeOS.Ssh), nil
}

// dutAddress extract the dut address from the Device information.
func dutAddress(dut *labapi.Dut) (string, error) {
	if dut.GetDevboard() != nil {
		// Ignore DUT address if the devboard server is set.
		return "", nil
	}
	crosSSH := dut.GetChromeos().GetSsh()
	if crosSSH == nil {
		return "", fmt.Errorf("DUT does not have end point information: %v", dut)
	}
	return joinHostAndPort(crosSSH), nil
}

// dutAddress extract the dut address from the Device information.
func dutIP(dut *labapi.Dut) (string, error) {
	if dut.GetDevboard() != nil {
		// Ignore DUT address if the devboard server is set.
		return "", nil
	}
	if dut.GetAndroid() != nil {
		return dut.GetAndroid().GetSerialNumber(), nil
	}
	crosSSH := dut.GetChromeos().GetSsh()
	if crosSSH == nil {
		return "", fmt.Errorf("DUT does not have end point information: %v", dut)
	}
	return crosSSH.Address, nil
}

// FillDUTInfo extracts DUT information from a device.
func FillDUTInfo(device *api.CrosTestRequest_Device, role string) (*DutInfo, error) {
	if device == nil {
		return nil, errors.New("requested device is nil")
	}
	dut := device.Dut
	if dut == nil {
		return nil, errors.New("DUT is nil")
	}
	addr, err := dutAddress(dut)
	if err != nil {
		return nil, fmt.Errorf("failed to get DUT address: %v", dut)
	}

	ip, err := dutIP(dut)
	if err != nil {
		return nil, fmt.Errorf("failed to get DUT ip: %v", dut)
	}

	cacheInfo := dut.GetCacheServer()
	chromeOS := dut.GetChromeos()

	// Servo address.
	var servo string
	var servoHostname string
	var servoPort string
	var servoSerial string
	if chromeOS.GetServo().GetServodAddress() != nil &&
		chromeOS.GetServo().GetState() != labapi.PeripheralState_BROKEN {
		servo = joinHostAndPort(chromeOS.GetServo().GetServodAddress())
		servoHostname = strings.ToLower(chromeOS.GetServo().GetServodAddress().GetAddress())
		if chromeOS.GetServo().GetServodAddress().GetPort() != 0 {
			servoPort = fmt.Sprintf("%v", chromeOS.GetServo().GetServodAddress().GetPort())
		}
		servoSerial = chromeOS.GetServo().GetSerial()
	}

	// Address to the frontend service for accessing the RPM.
	// The frontend service is an HTTP service that supports XMLRPC calls.
	frontendAddress := joinHostAndPort(chromeOS.GetRpm().GetFrontendAddress())
	// Hostname of a particular PDU that assigned to the DUT.
	powerUnitHostName := joinHostAndPort(chromeOS.GetRpm().GetPowerUnitHostname())
	// Outlet name/number assigned to the DUT
	powerUnitOutlet := chromeOS.GetRpm().GetPowerUnitOutlet()
	// (Optional) Hydra hostname if the PDU access require a hydra.
	hydraHostName := joinHostAndPort(chromeOS.GetRpm().GetHydraHostname())

	// DUT Server address.
	var dutServer string
	if device.DutServer != nil {
		dutServer = joinHostAndPort(device.DutServer)
	}
	// DUT Server address.
	var libsServer string
	if device.LibsServer != nil {
		libsServer = joinHostAndPort(device.LibsServer)
	}
	// Provision server address.
	var provisionServer string
	if device.ProvisionServer != nil {
		provisionServer = joinHostAndPort(device.ProvisionServer)
	}
	// Devboard server address.
	var devboardServer string
	if device.DevboardServer != nil {
		devboardServer = joinHostAndPort(device.DevboardServer)
	}

	board := chromeOS.GetDutModel().GetBuildTarget()
	model := chromeOS.GetDutModel().GetModelName()

	// - Chameleon

	var chameleonAudio bool
	var chamelonPresent bool
	var chamelonPeriphsList []string
	chameleonAudio = chromeOS.GetChameleon().GetAudioBoard()
	if len(chromeOS.GetChameleon().GetPeripherals()) > 0 {
		chamelonPresent = true
		for _, v := range chromeOS.GetChameleon().GetPeripherals() {
			lv := "chameleon:" + strings.ToLower(v.String())
			chamelonPeriphsList = append(chamelonPeriphsList, lv)
		}
	}

	atrusAudio := chromeOS.GetAudio().GetAtrus()

	touchMimo := chromeOS.GetTouch().GetMimo()

	var cameraboxFacing string
	if camerabox := chromeOS.GetCamerabox(); camerabox != nil {
		facing := camerabox.Facing
		cameraboxFacing = strings.ToLower(facing.String())
	}

	var cableList []string
	if cables := chromeOS.GetCables(); len(cables) > 0 {
		for _, v := range cables {
			// TODO: Figure out why this proto has an empty space at end
			// eg. USBAUDIO is returning "USBAUDIO "
			cableList = append(cableList, strings.Trim(strings.ToLower(v.String()), " "))
		}
	}

	var carriers []string
	if car := chromeOS.GetCellular(); car != nil {
		if len(car.GetOperators()) > 0 {
			for _, v := range car.Operators {
				lv := "carrier:" + strings.ToLower(v.String())
				carriers = append(carriers, lv)
			}
		}
		// Include DUT carrier label as a 'carrier:', 'Operators' refers to the individual
		// SIMs in DUT and is currently unused as it has been superseded by the SimInfo labels.
		if car.GetCarrier() != "" {
			lv := "carrier:" + strings.ToLower(car.GetCarrier())
			carriers = append(carriers, lv)
		}
	}

	var hwids []string
	if hwid := chromeOS.GetHwidComponent(); len(hwid) > 0 {
		for _, v := range hwid {
			// TODO: Figure out why this proto has an empty space at end
			// eg. USBAUDIO is returning "USBAUDIO "
			lv := "hwid_component:" + strings.ToLower(v)
			hwids = append(hwids, lv)
		}
	}

	sku := chromeOS.GetSku()

	phase := strings.ToUpper(chromeOS.GetPhase().String())

	hwid := chromeOS.GetHwid()

	btpeers := 0
	if peers := chromeOS.GetBluetoothPeers(); len(peers) > 0 {
		for _, v := range peers {
			state := v.State
			if strings.ToLower(state.String()) == "working" {
				btpeers++
			}
		}
	}

	cacheServer := ""
	if cacheInfo != nil {
		cacheServer = fmt.Sprintf("%v:%v", cacheInfo.GetAddress().GetAddress(), cacheInfo.GetAddress().GetPort())

	}

	wifiSecret := dut.GetWifiSecret()
	wifiSecretSsid := ""
	wifiSecretSecurity := ""
	wifiSecretPassword := ""

	if wifiSecret != nil {
		wifiSecretSsid = wifiSecret.GetSsid()
		wifiSecretSecurity = wifiSecret.GetSecurity()
		wifiSecretPassword = wifiSecret.GetPassword()
	}

	return &DutInfo{
		Addr:                addr,
		Role:                role,
		Servo:               servo,
		DutServer:           dutServer,
		LibsServer:          libsServer,
		ProvisionServer:     provisionServer,
		DevboardServer:      devboardServer,
		Board:               board,
		Model:               model,
		ServoHostname:       servoHostname,
		ServoPort:           servoPort,
		ServoSerial:         servoSerial,
		ChameleonAudio:      chameleonAudio,
		ChamelonPresent:     chamelonPresent,
		ChamelonPeriphsList: chamelonPeriphsList,
		AtrusAudio:          atrusAudio,
		TouchMimo:           touchMimo,
		CameraboxFacing:     cameraboxFacing,
		CableList:           cableList,
		CarrierList:         carriers,
		HwIDList:            hwids,
		Sku:                 sku,
		Phase:               phase,
		BTPeers:             btpeers,
		CacheServer:         cacheServer,
		HWID:                hwid,
		FrontendAddress:     frontendAddress,
		PowerUnitHostName:   powerUnitHostName,
		PowerUnitOutlet:     powerUnitOutlet,
		HydraHostName:       hydraHostName,
		SimInfos:            simInfoConverter(chromeOS.GetSimInfos()),
		WifiSecretSsid:      wifiSecretSsid,
		WifiSecretSecurity:  wifiSecretSecurity,
		WifiSecretPassword:  wifiSecretPassword,
		DUT:                 dut,
		IP:                  ip,
	}, nil
}

// AppendChromeOsLabels appends labels extracted from ChromeOS device.
func AppendChromeOsLabels(dut *DutInfo) (map[string]string, []string, error) {
	// attrMap is the map of attributes to be used for autotest hostinfo.
	// example: {"servo_host": "servohostname.cros"}
	attrMap := make(map[string]string)

	// labels is a list of labels describing the DUT to be used for autotest hostinfo.
	// example: "servo chameleon audio_board"
	var labels []string

	if dut.Board != "" {
		labels = append(labels, "board:"+strings.ToLower(dut.Board))
	}
	if dut.Model != "" {
		labels = append(labels, "model:"+strings.ToLower(dut.Model))
	}

	// - Servo
	if dut.Servo != "" {
		labels = append(labels, "servo")
	}
	if dut.ServoHostname != "" {
		attrMap["servo_host"] = dut.ServoHostname
	}
	if dut.ServoPort != "" {
		attrMap["servo_port"] = dut.ServoPort
	}
	if dut.ServoSerial != "" {
		attrMap["servo_serial"] = dut.ServoSerial
	}

	// HWID needs to be an attr
	if dut.HWID != "" {
		attrMap["HWID"] = dut.HWID
	}

	if dut.ChamelonPresent {
		labels = append(labels, "chameleon")
	}
	if dut.ChameleonAudio {
		labels = append(labels, "audio_board")
	}
	if len(dut.ChamelonPeriphsList) > 0 {
		labels = append(labels, dut.ChamelonPeriphsList...)
	}

	if dut.AtrusAudio {
		labels = append(labels, "atrus")
	}

	if dut.TouchMimo {
		labels = append(labels, "mimo")
	}

	// - Camerabox
	if dut.CameraboxFacing != "" {
		labels = append(labels, "camerabox_facing:"+dut.CameraboxFacing)
	}

	// - WifiSecret
	if dut.WifiSecretSsid != "" {
		labels = append(labels, "wifisecret_ssid:"+dut.WifiSecretSsid)
	}

	if dut.WifiSecretSecurity != "" {
		labels = append(labels, "wifisecret_security:"+dut.WifiSecretSecurity)
	}

	if dut.WifiSecretPassword != "" {
		labels = append(labels, "wifisecret_password:"+dut.WifiSecretPassword)
	}

	if len(dut.CableList) > 0 {
		labels = append(labels, dut.CableList...)
	}

	if len(dut.CarrierList) > 0 {
		labels = append(labels, dut.CarrierList...)
	}

	if len(dut.HwIDList) > 0 {
		labels = append(labels, dut.HwIDList...)
	}

	if dut.Sku != "" {
		labels = append(labels, "sku:"+dut.Sku)
	}

	if dut.Phase != "" {
		labels = append(labels, "phase:"+dut.Phase)
	}

	if dut.BTPeers > 0 {
		labels = append(labels, fmt.Sprintf("working_bluetooth_btpeer:%v", dut.BTPeers))
	}

	if len(dut.SimInfos) > 0 {
		labels = append(labels, dut.SimInfos...)
	}

	return attrMap, labels, nil
}

// simInfoConverter converts SIMInfo labels to Autotest labels
// converter logic is copied from go/src/infra/libs/skylab/inventory/autotest/labels/siminfo.go
func simInfoConverter(simInfos []*labapi.SIMInfo) []string {
	var labels []string
	for _, s := range simInfos {
		simID := ""
		if v := s.GetSlotId(); v != 0 {
			simID = strconv.Itoa(int(v))
			lv := "sim_slot_id:" + simID
			labels = append(labels, lv)
		}
		if v := s.GetType(); v != *labapi.SIMType_SIM_UNKNOWN.Enum() {
			lv := "sim_" + simID + "_type:" + v.String()
			labels = append(labels, lv)
		}
		if eid := s.GetEid(); eid != "" {
			lv := "sim_" + simID + "_eid:" + eid
			labels = append(labels, lv)
		}
		if s.GetTestEsim() {
			lv := "sim_" + simID + "_test_esim:True"
			labels = append(labels, lv)
		}
		lv := "sim_" + simID + "_num_profiles:" + strconv.Itoa(len(s.GetProfileInfo()))
		labels = append(labels, lv)
		for j, p := range s.GetProfileInfo() {
			profileID := strconv.Itoa(j)
			if k := p.GetIccid(); k != "" {
				lv := "sim_" + simID + "_" + profileID + "_iccid:" + k
				labels = append(labels, lv)
			}
			if k := p.GetSimPin(); k != "" {
				lv := "sim_" + simID + "_" + profileID + "_pin:" + k
				labels = append(labels, lv)
			}
			if k := p.GetSimPuk(); k != "" {
				lv := "sim_" + simID + "_" + profileID + "_puk:" + k
				labels = append(labels, lv)
			}
			if k := p.GetCarrierName(); k != *labapi.NetworkProvider_NETWORK_OTHER.Enum() {
				lv := "sim_" + simID + "_" + profileID + "_carrier_name:" + k.String()
				labels = append(labels, lv)
			}
			if k := p.GetOwnNumber(); k != "" {
				lv := "sim_" + simID + "_" + profileID + "_own_number:" + k
				labels = append(labels, lv)
			}
		}
	}
	return labels
}

// FillAndroidInfo extracts Android information from a device.
func FillAndroidInfo(device *api.CrosTestRequest_Device) *AndroidInfo {
	android := device.Dut.GetAndroid()
	return &AndroidInfo{
		AssoicateAddr: joinHostAndPort(android.GetAssociatedHostname()),
		Serial:        android.GetSerialNumber(),
		ModelName:     android.GetDutModel().GetModelName(),
		DUT:           device.Dut,
	}
}

// GenLabConfig gemerates DUT lab config proto.
func GenLabConfig(primary *DutInfo, companions []*DutInfo, androidCompanion []*AndroidInfo) (*protocol.DUTLabConfig, error) {
	chromeOSDUTLabConfig := make(map[string]*labapi.Dut)
	androidDUTLabConfig := make(map[string]*labapi.Dut)
	devboardDUTLabConfig := make(map[string]*labapi.Dut)

	if primary.DUT.GetChromeos() != nil {
		chromeOSDUTLabConfig[primary.Role] = primary.DUT
	} else if primary.DUT.GetDevboard() != nil {
		devboardDUTLabConfig[primary.Role] = primary.DUT
	}

	for _, c := range companions {
		if c.DUT.GetChromeos() != nil {
			chromeOSDUTLabConfig[c.Role] = c.DUT
		} else if c.DUT.GetDevboard() != nil {
			devboardDUTLabConfig[c.Role] = c.DUT
		}
	}

	for _, a := range androidCompanion {
		androidDUTLabConfig[a.AssoicateAddr] = a.DUT
	}

	return &protocol.DUTLabConfig{
		ChromeOSDUTLabConfig: chromeOSDUTLabConfig,
		AndroidDUTLabConfig:  androidDUTLabConfig,
		DevboardDUTLabConfig: devboardDUTLabConfig,
	}, nil

}

// Returns a list of all devices in a request, both companion and primary.
func Devices(req *api.CrosTestRequest) []*labapi.Dut {
	devices := []*labapi.Dut{}
	if req.GetPrimary().GetDut() != nil {
		devices = append(devices, req.GetPrimary().GetDut())
	}
	for _, c := range req.GetCompanions() {
		devices = append(devices, c.GetDut())
	}
	return devices
}

// DerviceSerials extracts serial information of all devices from a requests test.
func DerviceSerials(req *api.CrosTestRequest) ([]string, error) {
	var serials []string
	primary := req.GetPrimary()
	serial, err := deviceSerial(primary.GetDut())
	if err != nil {
		return nil, fmt.Errorf("failed to get serial information from primary dut: %w", err)
	}
	if serial != "" {
		serials = append(serials, serial)
	}
	for _, c := range req.GetCompanions() {
		serial, err := dutIP(c.GetDut())
		if err != nil {
			return nil, fmt.Errorf("failed to get serial information from companion dut: %w", err)
		}
		if serial != "" {
			serials = append(serials, serial)
		}
	}
	return serials, nil
}

// deviceSerial extract the serial informaiton of a dut.
func deviceSerial(dut *labapi.Dut) (string, error) {
	if dut.GetDevboard() != nil {
		// Ignore DUT address if the devboard server is set.
		return "", nil
	}
	if dut.GetAndroid() != nil {
		return dut.GetAndroid().GetSerialNumber(), nil
	}
	crosSSH := dut.GetChromeos().GetSsh()
	if crosSSH == nil {
		return "", fmt.Errorf("DUT does not have end point information: %v", dut)
	}
	// TODO: need to figure out to handle none non-default port.
	// Currently, we assume chromeos device serial is always <ip>:5555.
	return fmt.Sprintf("%s:5555", crosSSH.Address), nil
}

// Returns a list of all lsnexus endpoints in a request, both companion and primary.
func LSNexus(req *api.CrosTestRequest) map[string]string {
	addresses := make(map[string]string)
	if req.GetPrimary().GetLsnexusServer() != nil &&
		req.GetPrimary().GetDut().GetChromeos().GetServo() != nil &&
		req.GetPrimary().GetDut().GetChromeos().GetServo().GetState() != labapi.PeripheralState_BROKEN {
		a := lsnexusKey(req.GetPrimary().GetDut().GetChromeos().GetSsh())
		if a != "" {
			addresses[a] = address(req.GetPrimary().GetLsnexusServer())
		}
	}
	for _, c := range req.GetCompanions() {
		if c.GetLsnexusServer() == nil ||
			c.GetDut().GetChromeos().GetServo() == nil ||
			c.GetDut().GetChromeos().GetServo().GetState() == labapi.PeripheralState_BROKEN {
			continue
		}
		a := lsnexusKey(c.GetDut().GetChromeos().GetSsh())
		if a != "" {
			addresses[a] = address(c.GetLsnexusServer())
		}
	}
	return addresses
}

func lsnexusKey(endPoint *labapi.IpEndpoint) string {
	a := fmt.Sprintf("lsnexus_%s", endPoint.GetAddress())
	return a
}

func address(endPoint *labapi.IpEndpoint) string {
	a := endPoint.GetAddress()
	if endPoint.GetPort() != 0 {
		a = fmt.Sprintf("%s:%d", a, endPoint.GetPort())
	}
	return a
}
