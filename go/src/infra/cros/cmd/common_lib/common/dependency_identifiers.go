// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import "fmt"

type DeviceIdentifier struct {
	Id string
}

func DeviceIdentifierFromString(str string) *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: str,
	}
}

func NewPrimaryDeviceIdentifier() *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: Primary,
	}
}

func NewCompanionDeviceIdentifier(board string) *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: fmt.Sprintf("%s_%s", Companion, board),
	}
}

func (id *DeviceIdentifier) AddPostfix(postfix string) *DeviceIdentifier {
	return &DeviceIdentifier{
		Id: fmt.Sprintf("%s_%s", id.Id, postfix),
	}
}

func (id *DeviceIdentifier) GetDevice(innerValueCallChain ...string) string {
	resp := fmt.Sprintf("device_%s", id.Id)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

func (id *DeviceIdentifier) GetDeviceMetadata(innerValueCallChain ...string) string {
	resp := fmt.Sprintf("deviceMetadata_%s", id.Id)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

func (id *DeviceIdentifier) GetCrosDutServer() string {
	return fmt.Sprintf("crosDutServer_%s", id.Id)
}

func (id *DeviceIdentifier) GetUpdateFirmware() string {
	return fmt.Sprintf("updateFirmware_%s", id.Id)
}

type TaskIdentifier struct {
	Id string
}

func NewTaskIdentifier(taskBaseIdentifier string) *TaskIdentifier {
	return &TaskIdentifier{
		Id: taskBaseIdentifier,
	}
}

func (id *TaskIdentifier) AddDeviceId(deviceId *DeviceIdentifier) *TaskIdentifier {
	return &TaskIdentifier{
		Id: fmt.Sprintf("%s_%s", id.Id, deviceId.Id),
	}
}

func (id *TaskIdentifier) GetRpcResponse(rpc string, innerValueCallChain ...string) string {
	resp := fmt.Sprintf("%s_%s", id.Id, rpc)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}

func (id *TaskIdentifier) GetRpcRequest(rpc string, innerValueCallChain ...string) string {
	resp := fmt.Sprintf("%s_%sRequest", id.Id, rpc)

	for _, innerValueCall := range innerValueCallChain {
		resp = fmt.Sprintf("%s.%s", resp, innerValueCall)
	}

	return resp
}
