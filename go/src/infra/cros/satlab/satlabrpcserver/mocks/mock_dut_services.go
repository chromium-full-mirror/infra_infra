// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"go.chromium.org/infra/cros/satlab/common/enumeration"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/models"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/services/dut_services"
)

// MockDUTServices This object is only for testing
//
// Object should provide the same functions that `IDUTServices` interfaces provide.
type MockDUTServices struct {
	mock.Mock
}

// RunCommandOnIP send the command to the DUT device and then get the result back
func (m *MockDUTServices) RunCommandOnIP(ctx context.Context, IP, cmd string) (*models.SSHResult, error) {
	args := m.Called(ctx, IP, cmd)
	return args.Get(0).(*models.SSHResult), args.Error(1)
}

// RunCommandOnIPs send the command to DUT devices and then get the result back
func (m *MockDUTServices) RunCommandOnIPs(ctx context.Context, IPs []string, cmd string) []*models.SSHResult {
	args := m.Called(ctx, IPs, cmd)
	return args.Get(0).([]*models.SSHResult)
}

func (m *MockDUTServices) GetConnectedIPs(ctx context.Context) ([]dut_services.Device, error) {
	args := m.Called(ctx)
	return args.Get(0).([]dut_services.Device), args.Error(1)
}

func (m *MockDUTServices) GetBoard(ctx context.Context, IP string) (string, error) {
	args := m.Called(ctx, IP)
	return args.String(0), args.Error(1)
}

func (m *MockDUTServices) GetModel(ctx context.Context, IP string) (string, error) {
	args := m.Called(ctx, IP)
	return args.String(0), args.Error(1)
}

// GetGSCSerialAndServoUSBCount gets the Cr50 or Ti50 port serial number on the DUT`
func (m *MockDUTServices) GetGSCSerialAndServoUSBCount(ctx context.Context, address string) (*dut_services.GSCInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(*dut_services.GSCInfo), args.Error(1)
}

// GetServoSerial gets the Servo serial number for give DUT IP address.
func (m *MockDUTServices) GetServoSerial(ctx context.Context, IP string, usbDevices []enumeration.USBDevice) (bool, string, error) {
	args := m.Called(ctx, IP)
	return args.Get(0).(bool), args.String(1), args.Error(2)
}

// GetUSBDevicePaths returns all the USBDevices instance of plugged devices
func (m *MockDUTServices) GetUSBDevicePaths(ctx context.Context) ([]enumeration.USBDevice, error) {
	args := m.Called(ctx)
	return args.Get(0).([]enumeration.USBDevice), args.Error(1)
}

// GetCCDStatus gets CCD status from the DUT
func (m *MockDUTServices) GetCCDStatus(ctx context.Context, address string) (string, error) {
	args := m.Called(ctx, address)
	return args.String(0), args.Error(1)
}
