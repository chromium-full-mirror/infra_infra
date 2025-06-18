// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
)

type paramMap = map[string]string

// combinedParamMap combines the provided paramMaps and returns a new one.
func combinedParamMap(maps ...paramMap) paramMap {
	copy := paramMap{}
	for _, p := range maps {
		for k, v := range p {
			copy[k] = v
		}
	}
	return copy
}

type deviceParams struct {
	deviceId string
	params   paramMap
}

func deviceParamMap(params []deviceParams) map[string]paramMap {
	res := make(map[string]paramMap)
	for _, device := range params {
		res[device.deviceId] = device.params
	}
	return res
}

type MoblyTestConfig struct {
	TestBeds []*TestBed `yaml:"TestBeds"`
}

type TestBed struct {
	Name        string       `yaml:"Name"`
	TestParams  *TestParams  `yaml:"TestParams"`
	Controllers *Controllers `yaml:"Controllers"`
}

type TestParams struct {
	Params paramMap `yaml:",inline"`
}

type Controllers struct {
	OpenWrtDevices    []*OpenWrtDevice     `yaml:"OpenWrtDevice,omitempty"`
	AndroidDevices    []*AndroidDevice     `yaml:"AndroidDevice,omitempty"`
	BtReferenceDevice []*BtReferenceDevice `yaml:"BtReferenceDevice,omitempty"`
	PassportHost      []*PassportHost      `yaml:"PassportHost,omitempty"`
	ChameleonDevice   []*ChameleonDevice   `yaml:"ChameleonDevice,omitempty"`
	StarfishDevice    []*StarfishDevice    `yaml:"StarfishDevice,omitempty"`
}

type AndroidDevice struct {
	Serial string   `yaml:"serial"`
	Role   string   `yaml:"role"`
	Params paramMap `yaml:",inline"`
}

type OpenWrtDevice struct {
	Hostname string   `yaml:"hostname"`
	Params   paramMap `yaml:",inline"`
}

type BtReferenceDevice struct {
	Hostname string   `yaml:"hostname"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	Params   paramMap `yaml:",inline"`
}

type PassportHost struct {
	HostTopology *labapi.PasitHost
	Params       paramMap
}

type StarfishDevice struct {
	Carrier   string
	SIMInfos  []*labapi.SIMInfo
	ModemInfo *labapi.ModemInfo
	Params    paramMap
}

func (s StarfishDevice) MarshalYAML() (any, error) {
	var simInfos []any
	for _, si := range s.SIMInfos {
		friendlySI, err := yamlFriendlyPb(si)
		if err != nil {
			return "", fmt.Errorf("failed to make sim info protobuf yaml compatible: %w", err)
		}
		simInfos = append(simInfos, friendlySI)
	}

	modemInfo, err := yamlFriendlyPb(s.ModemInfo)
	if err != nil {
		return "", fmt.Errorf("failed to make modem info protobuf yaml compatible: %w", err)
	}

	return struct {
		Carrier   string   `yaml:"carrier"`
		ModemInfo any      `yaml:"modem_info,omitempty"`
		SIMInfos  any      `yaml:"sim_infos,omitempty"`
		Params    paramMap `yaml:",inline"`
	}{
		Carrier:   s.Carrier,
		ModemInfo: modemInfo,
		SIMInfos:  simInfos,
		Params:    s.Params,
	}, nil
}

func (p PassportHost) MarshalYAML() (any, error) {
	topology, err := yamlFriendlyPb(p.HostTopology)
	if err != nil {
		return "", fmt.Errorf("failed to make protobuf yaml compatible: %w", err)
	}

	return struct {
		HostTopology any      `yaml:"host_topology"`
		Params       paramMap `yaml:",inline"`
	}{
		HostTopology: topology,
		Params:       p.Params,
	}, nil
}

type ChameleonDevice struct {
	ChameleonIP         string   `yaml:"chameleon_ip"`
	ChameleonXMLRPCPort string   `yaml:"chameleon_xmlrpc_port"`
	Params              paramMap `yaml:",inline"`
}

// Config Parameters from ExecutionMetadata
// Keyed by `config-params`
type ConfigParams struct {
	// Keyed by `test-params`
	TestParams paramMap
	// Values prefixed by `primary` or `secondary` will only apply to that device.
	// Keyed by `android-params`
	AndroidParams []deviceParams
	// Values prefixed by `primary-<suffix>` or `secondary-<suffix>` will only apply to that device.
	// Keyed by `openwrt-params`
	OpenWrtParams []deviceParams
	// Keyed by `btreference-params`
	BtReferenceParams []deviceParams
	// Keyed by `passport-params`
	PassportParams paramMap
	// Keyed by `chameleon-params`
	ChameleonParams paramMap
	// Keyed by `starfish-params` and `starfish.carrier`
	StarfishParams paramMap
}

func NewMoblyConfig(logger *log.Logger, serials []string, metadata []*api.Arg,
	devices []*labapi.Dut, lsnexues map[string]string) *MoblyTestConfig {
	logger.Println("Generating Mobly Test Config")
	for _, arg := range metadata {
		logger.Println(arg.Flag, arg.Value)
	}
	configParams := ParseMetadata(logger, metadata)
	logger.Println("After parsing")
	for k, v := range configParams.TestParams {
		logger.Println(k, v)
	}

	for _, d := range configParams.AndroidParams {
		for k, v := range d.params {
			logger.Println(d.deviceId, k, v)
		}
	}
	for _, d := range configParams.OpenWrtParams {
		for k, v := range d.params {
			logger.Println(d, k, v)
		}
	}
	for _, d := range configParams.BtReferenceParams {
		for k, v := range d.params {
			logger.Println(d, k, v)
		}
	}
	for k, v := range lsnexues {
		configParams.TestParams[k] = v
	}

	Controllers := &Controllers{
		AndroidDevices:    GenerateAndroidDevices(serials, configParams.AndroidParams),
		OpenWrtDevices:    GenerateOpenWrtDevices(serials, devices, configParams.OpenWrtParams),
		BtReferenceDevice: GenerateBtReferenceDevices(serials, devices, configParams.BtReferenceParams),
		PassportHost:      GeneratePassportHost(logger, devices, configParams.PassportParams),
		ChameleonDevice:   GenerateChameleonDevices(serials, devices, configParams.ChameleonParams),
		StarfishDevice:    GenerateStarfishDevice(logger, devices, configParams.StarfishParams),
	}

	return &MoblyTestConfig{
		TestBeds: []*TestBed{
			{
				Name:        "LocalTestBed",
				Controllers: Controllers,
				TestParams: &TestParams{
					Params: configParams.TestParams,
				},
			},
		},
	}
}

func (c *MoblyTestConfig) Write(logger *log.Logger, dir string) (err error) {
	yamlData, err := yaml.Marshal(c)
	if err != nil {
		err = errors.Annotate(err, "failed to marshal yaml config").Err()
		return
	}

	logger.Println(string(yamlData))

	fileName := "test_config.yml"
	yamlPath := filepath.Join(dir, fileName)
	err = os.WriteFile(yamlPath, yamlData, 0644)
	if err != nil {
		err = errors.Annotate(err, "failed to write yaml config").Err()
		return
	}

	return
}

func GeneratePassportHost(logger *log.Logger, devices []*labapi.Dut, passportParams paramMap) []*PassportHost {
	var passportHosts []*PassportHost
	for _, dut := range devices {
		if dut.GetChromeos() == nil || dut.GetChromeos().GetPasitHost() == nil {
			continue
		}
		topology := dut.GetChromeos().GetPasitHost()
		passportHosts = append(passportHosts, &PassportHost{
			HostTopology: topology,
			Params:       passportParams,
		})
	}

	// If test params indicate a passport device, but none was found then add one with an empty topology
	// so users can still specify based on command line.
	if len(passportHosts) == 0 && len(passportParams) > 0 {
		passportHosts = append(passportHosts, &PassportHost{
			Params:       passportParams,
			HostTopology: &labapi.PasitHost{},
		})
	}

	return passportHosts
}

func GenerateStarfishDevice(logger *log.Logger, devices []*labapi.Dut, starfishParams paramMap) []*StarfishDevice {
	var starfishDevices []*StarfishDevice
	for _, dut := range devices {
		if dut.GetChromeos() == nil {
			continue
		}

		// No carrier -> Not a cellular DUT.
		if dut.GetChromeos().GetCellular() == nil || dut.GetChromeos().GetCellular().GetCarrier() == "" {
			continue
		}

		starfishDevices = append(starfishDevices, &StarfishDevice{
			Carrier:   dut.GetChromeos().GetCellular().GetCarrier(),
			SIMInfos:  dut.GetChromeos().GetSimInfos(),
			ModemInfo: dut.GetChromeos().GetModemInfo(),
			Params:    starfishParams,
		})
	}

	return starfishDevices
}

func GenerateBtReferenceDevices(serials []string, duts []*labapi.Dut, btReferenceParams []deviceParams) []*BtReferenceDevice {
	devices := []*BtReferenceDevice{}
	deviceKeyToSerial := map[string]string{}
	for i, serial := range serials {
		if i == 0 {
			deviceKeyToSerial["primary"] = strings.TrimSuffix(serial, ":5555")
		} else {
			deviceKeyToSerial["secondary"] = strings.TrimSuffix(serial, ":5555")
		}
	}

	paramsForEachDevice := paramMap{}
	for _, device := range btReferenceParams {
		if device.deviceId == "all" {
			paramsForEachDevice = device.params
			break
		}
	}

	// First copy all from infra.
	seen := make(map[string]*BtReferenceDevice)
	for _, dut := range duts {
		if dut.GetChromeos() == nil {
			continue
		}
		for _, btpeer := range dut.GetChromeos().GetBluetoothPeers() {
			hostname := btpeer.GetHostname()
			device := &BtReferenceDevice{
				Hostname: hostname,
				Username: "root",
				Password: "test0000",
				Params:   combinedParamMap(paramsForEachDevice),
			}
			seen[device.Hostname] = device
			devices = append(devices, device)
		}
	}

	// Now append devices specified by btReferenceParams & update infra params if provided.
	for _, device := range btReferenceParams {
		deviceKey := device.deviceId
		deviceParams := device.params
		if deviceKey == "all" {
			continue
		}

		deviceKey = strings.ReplaceAll(deviceKey, "primary", deviceKeyToSerial["primary"])
		deviceKey = strings.ReplaceAll(deviceKey, "secondary", deviceKeyToSerial["secondary"])

		if val, ok := seen[deviceKey]; ok {
			val.Params = combinedParamMap(val.Params, deviceParams)
		} else {
			// This device is unknown to infra ATM, just append it.
			devices = append(devices, &BtReferenceDevice{
				Hostname: deviceKey,
				Username: "root",
				Password: "test0000",
				Params:   combinedParamMap(paramsForEachDevice, deviceParams),
			})
		}
	}

	return devices
}

func GenerateChameleonDevices(serials []string, duts []*labapi.Dut, chameleonParams paramMap) []*ChameleonDevice {
	chameleon_devices := []*ChameleonDevice{}

	for _, dut := range duts {
		if chameleon_ip := dut.GetChromeos().GetChameleon().GetHostname(); chameleon_ip != "" {
			chameleon_devices = append(chameleon_devices, &ChameleonDevice{
				ChameleonIP:         chameleon_ip,
				ChameleonXMLRPCPort: "9992",
				Params:              chameleonParams,
			})
		}
	}

	return chameleon_devices
}

func GenerateOpenWrtDevices(serials []string, duts []*labapi.Dut, openWrtParams []deviceParams) []*OpenWrtDevice {
	devices := []*OpenWrtDevice{}
	deviceKeyToSerial := map[string]string{}
	for i, serial := range serials {
		if i == 0 {
			deviceKeyToSerial["primary"] = strings.TrimSuffix(serial, ":5555")
		} else {
			deviceKeyToSerial["secondary"] = strings.TrimSuffix(serial, ":5555")
		}
	}

	paramsForEachDevice := paramMap{}
	for _, device := range openWrtParams {
		if device.deviceId == "all" {
			paramsForEachDevice = device.params
			break
		}
	}

	// First copy all from infra.
	seen := make(map[string]*OpenWrtDevice)
	for _, dut := range duts {
		if dut.GetChromeos() == nil {
			continue
		}
		for _, ap := range dut.GetChromeos().GetWifi().GetWifiRouters() {
			hostname := ap.GetHostname()
			device := &OpenWrtDevice{
				Hostname: hostname,
				Params:   combinedParamMap(paramsForEachDevice),
			}
			seen[device.Hostname] = device
			devices = append(devices, device)
		}
	}

	for _, device := range openWrtParams {
		deviceKey := device.deviceId
		deviceParams := device.params
		if deviceKey == "all" {
			continue
		}

		deviceKey = strings.ReplaceAll(deviceKey, "primary", deviceKeyToSerial["primary"])
		deviceKey = strings.ReplaceAll(deviceKey, "secondary", deviceKeyToSerial["secondary"])

		if val, ok := seen[deviceKey]; ok {
			val.Params = combinedParamMap(val.Params, deviceParams)
		} else {
			devices = append(devices, &OpenWrtDevice{
				Hostname: deviceKey,
				Params:   combinedParamMap(paramsForEachDevice, deviceParams),
			})
		}
	}

	return devices
}

func GenerateAndroidDevices(serials []string, androidParams []deviceParams) []*AndroidDevice {
	devices := []*AndroidDevice{}

	androidParamsMap := deviceParamMap(androidParams)
	paramsForEachDevice := androidParamsMap["all"]
	for i, serial := range serials {
		var role string
		params := paramMap{}
		for k, v := range paramsForEachDevice {
			params[k] = v
		}

		var deviceParams paramMap
		if i == 0 {
			role = "source_device"
			deviceParams = androidParamsMap["primary"]
		} else {
			role = "target_device"
			deviceParams = androidParamsMap["secondary"]
		}
		for k, v := range deviceParams {
			params[k] = v
		}

		device := &AndroidDevice{
			Serial: serial,
			Role:   role,
			Params: params,
		}
		devices = append(devices, device)
	}

	return devices
}

func ParseMetadata(logger *log.Logger, metadata []*api.Arg) *ConfigParams {
	logger.Println("ParseMetadata")
	configParams := &ConfigParams{}

	for _, arg := range metadata {
		switch arg.Flag {
		case "test-params":
			logger.Println("test-params")
			configParams.TestParams = ParseArg(logger, arg, configParams.TestParams)
		case "android-params":
			logger.Println("android-params")
			configParams.AndroidParams = ParseDeviceArg(logger, arg)
		case "openwrt-params":
			logger.Println("openwrt-params")
			configParams.OpenWrtParams = ParseDeviceArg(logger, arg)
		case "btreference-params":
			logger.Println("btreference-params")
			configParams.BtReferenceParams = ParseDeviceArg(logger, arg)
		case "passport-params":
			logger.Println("passport-params")
			configParams.PassportParams = ParseArg(logger, arg, configParams.PassportParams)
		case "chameleon-params":
			logger.Println("chameleon-params")
			configParams.ChameleonParams = ParseArg(logger, arg, configParams.ChameleonParams)
		case "starfish-params":
			logger.Println("starfish-params")
			configParams.StarfishParams = ParseArg(logger, arg, configParams.StarfishParams)
		case "starfish.carrier":
			// allow starfish.carrier to be used directly as it's exposed directly to Testhaus (b/337286675).
			configParams.StarfishParams = combinedParamMap(configParams.StarfishParams, paramMap{arg.Flag: arg.Value})
		default:
			logger.Println("default")
			// Any additional test params dynamically passed in.
			if strings.HasPrefix(arg.Flag, "extra-test-params") {
				logger.Println("test-params")
				configParams.TestParams = ParseArg(logger, arg, configParams.TestParams)
			}
		}
	}

	return configParams
}

func ParseArg(logger *log.Logger, arg *api.Arg, argMap paramMap) paramMap {
	logger.Println("ParseArg")
	if argMap == nil {
		argMap = paramMap{}
	}

	for _, param := range strings.Split(arg.Value, ",") {
		logger.Println(param)
		parts := strings.Split(param, ":")
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		if _, ok := argMap[key]; ok {
			logger.Printf("duplicate arg: %q, original value: %q, new value: %q", key, argMap[key], value)
		}
		argMap[key] = value
		logger.Println(key, value)
	}

	logger.Println("ParseArg Return")
	return argMap
}

func ParseDeviceArg(logger *log.Logger, arg *api.Arg) []deviceParams {
	logger.Println("ParseDeviceArg")
	argMap := make(map[string]paramMap)

	deviceKey := "all"
	argMap["all"] = paramMap{}
	keys := []string{"all"}
	for _, param := range strings.Split(arg.Value, ",") {
		logger.Println(param)
		parts := strings.Split(param, ":")
		if len(parts) == 1 {
			deviceKey = parts[0]
			argMap[deviceKey] = paramMap{}
			keys = append(keys, deviceKey)
		}
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		argMap[deviceKey][key] = value
		logger.Println(deviceKey, key, value)
	}

	params := []deviceParams{}
	for _, key := range keys {
		params = append(params, deviceParams{
			deviceId: key,
			params:   argMap[key],
		})
	}

	logger.Println("ParseDeviceArg Return")
	return params
}

// yamlFriendlyPb converts a protobuf message into an object that, when converted into
// yaml, can easily be unmarshalled into the original protobuf. Without this,
// the golang field names will be mangled by the default yaml marshaller.
//
//	e.g. go from:
//	  * DeviceSerial -> deviceserial
//	to
//	  * DeviceSerial -> device_serial
func yamlFriendlyPb(m proto.Message) (any, error) {
	// If message is empty default to nil.
	if proto.Size(m) == 0 {
		return nil, nil
	}

	marshalOpts := protojson.MarshalOptions{
		// Optional, but helps keep marshalling in line with the rest of the testbed
		// .yaml file structs by switching to underscores vs camelCase.
		UseProtoNames: true,
	}

	// Convert to .json and then unmarshal into a new object whose fields will now
	// be marshalled
	asJson := marshalOpts.Format(proto.MessageV2(m))

	var res any
	if err := json.Unmarshal([]byte(asJson), &res); err != nil {
		return nil, err
	}
	return res, nil
}
