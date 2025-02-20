// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	// Values presented as the string of the hex without 0x to match
	// representation in sysfs (idVendor/idProduct).

	// Servo's DUT side HUB vendor id
	SERVO_DUT_HUB_VID = "04b4"
	// Servo's DUT side HUB product id
	SERVO_DUT_HUB_PID = "6502"
	// Servo's DUT side NIC vendor id
	SERVO_DUT_NIC_VID = "0bda"
	// Servo's DUT side NIC product id
	SERVO_DUT_NIC_PID = "8153"
)

const (
	// Time to wait a rebooting ChromeOS, in seconds.
	NormalBootingTime = 150 * time.Second
)

// hasOnlySingleLine determines if the given string is only one single line.
func hasOnlySingleLine(ctx context.Context, s string) bool {
	if s == "" {
		log.Debugf(ctx, "The string is empty")
		return false
	}
	lines := strings.Split(s, "\n")
	if len(lines) != 1 {
		log.Debugf(ctx, "Found %d lines in the string.", len(lines))
		return false
	}
	return true
}

const (
	// findFilePathByContentCmdGlob find the file path by the content.
	// ex: grep -l xxx $(find /xxx/xxxx -maxdepth 1 -name xxx)
	findFilePathByContentCmdGlob = "grep -l %s $(find %s -maxdepth 1 -name %s)"
)

// FindSingleUsbDeviceFSDir find the common parent directory where the unique device with VID and PID is enumerated by file system.
//
//  1. Get path to the unique idVendor file with VID
//  2. Get path to the unique idProduct file with PID
//  3. Get directions of both files and compare them
//
// @param basePath: Path to the directory where to look for the device.
// @param vid: Vendor ID of the looking device.
// @param pid: Product ID of the looking device.
//
// @returns: path to the folder of the device.
func FindSingleUsbDeviceFSDir(ctx context.Context, r execs.Runner, basePath string, vid string, pid string) (string, error) {
	if basePath == "" {
		return "", errors.Reason("find single usb device file system directory: basePath is not provided").Err()
	}
	basePath += "*/"
	// find vid path:
	vidPath, err := r(ctx, time.Minute, fmt.Sprintf(findFilePathByContentCmdGlob, vid, basePath, "idVendor"))
	if err != nil {
		return "", errors.Annotate(err, "find single usb device file system directory").Err()
	} else if !hasOnlySingleLine(ctx, vidPath) {
		return "", errors.Reason("find single usb device file system directory: found more then one device with required VID: %s", vid).Err()
	}
	// find pid path:
	pidPath, err := r(ctx, time.Minute, fmt.Sprintf(findFilePathByContentCmdGlob, pid, basePath, "idProduct"))
	if err != nil {
		return "", errors.Annotate(err, "find single usb device file system directory").Err()
	} else if !hasOnlySingleLine(ctx, pidPath) {
		return "", errors.Reason("find single usb device file system directory: found more then one device with required PID: %s", pid).Err()
	}
	// If both files locates matched then we found our device.
	commDirCmd := fmt.Sprintf("LC_ALL=C comm -12 <(dirname %s) <(dirname %s)", vidPath, pidPath)
	commDir, err := r(ctx, time.Minute, commDirCmd)
	if err != nil {
		return "", errors.Annotate(err, "find single usb device file system directory").Err()
	} else if commDir == "" || commDir == "." {
		return "", errors.Reason("find single usb device file system directory: directory not found").Err()
	}
	return commDir, nil
}

const (
	// macAddressFileUnderNetFolderOfThePathGlob find NIC address from the nic path.
	// start finding the file name that contains both the /net/ and /address/ under the nic path folder.
	macAddressFileUnderNetFolderOfThePathGlob = "find %s/ | grep /net/ | grep /address"
	// Regex string to validate that MAC address is valid.
	// example of a correct format MAC address: f4:f5:e8:50:d1:cf
	macAddressVerifyRegexp = `^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`
)

// ServoNICMacAddress read servo NIC mac address visible from DUT side.
//
// @param nic_path: Path to network device on the host
func ServoNICMacAddress(ctx context.Context, r execs.Runner, nicPath string) (string, error) {
	findNICAddressFileCmd := fmt.Sprintf(macAddressFileUnderNetFolderOfThePathGlob, nicPath)
	nicAddressFile, err := r(ctx, time.Minute, findNICAddressFileCmd)
	if err != nil {
		return "", errors.Annotate(err, "servo nic mac address").Err()
	} else if !hasOnlySingleLine(ctx, nicAddressFile) {
		return "", errors.Reason("servo nic mac address: found more then one nic address file").Err()
	}
	log.Infof(ctx, "Found servo NIC address file: %q", nicAddressFile)
	macAddress, err := r(ctx, time.Minute, fmt.Sprintf("cat %s", nicAddressFile))
	if err != nil {
		return "", errors.Annotate(err, "servo nic mac address").Err()
	}
	macAddressRegexp, err := regexp.Compile(macAddressVerifyRegexp)
	if err != nil {
		return "", errors.Annotate(err, "servo nic mac address: regular expression for correct mac address cannot compile").Err()
	}
	if !macAddressRegexp.MatchString(macAddress) {
		log.Infof(ctx, "Incorrect format of the servo nic mac address: %s", macAddress)
		return "", errors.Reason("servo nic mac address: incorrect format mac address found").Err()
	}
	log.Infof(ctx, "Servo NIC MAC address visible from DUT: %s", macAddress)
	return macAddress, nil
}
