// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package enumeration

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// GoogleVID is the USB Vendor ID for Google
const GoogleVID = "18d1"

var GooglePIDs = map[string]string{
	// Debugger USB connection
	// For Servo V4.1
	"520d": "servo4.1",
	// For Servo V4
	"501b": "servo4",
	// For Servo SuzyQ
	"501f": "suzyq",
	// For H1 chip
	"5014": "cr50",
	// For D2 chip
	"504a": "ti50",
	// For NuvoTitan chip
	"5066": "ti50",
}

// USBDevice represents a plugged in USB Device
type USBDevice struct {
	Serial     string
	DevicePath string
	HubPath    string
	DeviceType string
}

// NewUSBDevice is a constructor for USBDevice
func NewUSBDevice(path string) (*USBDevice, error) {
	u := USBDevice{}
	var dPID, dSerial string

	splitPath := strings.Split(path, ".")
	hubPath := strings.Join(splitPath[:len(splitPath)-1], ".")

	dPID, err := getPathContent(path + "idProduct")
	if err != nil {
		return nil, err
	}

	dType, ok := GooglePIDs[dPID]
	if !ok {
		err := fmt.Errorf("unknown PID: %s", dPID)
		return nil, err
	}

	dSerial, err = getPathContent(path + "serial")
	if err != nil {
		return nil, err
	}

	u.DevicePath = path
	u.HubPath = hubPath
	u.DeviceType = dType
	u.Serial = dSerial

	return &u, nil
}

// getPathContents returns the text output from USB Device properties
func getPathContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	fileContent := strings.TrimSpace(string(content))

	return fileContent, nil
}

// getGoogleDevicePaths returns an array of local USB devices that have Googles VID
func getGoogleDevicePaths() ([]string, error) {
	devicePaths := []string{}

	vidPaths, err := filepath.Glob("/sys/bus/usb/devices/*/idVendor")
	if err != nil {
		return nil, err
	}
	if len(vidPaths) == 0 {
		err := errors.New("no google usb devices detected")
		return nil, err
	}

	for _, item := range vidPaths {
		contents, err := os.ReadFile(item)
		if err != nil {
			return nil, err
		}

		if strings.Contains(string(contents), GoogleVID) {
			devicePaths = append(devicePaths, item)
		}
	}

	return devicePaths, nil
}

// getDevicesFromPaths uses the output of getGoogleDevicePaths to return an array of USBDevice objects
func getDevicesFromPaths(paths []string) ([]USBDevice, error) {
	devices := []USBDevice{}

	for _, path := range paths {
		dPath := strings.TrimSuffix(path, "idVendor")
		device, err := NewUSBDevice(dPath)

		if err != nil {
			continue
		}
		if device != nil {
			devices = append(devices, *device)
		}
	}

	return devices, nil
}

// FindServoFromDUT finds the Servo associated a given serial number from a DUTs cr50/ti50 by comparing
// the USB hub path. Both a Servo and a device plugged into a Servo will be enumerated under the Servos
// built-in hub in the USB device tree.
func FindServoFromDUT(dutSerial string, devices []USBDevice) (USBDevice, error) {
	var dut *USBDevice

	for _, device := range devices {
		// Dut serial found in USB hub path on Satlab is always capitalized, but when we on the DUT it is mix
		// of both lower and capital for different DUTs. Therefore compare these values case-insensitively.
		if strings.EqualFold(device.Serial, dutSerial) {
			dut = &device
			break
		}
	}

	if dut == nil {
		err := fmt.Errorf("no DUT found with serial: %s", dutSerial)
		return USBDevice{}, err
	}

	for _, device := range devices {
		if (device.DeviceType == "servo4" || device.DeviceType == "servo4.1") && device.HubPath == dut.HubPath {
			return device, nil
		}
	}

	err := fmt.Errorf("no servo was found that is associated with DUT serial: %s", dutSerial)
	return USBDevice{}, err
}

// GetAllServoUSBDevices returns all the USBDevices instance of plugged devices
func GetAllServoUSBDevices() ([]USBDevice, error) {
	devPaths, err := getGoogleDevicePaths()
	if err != nil {
		log.Printf("error process Google Devices Paths %v", err)
		return nil, err
	}
	return getDevicesFromPaths(devPaths)
}
