// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import "fmt"

type DeviceIdentifier struct {
	ID string
}

func DeviceIdentifierFromString(str string) *DeviceIdentifier {
	return &DeviceIdentifier{
		ID: str,
	}
}

func NewPrimaryDeviceIdentifier() *DeviceIdentifier {
	return &DeviceIdentifier{
		ID: Primary,
	}
}

func NewCompanionDeviceIdentifier(board string) *DeviceIdentifier {
	return &DeviceIdentifier{
		ID: fmt.Sprintf("%s_%s", Companion, board),
	}
}

func (id *DeviceIdentifier) AddPostfix(postfix string) *DeviceIdentifier {
	return &DeviceIdentifier{
		ID: fmt.Sprintf("%s_%s", id.ID, postfix),
	}
}

func (id *DeviceIdentifier) GetDevice(innerValueCallChain ...string) string {
	resp := fmt.Sprintf("device_%s", id.ID)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

func (id *DeviceIdentifier) GetDeviceMetadata(innerValueCallChain ...string) string {
	resp := fmt.Sprintf("deviceMetadata_%s", id.ID)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

func (id *DeviceIdentifier) GetCrosDutServer() string {
	return fmt.Sprintf("crosDutServer_%s", id.ID)
}

func (id *DeviceIdentifier) GetUpdateFirmware() string {
	return fmt.Sprintf("updateFirmware_%s", id.ID)
}

type TaskIdentifier struct {
	ID string
}

func NewTaskIdentifier(taskBaseIdentifier string) *TaskIdentifier {
	return &TaskIdentifier{
		ID: taskBaseIdentifier,
	}
}

func (id *TaskIdentifier) AddDeviceID(deviceID *DeviceIdentifier) *TaskIdentifier {
	return &TaskIdentifier{
		ID: fmt.Sprintf("%s_%s", id.ID, deviceID.ID),
	}
}

func (id *TaskIdentifier) GetRPCResponse(rpc string, innerValueCallChain ...string) string {
	resp := fmt.Sprintf("%s_%s", id.ID, rpc)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

func (id *TaskIdentifier) GetRPCRequest(rpc string, innerValueCallChain ...string) string {
	resp := fmt.Sprintf("%s_%sRequest", id.ID, rpc)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}
