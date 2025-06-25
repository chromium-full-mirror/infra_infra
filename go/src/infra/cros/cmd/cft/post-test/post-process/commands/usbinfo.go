// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package commands

import (
	"context"
	"encoding/json"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"

	"go.chromium.org/infra/cros/cmd/cft/common/files"
	"go.chromium.org/infra/cros/cmd/cft/post-test/post-process/common"
)

const USBInfoFileName = "usb_info.json"

// USBJson represents the structure of the USB info JSON file.
type USBJson struct {
	DUTPdPortServo json.Number `json:"dut_pd_port"`
	DUTPdPortCount uint32      `json:"dut_pd_port_count"`
}

// GetUsbInfo retrieves USB info from USB info JSON file.
func GetUsbInfo(logger *log.Logger, testResult *artifact.TestResult) (*api.GetUsbInfoResponse, error) {
	if testResult == nil {
		return &api.GetUsbInfoResponse{}, nil
	}

	usbFiles := common.TestLevelFiles(logger, testResult, USBInfoFileName)
	ctx := context.Background()
	usbInfo, err := usbInfos(ctx, logger, usbFiles)
	if err != nil {
		return nil, err
	}
	return &api.GetUsbInfoResponse{UsbInfo: usbInfo}, nil
}

// usbInfos ingests the USB info from the first successfully read JSON file. It is expected that
// all the files are identical. usbFiles is a map of test name to the path of the USB info JSON file.
func usbInfos(ctx context.Context, logger *log.Logger, usbFiles map[string]string) (*anypb.Any, error) {
	usbInfo := &artifact.DutInfo_UsbInfo{}

	if len(usbFiles) == 0 {
		return anypb.New(usbInfo)
	}

	// Process the first USB file
	var test, usbFile string
	for t, f := range usbFiles {
		test = t
		usbFile = f
		break
	}

	// Converts the USB info JSON file into USB info proto.
	if err := readUSBJSONFile(ctx, usbFile, usbInfo); err != nil {
		logger.Printf("Failed to read USB info from file: %q for test: %q with err: %v", usbFile, test, err)
		return nil, err
	}
	logger.Printf("Successfully read USB info from file: %q for test: %q", usbFile, test)

	return anypb.New(usbInfo)
}

// readUSBJSONFile reads a JSON file and converts into a DutInfo_UsbInfo proto.
func readUSBJSONFile(ctx context.Context, filePath string, usbInfo *artifact.DutInfo_UsbInfo) error {
	bytes, err := files.ReadJSONFile(ctx, filePath)
	if err != nil {
		return err
	}

	var usbInfoJSON USBJson
	err = json.Unmarshal(bytes, &usbInfoJSON)
	if err != nil {
		return err
	}

	if usbInfoJSON.DUTPdPortServo != "" {
		usbInfo.PowerDeliveryPortServo = string(usbInfoJSON.DUTPdPortServo)
	}
	if usbInfoJSON.DUTPdPortCount != 0 {
		usbInfo.PowerDeliveryPortCount = usbInfoJSON.DUTPdPortCount
	}

	return nil
}
