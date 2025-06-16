// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/labservice/internal/ufs/cache"
	"go.chromium.org/infra/cros/cmd/labservice/internal/ufs/wifisecret"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	manufacturing "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

type DeviceType int64

const (
	ChromeOSDevice DeviceType = iota
	AndroidDevice
)

// Inventory builds the DutTopology object from UFS.
type Inventory struct {
	client       ufsapi.FleetClient
	cacheLocator *cache.Locator
}

func NewInventory(c ufsapi.FleetClient, cl *cache.Locator) *Inventory {
	return &Inventory{
		client:       c,
		cacheLocator: cl,
	}
}

type deviceInfo struct {
	deviceType          DeviceType
	machine             *ufspb.Machine
	machineLse          *ufspb.MachineLSE
	manufactoringConfig *manufacturing.ManufacturingConfig
	hwidData            *ufspb.HwidData
	dutState            *lab.DutState
}

// GetDutTopology returns a DutTopology constructed from UFS.
// The returned error, if any, has gRPC status information.
func (inv *Inventory) GetDutTopology(ctx context.Context, id string) (*labapi.DutTopology, error) {
	deviceInfos, err := inv.getAllDevicesInfo(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "get dut topology: ID %q: %s", id, err)
	}
	wf, err := wifisecret.NewFinder(ctx, inv.client)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "get dut topology: ID %q: %s", id, err)
	}
	defer wf.Close()

	dt := &labapi.DutTopology{
		Id: &labapi.DutTopology_Id{Value: id},
	}
	for _, deviceInfo := range deviceInfos {
		d, err := inv.makeDutProto(deviceInfo)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "get dut topology: ID %q: %s", id, err)
		}
		w, err := wf.GetSecretForMachineLSE(ctx, deviceInfo.machineLse)
		if err != nil {
			log.Printf("Failed to get wifi secret for machine LSE %q: %s", id, err)
		}
		d.WifiSecret = w
		dt.Duts = append(dt.Duts, d)
	}
	return dt, nil
}

// getAllDevicesInfo fetches inventory entry of all DUTs / attached devices by a resource name.
func (inv *Inventory) getAllDevicesInfo(ctx context.Context, resourceName string) ([]*deviceInfo, error) {
	resp, err := inv.getDeviceData(ctx, resourceName)
	if err != nil {
		return nil, err
	}
	if resp.GetResourceType() == ufsapi.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT {
		return inv.getSchedulingUnitInfo(ctx, resp.GetSchedulingUnit().GetMachineLSEs())
	}
	di, err := toDeviceInfo(resp)
	if err != nil {
		return nil, fmt.Errorf("get all devices info: %w for %s", err, resourceName)
	}
	return []*deviceInfo{di}, nil
}

// getSchedulingUnitInfo fetches device info for every DUT / attached device in the scheduling unit.
func (inv *Inventory) getSchedulingUnitInfo(ctx context.Context, hostnames []string) ([]*deviceInfo, error) {
	// Get device info for every DUT / attached device in the scheduling unit.
	var deviceInfos []*deviceInfo
	for _, hostname := range hostnames {
		resp, err := inv.getDeviceData(ctx, hostname)
		if err != nil {
			return nil, err
		}
		di, err := toDeviceInfo(resp)
		if err != nil {
			return nil, fmt.Errorf("get scheduling unit info: %w for %s", err, hostname)
		}
		deviceInfos = append(deviceInfos, di)
	}
	return deviceInfos, nil
}

// getDeviceData fetches a device entry.
func (inv *Inventory) getDeviceData(ctx context.Context, id string) (*ufsapi.GetDeviceDataResponse, error) {
	resp, err := inv.client.GetDeviceData(ctx, &ufsapi.GetDeviceDataRequest{Hostname: id})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// toDeviceInfo convert a device data response to DeviceInfo
// after validation. Returns error if the device type is different from ChromeOs
// or Android device.
func toDeviceInfo(resp *ufsapi.GetDeviceDataResponse) (*deviceInfo, error) {
	switch resp.GetResourceType() {
	case ufsapi.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		return &deviceInfo{
			deviceType:          ChromeOSDevice,
			machine:             resp.GetChromeOsDeviceData().GetMachine(),
			machineLse:          resp.GetChromeOsDeviceData().GetLabConfig(),
			manufactoringConfig: resp.GetChromeOsDeviceData().GetManufacturingConfig(),
			hwidData:            resp.GetChromeOsDeviceData().GetHwidData(),
			dutState:            resp.GetChromeOsDeviceData().GetDutState(),
		}, nil
	case ufsapi.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		return &deviceInfo{
			deviceType: AndroidDevice,
			machine:    resp.GetAttachedDeviceData().GetMachine(),
			machineLse: resp.GetAttachedDeviceData().GetLabConfig(),
		}, nil
	}
	return nil, fmt.Errorf("append device info: invalid device type (%s)", resp.GetResourceType())
}

// makeDutProto makes a DutTopology Dut protobuf.
func (inv *Inventory) makeDutProto(di *deviceInfo) (*labapi.Dut, error) {
	switch di.deviceType {
	case ChromeOSDevice:
		return inv.makeChromeOsDutProto(di)
	case AndroidDevice:
		return inv.makeAndroidDutProto(di)
	}
	return nil, errors.New("make dut proto: invalid device type for " + di.machineLse.GetHostname())
}

// makeChromeOsDutProto populates DutTopology proto for ChromeOS device.
func (inv *Inventory) makeChromeOsDutProto(di *deviceInfo) (*labapi.Dut, error) {
	if di.machine.GetDevboard() != nil {
		return inv.makeChromeOsDevboardProto(di)
	}
	lse := di.machineLse
	hostname := lse.GetHostname()
	if hostname == "" {
		return nil, errors.New("make chromeos dut proto: empty hostname")
	}
	cs, err := inv.cacheLocator.FindCacheServer(hostname, inv.client)
	if err != nil {
		return nil, fmt.Errorf("make chromeos dut proto: %w", err)
	}
	croslse := lse.GetChromeosMachineLse()
	if croslse == nil {
		return nil, errors.New("make chromeos dut proto: empty chromeos_machine_lse")
	}
	dlse := croslse.GetDeviceLse()
	if dlse == nil {
		return nil, errors.New("make chromeos dut proto: empty device_lse")
	}
	d := dlse.GetDut()
	if d == nil {
		return nil, errors.New("make chromeos dut proto: empty dut")
	}
	p := d.GetPeripherals()
	if p == nil {
		return nil, errors.New("make chromeos dut proto: empty peripherals")
	}

	return &labapi.Dut{
		Id: &labapi.Dut_Id{Value: hostname},
		DutType: &labapi.Dut_Chromeos{
			Chromeos: &labapi.Dut_ChromeOS{
				Ssh: &labapi.IpEndpoint{
					Address: hostname,
					Port:    22,
				},
				DutModel:       getDutModel(di),
				Servo:          getServo(p.GetServo(), di.dutState),
				Chameleon:      getChameleon(p, di.dutState),
				Audio:          getAudio(p),
				Wifi:           getWifi(p),
				Touch:          getTouch(p),
				Camerabox:      getCamerabox(p),
				Cables:         getCables(p),
				HwidComponent:  getHwidComponent(di.manufactoringConfig),
				BluetoothPeers: getBluetoothPeers(p),
				Sku:            di.hwidData.GetSku(),
				Hwid:           di.hwidData.GetHwid(),
				Phase:          getPhase(di.hwidData),
				SimInfos:       getSimInfo(d.GetSiminfo()),
				ModemInfo:      getModemInfo(d.GetModeminfo()),
				Cellular:       getCellular(p),
				PasitHost:      getPasit(p.GetPasit()),
				Rpm:            getRpm(p.GetRpm()),
			},
		},
		CacheServer: &labapi.CacheServer{
			Address: cs,
		},
		Pools: d.GetPools(),
	}, nil
}

func getPasit(in *lab.Pasit) *labapi.PasitHost {
	if in == nil {
		return nil
	}

	out := &labapi.PasitHost{
		Hostname: in.GetHostname(),
	}

	for _, d := range in.GetDevices() {
		newDevice := &labapi.PasitHost_Device{
			Id:    d.GetId(),
			Model: d.GetModel(),
			Type:  labapi.PasitHost_Device_Type(d.GetType()),
		}

		if d.GetPowerSupply() != nil {
			newDevice.PowerSupply = &labapi.PasitHost_Device_PowerSupply{
				Current: d.GetPowerSupply().GetCurrent(),
				Voltage: d.GetPowerSupply().GetVoltage(),
				Power:   d.GetPowerSupply().GetPower(),
			}
		}
		out.Devices = append(out.Devices, newDevice)
	}

	for _, c := range in.GetConnections() {
		out.Connections = append(out.Connections,
			&labapi.PasitHost_Connection{
				ParentPort: c.GetParentPort(),
				ParentId:   c.GetParentId(),
				ChildId:    c.GetChildId(),
				Speed:      c.GetSpeed(),
				Type:       c.GetType(),
			},
		)
	}

	return out
}

// makeChromeOsDevboardProto populates DutTopology proto for Devboard device.
func (inv *Inventory) makeChromeOsDevboardProto(di *deviceInfo) (*labapi.Dut, error) {
	lse := di.machineLse
	hostname := lse.GetHostname()
	if hostname == "" {
		return nil, errors.New("make devboard proto: empty hostname")
	}
	croslse := lse.GetChromeosMachineLse()
	if croslse == nil {
		return nil, errors.New("make devboard proto: empty chromeos_machine_lse")
	}
	dlse := croslse.GetDeviceLse()
	if dlse == nil {
		return nil, errors.New("make devboard proto: empty device_lse")
	}
	lsed := dlse.GetDevboard()
	if lsed == nil {
		return nil, errors.New("make devboard proto: empty devboard machinelse")
	}
	mdb := di.machine.GetDevboard()
	if mdb == nil {
		return nil, errors.New("Make devboard proto: emtpy devboard machine")
	}
	cs, err := inv.cacheLocator.FindCacheServer(hostname, inv.client)
	if err != nil {
		return nil, fmt.Errorf("make chromeos dut proto: %w", err)
	}
	ret := &labapi.Dut{
		Id: &labapi.Dut_Id{Value: hostname},
		DutType: &labapi.Dut_Devboard_{
			Devboard: &labapi.Dut_Devboard{
				Servo: getServo(lsed.GetServo(), di.dutState),
			},
		},
		CacheServer: &labapi.CacheServer{
			Address: cs,
		},
	}

	switch mdb.GetBoard().(type) {
	case *ufspb.Devboard_Andreiboard:
		ret.GetDevboard().BoardType = "andreiboard"
		ret.GetDevboard().UltradebugSerial = mdb.GetAndreiboard().GetUltradebugSerial()
	case *ufspb.Devboard_Icetower:
		ret.GetDevboard().BoardType = "icetower"
		ret.GetDevboard().FingerprintModuleId = mdb.GetIcetower().GetFingerprintId()
	case *ufspb.Devboard_Dragonclaw:
		ret.GetDevboard().BoardType = "dragonclaw"
		ret.GetDevboard().FingerprintModuleId = mdb.GetDragonclaw().GetFingerprintId()
	}

	return ret, nil
}

// makeAndroidDutProto populates DutTopology proto for Android device.
func (inv *Inventory) makeAndroidDutProto(di *deviceInfo) (*labapi.Dut, error) {
	machine := di.machine
	lse := di.machineLse
	hostname := lse.GetHostname()
	if hostname == "" {
		return nil, errors.New("make android dut proto: empty hostname")
	}
	androidLse := lse.GetAttachedDeviceLse()
	if androidLse == nil {
		return nil, errors.New("make android dut proto: empty attached_device_lse")
	}
	associatedHostname := androidLse.GetAssociatedHostname()
	if associatedHostname == "" {
		return nil, errors.New("make android dut proto: empty associated_hostname")
	}
	serialNumber := machine.GetSerialNumber()
	if serialNumber == "" {
		return nil, errors.New("make android dut proto: empty serial_number")
	}
	return &labapi.Dut{
		Id: &labapi.Dut_Id{Value: hostname},
		DutType: &labapi.Dut_Android_{
			Android: &labapi.Dut_Android{
				AssociatedHostname: &labapi.IpEndpoint{
					Address: associatedHostname,
				},
				Name:         hostname,
				SerialNumber: serialNumber,
				DutModel:     getDutModel(di),
			},
		},
	}, nil
}

func getDutModel(di *deviceInfo) *labapi.DutModel {
	machine := di.machine
	if di.deviceType == ChromeOSDevice {
		return &labapi.DutModel{
			BuildTarget: machine.GetChromeosMachine().GetBuildTarget(),
			ModelName:   machine.GetChromeosMachine().GetModel(),
		}
	}
	return &labapi.DutModel{
		BuildTarget: machine.GetAttachedDevice().GetBuildTarget(),
		ModelName:   machine.GetAttachedDevice().GetModel(),
	}
}

func getServo(s *lab.Servo, ds *lab.DutState) *labapi.Servo {
	if s != nil && s.GetServoHostname() != "" {
		servo := &labapi.Servo{
			Present: true,
			ServodAddress: &labapi.IpEndpoint{
				Address: s.GetServoHostname(),
				Port:    s.GetServoPort(),
			},
			Serial:        s.GetServoSerial(),
			ContainerName: s.GetDockerContainerName(),
		}
		servo.State = labapi.PeripheralState_BROKEN
		if ds.GetServo() == lab.PeripheralState_WORKING {
			servo.State = labapi.PeripheralState_WORKING
		}
		return servo
	}
	return nil
}

func getChameleon(p *lab.Peripherals, ds *lab.DutState) *labapi.Chameleon {
	c := p.GetChameleon()
	if c == nil {
		return nil
	}
	cham := &labapi.Chameleon{
		AudioBoard:  c.GetAudioBoard(),
		Peripherals: mapChameleonPeripherals(p, c),
		Hostname:    c.GetHostname(),
		Types:       mapChameleonTypes(p, c),
	}
	switch ds.GetChameleon() {
	case lab.PeripheralState_WORKING:
		cham.State = labapi.PeripheralState_WORKING
	case lab.PeripheralState_BROKEN:
		cham.State = labapi.PeripheralState_BROKEN
	case lab.PeripheralState_NOT_APPLICABLE:
		cham.State = labapi.PeripheralState_NOT_APPLICABLE
	}
	return cham
}

func mapChameleonPeripherals(p *lab.Peripherals, c *lab.Chameleon) []labapi.Chameleon_Peripheral {
	var res []labapi.Chameleon_Peripheral
forLoop:
	for _, cp := range c.GetChameleonPeripherals() {
		m := labapi.Chameleon_PERIPHERAL_UNSPECIFIED
		switch cp {
		case lab.ChameleonType_CHAMELEON_TYPE_INVALID:
			m = labapi.Chameleon_PERIPHERAL_UNSPECIFIED
		case lab.ChameleonType_CHAMELEON_TYPE_DP:
			m = labapi.Chameleon_DP
		case lab.ChameleonType_CHAMELEON_TYPE_HDMI:
			m = labapi.Chameleon_HDMI
		case lab.ChameleonType_CHAMELEON_TYPE_RPI:
			m = labapi.Chameleon_RPI
		// Skip V2, V3 which are not physical peripherals but chameleon types
		case lab.ChameleonType_CHAMELEON_TYPE_V2, lab.ChameleonType_CHAMELEON_TYPE_V3:
			continue forLoop
		}
		res = append(res, m)
	}
	return res
}

func mapChameleonTypes(p *lab.Peripherals, c *lab.Chameleon) []labapi.Chameleon_Type {
	var res []labapi.Chameleon_Type
	for _, cp := range c.GetChameleonPeripherals() {
		switch cp {
		case lab.ChameleonType_CHAMELEON_TYPE_V2:
			res = append(res, labapi.Chameleon_V2)
		case lab.ChameleonType_CHAMELEON_TYPE_V3:
			res = append(res, labapi.Chameleon_V3)
		}
	}
	return res
}

func getAudio(p *lab.Peripherals) *labapi.Audio {
	a := p.GetAudio()
	if a == nil {
		return nil
	}
	return &labapi.Audio{
		AudioBox: a.AudioBox,
		Atrus:    a.Atrus,
	}
}

func getWifi(p *lab.Peripherals) *labapi.Wifi {
	res := &labapi.Wifi{}
	w := p.GetWifi()
	if p.GetChaos() {
		res.Environment = labapi.Wifi_ROUTER_802_11AX
	} else if w != nil && w.GetWificell() {
		res.Environment = labapi.Wifi_WIFI_CELL
	} else if w != nil && w.GetRouter() == lab.Wifi_ROUTER_802_11AX {
		res.Environment = labapi.Wifi_ROUTER_802_11AX
	} else if w != nil {
		res.Environment = labapi.Wifi_STANDARD
	}
	// TODO(ivanbrovkovich): Do we still get antenna for Chaos and wificell?
	if w != nil {
		res.Antenna = &labapi.WifiAntenna{
			Connection: mapWifiAntenna(w.GetAntennaConn()),
		}
	}
	// Add WiFi AP info into dut topology.
	for _, r := range w.GetWifiRouters() {
		router := &labapi.WifiRouter{
			Hostname:          r.GetHostname(),
			Model:             r.GetModel(),
			SupportedFeatures: r.GetSupportedFeatures(),
			DeviceType:        r.GetDeviceType(),
			Rpm:               getRpm(r.GetRpm()),
		}
		switch r.GetState() {
		case lab.PeripheralState_WORKING:
			router.State = labapi.PeripheralState_WORKING
		case lab.PeripheralState_BROKEN:
			router.State = labapi.PeripheralState_BROKEN
		case lab.PeripheralState_UNKNOWN:
			router.State = labapi.PeripheralState_PERIPHERAL_STATE_UNSPECIFIED
		default:
			// Unknown -> Set as NA
			router.State = labapi.PeripheralState_NOT_APPLICABLE
		}
		res.WifiRouters = append(res.WifiRouters, router)
	}
	return res
}

func mapWifiAntenna(wa lab.Wifi_AntennaConnection) labapi.WifiAntenna_Connection {
	switch wa {
	case lab.Wifi_CONN_UNKNOWN:
		return labapi.WifiAntenna_CONNECTION_UNSPECIFIED
	case lab.Wifi_CONN_CONDUCTIVE:
		return labapi.WifiAntenna_CONDUCTIVE
	case lab.Wifi_CONN_OTA:
		return labapi.WifiAntenna_OTA
	}
	return labapi.WifiAntenna_CONNECTION_UNSPECIFIED
}

func getTouch(p *lab.Peripherals) *labapi.Touch {
	if t := p.GetTouch(); t != nil {
		return &labapi.Touch{
			Mimo: t.GetMimo(),
		}
	}
	return nil
}

func getCamerabox(p *lab.Peripherals) *labapi.Camerabox {
	if !p.GetCamerabox() {
		return nil
	}
	cb := p.GetCameraboxInfo()
	return &labapi.Camerabox{
		Facing: mapCameraFacing(cb.Facing),
	}
}

func mapCameraFacing(cf lab.Camerabox_Facing) labapi.Camerabox_Facing {
	switch cf {
	case lab.Camerabox_FACING_UNKNOWN:
		return labapi.Camerabox_FACING_UNSPECIFIED
	case lab.Camerabox_FACING_BACK:
		return labapi.Camerabox_BACK
	case lab.Camerabox_FACING_FRONT:
		return labapi.Camerabox_FRONT
	}
	return labapi.Camerabox_FACING_UNSPECIFIED
}

func getCables(p *lab.Peripherals) []*labapi.Cable {
	var ret []*labapi.Cable
	for _, c := range p.GetCable() {
		ret = append(ret, &labapi.Cable{
			Type: mapCables(c.GetType()),
		})
	}
	return ret
}

func mapCables(ct lab.CableType) labapi.Cable_Type {
	switch ct {
	case lab.CableType_CABLE_INVALID:
		return labapi.Cable_TYPE_UNSPECIFIED
	case lab.CableType_CABLE_AUDIOJACK:
		return labapi.Cable_AUDIOJACK
	case lab.CableType_CABLE_USBAUDIO:
		return labapi.Cable_USBAUDIO
	case lab.CableType_CABLE_USBPRINTING:
		return labapi.Cable_USBPRINTING
	case lab.CableType_CABLE_HDMIAUDIO:
		return labapi.Cable_HDMIAUDIO
	}
	return labapi.Cable_TYPE_UNSPECIFIED
}

func getHwidComponent(mf *manufacturing.ManufacturingConfig) []string {
	if mf != nil {
		return mf.GetHwidComponent()
	}
	return nil
}

func getBluetoothPeers(p *lab.Peripherals) []*labapi.BluetoothPeer {
	ret := []*labapi.BluetoothPeer{}
	for _, btp := range p.GetBluetoothPeers() {
		r := btp.GetRaspberryPi()
		if r == nil || r.GetHostname() == "" {
			continue
		}
		bp := &labapi.BluetoothPeer{
			Hostname: r.GetHostname(),
		}
		switch r.GetState() {
		case lab.PeripheralState_WORKING:
			bp.State = labapi.PeripheralState_WORKING
		case lab.PeripheralState_BROKEN:
			bp.State = labapi.PeripheralState_BROKEN
		}
		ret = append(ret, bp)
	}
	return ret
}

func getPhase(hd *ufspb.HwidData) labapi.Phase {
	for _, label := range hd.GetDutLabel().GetLabels() {
		if label.GetName() == "phase" {
			p := strings.ReplaceAll(strings.ToUpper(label.GetValue()), "-", "_")
			switch p {
			case "PVT2":
				return labapi.Phase_PVT_2
			case "DVT2":
				return labapi.Phase_DVT_2
			}
			if val, ok := labapi.Phase_value[p]; ok {
				return labapi.Phase(val)
			}
		}
	}
	return labapi.Phase_PHASE_UNSPECIFIED
}

func getCellular(src *lab.Peripherals) *labapi.Cellular {
	return &labapi.Cellular{
		Carrier: src.GetCarrier(),
	}
}

func getModemInfo(src *lab.ModemInfo) *labapi.ModemInfo {
	r := &labapi.ModemInfo{
		Imei:           src.GetImei(),
		SupportedBands: src.GetSupportedBands(),
		SimCount:       src.GetSimCount(),
		ModelVariant:   src.GetModelVariant(),
	}

	switch src.GetType() {
	case lab.ModemType_MODEM_TYPE_UNSPECIFIED:
		r.Type = labapi.ModemType_MODEM_TYPE_UNSPECIFIED
	case lab.ModemType_MODEM_TYPE_QUALCOMM_SC7180:
		r.Type = labapi.ModemType_MODEM_TYPE_QUALCOMM_SC7180
	case lab.ModemType_MODEM_TYPE_QUALCOMM_SC7280:
		r.Type = labapi.ModemType_MODEM_TYPE_QUALCOMM_SC7280
	case lab.ModemType_MODEM_TYPE_FIBOCOMM_L850GL:
		r.Type = labapi.ModemType_MODEM_TYPE_FIBOCOMM_L850GL
	case lab.ModemType_MODEM_TYPE_NL668:
		r.Type = labapi.ModemType_MODEM_TYPE_NL668
	case lab.ModemType_MODEM_TYPE_FM350:
		r.Type = labapi.ModemType_MODEM_TYPE_FM350
	case lab.ModemType_MODEM_TYPE_FM101:
		r.Type = labapi.ModemType_MODEM_TYPE_FM101
	case lab.ModemType_MODEM_TYPE_EM060:
		r.Type = labapi.ModemType_MODEM_TYPE_EM060
	case lab.ModemType_MODEM_TYPE_RW101:
		r.Type = labapi.ModemType_MODEM_TYPE_RW101
	case lab.ModemType_MODEM_TYPE_RW135:
		r.Type = labapi.ModemType_MODEM_TYPE_RW135
	case lab.ModemType_MODEM_TYPE_RW350:
		r.Type = labapi.ModemType_MODEM_TYPE_RW350
	case lab.ModemType_MODEM_TYPE_LCUK54:
		r.Type = labapi.ModemType_MODEM_TYPE_LCUK54
	default:
		r.Type = labapi.ModemType_MODEM_TYPE_UNSUPPORTED
	}
	return r
}

func getSimInfo(src []*lab.SIMInfo) []*labapi.SIMInfo {
	var r []*labapi.SIMInfo
	for _, s := range src {
		info := labapi.SIMInfo{
			SlotId:   s.GetSlotId(),
			Type:     labapi.SIMType(s.GetType()),
			Eid:      s.GetEid(),
			TestEsim: s.GetTestEsim(),
		}
		for _, p := range s.GetProfileInfo() {
			newPI := &labapi.SIMProfileInfo{
				Iccid:       p.GetIccid(),
				SimPin:      p.GetSimPin(),
				SimPuk:      p.GetSimPuk(),
				CarrierName: labapi.NetworkProvider(p.GetCarrierName()),
				OwnNumber:   p.GetOwnNumber(),
				Features:    make([]labapi.SIMProfileInfo_Feature, len(p.GetFeatures())),
			}
			for i, f := range p.GetFeatures() {
				newPI.Features[i] = labapi.SIMProfileInfo_Feature(f)
			}
			info.ProfileInfo = append(info.ProfileInfo, newPI)
		}
		r = append(r, &info)
	}
	return r
}

const (
	// The k8s cluster internal service name point to rpm frontend server.
	rpmServiceHost = "rpm-service"
	// The service port of rpm frontend server.
	rpmServicePort = 9999
)

func getRpm(src *lab.OSRPM) *labapi.RPM {
	if src != nil {
		rpm := &labapi.RPM{
			Present:           true,
			PowerUnitHostname: &labapi.IpEndpoint{Address: src.GetPowerunitName()},
			PowerUnitOutlet:   src.GetPowerunitOutlet(),
		}
		switch src.GetPowerunitType() {
		case lab.OSRPM_TYPE_SENTRY:
			rpm.Type = labapi.RPMType_RPM_TYPE_SENTRY
		case lab.OSRPM_TYPE_IP9850:
			rpm.Type = labapi.RPMType_RPM_TYPE_IP9850
		default:
			rpm.Type = labapi.RPMType_RPM_TYPE_UNKNOWN
		}
		// Other type of RPMs are only support the http based call direct to the power unit.
		if src.GetPowerunitType() == lab.OSRPM_TYPE_SENTRY {
			rpm.FrontendAddress = &labapi.IpEndpoint{
				Address: rpmServiceHost,
				Port:    rpmServicePort,
			}
		}
		return rpm
	}
	return nil
}
