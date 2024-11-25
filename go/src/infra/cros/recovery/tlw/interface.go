// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tlw provides an abstract representation of the TLW API which is used by recovery
package tlw

import (
	"context"

	"infra/cros/dutstate"
)

// Access represent TLW level to access to the devices and inventory.
// Each device in the lab is representing as resource with name.
// For now the resource name matche to host-name but later can became different.
// Examples:
//
//	Hostname: lab1-row1-rack1-device1, lab1-row1-rack1-ap1
//	Resource Name: TestDevice256, CustomApV3.0
type Access interface {
	// Ping performs ping by resource name.
	Ping(ctx context.Context, resourceName string, count int) error
	// Run executes command on device by SSH related to resource name.
	Run(ctx context.Context, req *RunRequest) *RunResult
	// InitServod initiates servod daemon on servo-host.
	InitServod(ctx context.Context, req *InitServodRequest) error
	// StopServod stops servod daemon on servo-host.
	StopServod(ctx context.Context, resourceName string) error
	// CallServod executes a command on servod related to resource name.
	// Commands will be run against servod on servo-host.
	CallServod(ctx context.Context, req *CallServodRequest) *CallServodResponse
	// Commands will be run against servod on servo-host.
	CallBluetoothPeer(ctx context.Context, req *CallBluetoothPeerRequest) *CallBluetoothPeerResponse
	// CallTouchHostd calls API on touchostd with XMLRPC.
	CallTouchHostd(ctx context.Context, req *CallTouchHostdRequest) *CallTouchHostdResponse
	// CopyFileTo copies file to remote device from local.
	CopyFileTo(ctx context.Context, req *CopyRequest) error
	// CopyFileFrom copies file from remote device to local.
	CopyFileFrom(ctx context.Context, req *CopyRequest) error
	// CopyDirectoryTo copies directory to remote device from local, recursively.
	CopyDirectoryTo(ctx context.Context, req *CopyRequest) error
	// CopyDirectoryFrom copies directory from remote device to local, recursively.
	CopyDirectoryFrom(ctx context.Context, req *CopyRequest) error
	// RunRPMAction performs power action on RPM outlet per request.
	RunRPMAction(ctx context.Context, req *RunRPMActionRequest) error
	// ListResourcesForUnit provides list of resources names related to target unit.
	// All test and task scheduling against the target unit which can link to 1 or more resources.
	ListResourcesForUnit(ctx context.Context, unitName string) ([]string, error)
	// GetDut provides DUT info per requested resource name from inventory.
	GetDut(ctx context.Context, resourceName string) (*Dut, error)
	// UpdateDut updates DUT info into inventory.
	UpdateDut(ctx context.Context, dut *Dut) error
	// GetCacheUrl provides URL to download requested path to file.
	// URL will use to download image to USB-drive and provisioning.
	GetCacheUrl(ctx context.Context, dutName, filePath string) (string, error)
	// Provision triggers provisioning of the device.
	Provision(ctx context.Context, req *ProvisionRequest) error
	// Close closes all used resources.
	Close(ctx context.Context) error
}

const (
	// Extra attributes for DUT to provide custom info.
	ExtraAttributePools          = "POOLS"
	ExtraAttributeServoSetup     = "SERVO_SETUP"
	ExtraAttributeServoSetupDual = "SERVO_SETUP_DUAL"
)

// Dut holds info about setup used as testbed.
type Dut struct {
	// Unique identifier in inventory.
	// Should never be changed in processes.
	Id string
	// Name is the resource name for the DUT.
	Name string
	// SetupType describes the setup of the DUT, which affects how it is verified/repaired.
	SetupType DUTSetupType
	// State of the DUT.
	State dutstate.State
	// ProvisionedInfo tells provisioned info for the DUT.
	// TODO: Remove this once confirmed new VersionInfo works from end-to-end.
	ProvisionedInfo *ProvisionedInfo
	// VersionInfo tells current system version(e.g. OS, firmware) info for the DUT.
	VersionInfo *VersionInfo
	// Explain why the DUT state was set.
	// The value may not be available, and is used to indicate reason of a bad
	// state.
	DutStateReason DutStateReason
	// Extra attrubes of the DUT.
	// Used to provide any other mapping data which can be used for custom actions.
	// All values has to be converted to string.
	// Example: pools, force_flashing, restrictions and special abilities.
	ExtraAttributes map[string][]string
	// List of repair-requestes specified by external services.
	RepairRequests []RepairRequest
	// Chromeos hold specific data for ChromeOS device's data.
	Chromeos *ChromeOS
	// Android hold specific data for Android device's data.
	Android *Android
	// DevBoard hold specific data for DevBoard device's data.
	DevBoard *DevBoard
}

// GetAndroid returns Android device.
// The method created to mimic proto syntactic.
func (d *Dut) GetAndroid() *Android {
	if d == nil {
		return nil
	}
	return d.Android
}

// GetChromeos returns ChromeOS device.
// The method created to mimic proto syntactic.
func (d *Dut) GetChromeos() *ChromeOS {
	if d == nil {
		return nil
	}
	return d.Chromeos
}

// GetDevBoard returns DevBoard device.
// The method created to mimic proto syntactic.
func (d *Dut) GetDevBoard() *DevBoard {
	if d == nil {
		return nil
	}
	return d.DevBoard
}

func (d *Dut) GetVersionInfo() *VersionInfo {
	if d == nil {
		return nil
	}
	return d.VersionInfo
}

// GetBoard provides board name of DUT.
func (d *Dut) GetBoard() string {
	if ch := d.GetChromeos(); ch != nil {
		return ch.GetBoard()
	} else if db := d.GetDevBoard(); db != nil {
		return db.GetBoard()
	} else if a := d.GetAndroid(); a != nil {
		return a.GetBoard()
	}
	return ""
}

// GetModel provides model name of DUT.
func (d *Dut) GetModel() string {
	if ch := d.GetChromeos(); ch != nil {
		return ch.GetModel()
	} else if db := d.GetDevBoard(); db != nil {
		return db.GetModel()
	} else if a := d.GetAndroid(); a != nil {
		return a.GetModel()
	}
	return ""
}

// GetPools provides list of the pools for the DUT.
func (d *Dut) GetPools() []string {
	if d != nil && d.ExtraAttributes != nil {
		return d.ExtraAttributes[ExtraAttributePools]
	}
	return nil
}
