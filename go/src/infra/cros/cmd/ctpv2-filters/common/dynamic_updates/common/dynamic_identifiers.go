// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import "fmt"

// Constants relating to dynamic dependency storage.
const (
	// Base task identifiers and image metadata keys.
	CrosProvision    = "cros-provision"
	AndroidProvision = "android-provision"
	CrosDut          = "cros-dut"
	RdbPublish       = "rdb-publish"
	ServoNexus       = "servo-nexus"
	CrosServod       = "cros-servod"

	// Device base identifiers.
	Primary   = "primary"
	Companion = "companion"

	// Commonly used Dynamic Dependecy keys.
	ServiceAddress = "serviceAddress"
	CacheServer    = "cache-server"
)

var (
	PrimaryDevice            = NewPrimaryDeviceIdentifier().GetDevice()
	CompanionDevices         = NewCompanionDeviceIdentifier("all").GetDevice()
	CompanionDevicesMetadata = NewCompanionDeviceIdentifier("all").GetDeviceMetadata()
)

// -------------------DeviceIdentifier------------------- //

// DeviceIdentifier is the baseline structure for
// handling dynamic identities for devices.
type DeviceIdentifier struct {
	Id string
}

// GetDevice returns the format for retrieving the device's dut
// topology value, and allows for calling interior fields.
func (id *DeviceIdentifier) GetDevice(innerValueCallChain ...string) string {
	resp := fmt.Sprintf("device_%s", id.Id)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

// GetDeviceMetadata returns the format for retrieving the device's
// metadata information.
func (id *DeviceIdentifier) GetDeviceMetadata(innerValueCallChain ...string) string {
	resp := fmt.Sprintf("deviceMetadata_%s", id.Id)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

// GetCrosDutServer returns the format for retrieving the device's
// specificed cros-dut server ip.
func (id *DeviceIdentifier) GetCrosDutServer() string {
	return fmt.Sprintf("crosDutServer_%s", id.Id)
}

// AddPostfix appends a value to the DeviceIdentifier.
func (id *DeviceIdentifier) AddPostfix(postfix string) *DeviceIdentifier {
	id.Id = fmt.Sprintf("%s_%s", id.Id, postfix)
	return id
}

// DeviceIdentifierFromString initializes the DeviceIdentifier from a string.
func DeviceIdentifierFromString(str string) *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: str,
	}
}

// NewPrimaryDeviceIdentifier initializes a DeviceIdentifier
// for the primary device.
func NewPrimaryDeviceIdentifier() *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: Primary,
	}
}

// NewCompanionDeviceIdentifier initializes a DeviceIdentifier for
// a companion device, postfixed with the board.
func NewCompanionDeviceIdentifier(board string) *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: fmt.Sprintf("%s_%s", Companion, board),
	}
}

// -------------------TaskIdentifier------------------- //

// TaskIdentifier is the baseline structure for handling
// dynamic identities for tasks.
type TaskIdentifier struct {
	Id string
}

// GetRpcResponse returns the format for accessing responses from a task.
func (id *TaskIdentifier) GetRpcResponse(rpc string, innerValueCallChain ...string) string {
	resp := fmt.Sprintf("%s_%s", id.Id, rpc)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

// GetRpcRequest returns the format for accessing requests from a task.
func (id *TaskIdentifier) GetRpcRequest(rpc string, innerValueCallChain ...string) string {
	resp := fmt.Sprintf("%s_%sRequest", id.Id, rpc)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

// AddDeviceId postfixes the task id with a device id. Important to use
// when working with provisions or other device targeting tasks.
func (id *TaskIdentifier) AddDeviceId(deviceId *DeviceIdentifier) *TaskIdentifier {
	id.Id = fmt.Sprintf("%s_%s", id.Id, deviceId.Id)
	return id
}

// NewTaskIdentifier initializes the TaskIdentifier from a string.
func NewTaskIdentifier(taskBaseIdentifier string) *TaskIdentifier {
	return &TaskIdentifier{
		Id: taskBaseIdentifier,
	}
}
