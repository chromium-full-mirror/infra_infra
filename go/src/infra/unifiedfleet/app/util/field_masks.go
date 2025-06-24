// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

// Valid Field Paths for Field Mask
//
// New field paths should generally follow: go/proto-field-mask
// In theory, the constant value should be the proto field name in snake case,
// including any nested proto fields. In practice, most of these constant values
// are the shortest non-ambiguous representation for a given proto field.
//
// Some efforts have been made to standardize these field paths.
// Try to avoid non-standard field paths for new use cases.

const (
	// Common
	CapacityPath         string = "capacity"
	DeploymentTicketPath string = "deployment_ticket"
	DescriptionPath      string = "description"
	LogicalZonePath      string = "logicalZone"
	MacAddressPath       string = "macAddress"
	MachinePath          string = "machine"
	MachinesPath         string = "machines"
	ManufacturerPath     string = "manufacturer"
	MlseprototypePath    string = "mlseprototype"
	NamePath             string = "name"
	OsImagePath          string = "osImage"
	OsVersionPath        string = "osVersion"
	ChromePlatformPath   string = "chrome_platform"
	PlatformPath         string = "platform"
	PortNamePath         string = "portName"
	RackPath             string = "rack"
	ResourceStatePath    string = "resource_state"
	SerialNumberPath     string = "serial_number"
	SwitchPath           string = "switch"
	TagsPath             string = "tags"
	TagsRemovePath       string = "tags.remove"
	TypePath             string = "type"
	UpdateTimePath       string = "update_time"
	ZonePath             string = "zone"
	ZonesPath            string = "zones"
	ZonesRemovePath      string = "zones.remove"
	// Non-standard
	DeploymentTicketCamelPath string = "deploymentTicket"
	ResourceStateCamelPath    string = "resourceState"
	SerialNumberCamelPath     string = "serialNumber"

	// Location
	LocationPath            string = "location"
	LocationAislePath       string = "location.aisle"
	LocationBarcodeNamePath string = "location.barcode_name"
	LocationPositionPath    string = "location.position"
	LocationRackNumberPath  string = "location.rack_number"
	LocationRackPath        string = "location.rack"
	LocationRowPath         string = "location.row"
	LocationShelfPath       string = "location.shelf"
	LocationZonePath        string = "location.zone"

	// Asset
	InfoAssetTagPath           string = "info.asset_tag"
	InfoBuildTargetPath        string = "info.build_target"
	InfoCostCenterPath         string = "info.cost_center"
	InfoEthernetMacAddressPath string = "info.ethernet_mac_address"
	InfoGoogleCodeNamePath     string = "info.google_code_name"
	InfoPhasePath              string = "info.phase"
	InfoReferenceBoardPath     string = "info.reference_board"
	ModelPath                  string = "model"

	// DUT
	DutAteHostPath              string = "dut.ateHost"
	DutAudioAtrusPath           string = "dut.audio.atrus"
	DutAudioBoxPath             string = "dut.audio.box"
	DutAudioCablePath           string = "dut.audio.cable"
	DutCableTypePath            string = "dut.cable.type"
	DutCameraboxFacingPath      string = "dut.camerabox.facing"
	DutCameraboxLightPath       string = "dut.camerabox.light"
	DutCameraboxPath            string = "dut.camerabox"
	DutCameraTypePath           string = "dut.camera.type"
	DutCarrierPath              string = "dut.carrier"
	DutChameleonAudioboardPath  string = "dut.chameleon.audioboard"
	DutChameleonTypePath        string = "dut.chameleon.type"
	DutChaosPath                string = "dut.chaos"
	DutDolosFirmwareVersionPath string = "dut.dolos.firmware.version"
	DutDolosHostnamePath        string = "dut.dolos.hostname"
	DutDolosRpmHostPath         string = "dut.dolos.rpm.host"
	DutDolosRpmOutletPath       string = "dut.dolos.rpm.outlet"
	DutDolosSerialCablePath     string = "dut.dolos.serial.cable"
	DutDolosSerialUsbPath       string = "dut.dolos.serial.usb"
	DutHivePath                 string = "dut.hive"
	DutHostnamePath             string = "dut.hostname"
	DutLicensesPath             string = "dut.licenses"
	DutModeminfoPath            string = "dut.modeminfo"
	DutOsRestrictionPath        string = "dut.os.restriction"
	DutPoolsPath                string = "dut.pools"
	DutRpmHostPath              string = "dut.rpm.host"
	DutRpmOutletPath            string = "dut.rpm.outlet"
	DutRpmTypePath              string = "dut.rpm.type"
	DutServoDockerContainerPath string = "dut.servo.dockerContainer"
	DutServoFwchannelPath       string = "dut.servo.fwchannel"
	DutServoHostnamePath        string = "dut.servo.hostname"
	DutServoPortPath            string = "dut.servo.port"
	DutServoSerialPath          string = "dut.servo.serial"
	DutServoSetupPath           string = "dut.servo.setup"
	DutServoTopologyPath        string = "dut.servo.topology"
	DutServoTypePath            string = "dut.servo.type"
	DutSiminfoPath              string = "dut.siminfo"
	DutStarfishSlotMappingPath  string = "dut.starfishSlotMapping"
	DutSubrailConfigPath        string = "dut.subrailConfig"
	DutTouchMimoPath            string = "dut.touch.mimo"
	DutUsbSmarthubPath          string = "dut.usb.smarthub"
	DutWifiAntennaconnPath      string = "dut.wifi.antennaconn"
	DutWifiRouterPath           string = "dut.wifi.router"
	DutWifiWificellPath         string = "dut.wifi.wificell"

	// Labstation
	LabstationHivePath      string = "labstation.hive"
	LabstationHostnamePath  string = "labstation.hostname"
	LabstationPoolsPath     string = "labstation.pools"
	LabstationRpmHostPath   string = "labstation.rpm.host"
	LabstationRpmOutletPath string = "labstation.rpm.outlet"
	LabstationRpmTypePath   string = "labstation.rpm.type"

	// Scheduling Unit
	CarrierPath           string = "carrier"
	ExposeTypePath        string = "expose-type"
	MachinelsesPath       string = "machinelses"
	MachinelsesRemovePath string = "machinelses.remove"
	PoolsPath             string = "pools"
	PoolsRemovePath       string = "pools.remove"
	PrimaryDutPath        string = "primary-dut"
	WificellPath          string = "wificell"

	// Chrome Browser Host
	KvmInterfaceKvmPath      string = "kvm_interface.kvm"
	KvmInterfacePortNamePath string = "kvm_interface.port_name"
	VirtualDatacenterPath    string = "virtualDatacenter"
	VmCapacityPath           string = "vmCapacity"
	// Non-standard
	KvmPath     string = "kvm"
	KvmPortPath string = "kvmport"

	// VM
	CpuCoresPath     string = "cpuCores"
	MachineLseIdPath string = "machineLseId"
	MemoryPath       string = "memory"
	StoragePath      string = "storage"
	VmidPath         string = "vmid"

	// AttachedDevice
	AttachedDeviceBuildTargetPath  string = "attached_device.build_target"
	AttachedDeviceDeviceTypePath   string = "attached_device.device_type"
	AttachedDeviceManufacturerPath string = "attached_device.manufacturer"
	AttachedDeviceModelPath        string = "attached_device.model"
	AssocHostnamePath              string = "assocHostname"
	AssocHostPortPath              string = "assocHostPort"
	// Non-standard
	AdmBuildTargetPath  string = "admBuildTarget"
	AdmDeviceTypePath   string = "admDeviceType"
	AdmManufacturerPath string = "admManufacturer"
	AdmModelPath        string = "admModel"

	// Devboard
	AndreiboardUltradebugSerialPath string = "devboard.andreiboard.ultradebug_serial"
	PoolsDevboardPath               string = "pools-devboard"
	PoolsDevboardRemovePath         string = "pools-devboard-remove"

	// MachineLSE
	SchedulablePath string = "schedulable"

	// MachineLSEDeployment
	ConfigsToPushPath        string = "configs_to_push"
	DeploymentEnvPath        string = "deployment_env"
	DeploymentIdentifierPath string = "deployment_identifier"
	HostnamePath             string = "hostname"

	// CachingService
	PortPath                 string = "port"
	PrimaryNodePath          string = "primary_node"
	SecondaryNodePath        string = "secondary_node"
	ServingSubnetPath        string = "serving_subnet"
	ServingSubnetsPath       string = "serving_subnets"
	ServingSubnetsRemovePath string = "serving_subnets.remove"
	StatePath                string = "state"

	// Rack
	BbnumPath string = "bbnum"

	// Drac
	DisplayNamePath string = "displayName"

	// Vlan
	VlanAddressPath string = "vlan_address"
	FreeEndIpPath   string = "free_end_ip"
	FreeStartIpPath string = "free_start_ip"
	ReservedIpsPath string = "reserved_ips"
	// Non-standard
	CidrBlockPath string = "cidr_block"

	// DefaultWifi
	WifiSecretProjectIdPath  string = "wifi_secret.project_id"
	WifiSecretSecretNamePath string = "wifi_secret.secret_name"

	// DeviceLabels
	LabelsPath       string = "labels"
	ResourceTypePath string = "resource_type"
)
