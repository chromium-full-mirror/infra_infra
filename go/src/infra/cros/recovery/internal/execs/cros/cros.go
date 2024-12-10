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

	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
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

const (
	// waitDownRebootTime is the time the program will wait for the device to be down.
	waitDownRebootTime = 120 * time.Second
	// waitUpRebootTime is the time the program will wait for the device to be up after reboot.
	waitUpRebootTime = 240 * time.Second
)

// WaitForRestart will first wait the device to go down and then wait
// for the device to come up.
func WaitForRestart(ctx context.Context, info *execs.ExecInfo) error {
	ping := info.DefaultPinger()
	logger := info.NewLogger()
	// wait for it to be down.
	if waitDownErr := cros.WaitUntilNotPingable(ctx, waitDownRebootTime, cros.PingRetryInterval, cros.DefaultPingCount, ping); waitDownErr != nil {
		logger.Debugf("Wait For Restart: device shutdown failed.")
		return errors.Annotate(waitDownErr, "wait for restart").Err()
	}
	// wait down for servo device is successful, then wait for device
	// up.
	if waitUpErr := cros.WaitUntilPingable(ctx, waitUpRebootTime, cros.PingRetryInterval, cros.DefaultPingCount, ping); waitUpErr != nil {
		return errors.Annotate(waitUpErr, "wait for restart").Err()
	}
	logger.Infof("Device is up.")
	return nil
}

// TpmStatus is a data structure to represent the parse-version of the
// TPM Status.
type TpmStatus struct {
	statusMap map[string]string
	success   bool
}

// NewTpmStatus retrieves the TPM status for the DUT and returns the
// status values as a map.
func NewTpmStatus(ctx context.Context, run execs.Runner, timeout time.Duration) *TpmStatus {
	status, _ := run(ctx, timeout, "tpm_manager_client", "status", "--nonsensitive")
	log.Debugf(ctx, "New Tpm Status :%q", status)
	statusItems := strings.Split(status, "\n")
	var ts = &TpmStatus{
		statusMap: make(map[string]string),
		// The uppercase on this string is deliberate.
		success: strings.Contains(strings.ToUpper(status), "STATUS_SUCCESS"),
	}
	// Following the logic in Labpack, if the TPM status string
	// contains 2 lines or fewer, we will return an empty map for the
	// TPM status values.
	if len(statusItems) > 2 {
		statusItems = statusItems[1 : len(statusItems)-1]
		for _, statusLine := range statusItems {
			item := strings.Split(statusLine, ":")[:]
			if item[0] == "" {
				continue
			}
			if len(item) == 1 {
				item = append(item, "")
			}
			for i, j := range item {
				item[i] = strings.TrimSpace(j)
			}
			ts.statusMap[item[0]] = item[1]
			// The labpack (Python) implementation checks whether the
			// string item[1] contains true of false in the string
			// form, and then explicitly converts that boolean
			// values. We do not attempt that here since the key and
			// value types for maps are strongly typed in Go-lang.
		}
	}
	return ts
}

// hasSuccess checks whether the TpmStatus includes success indicator
// or not.
func (tpmStatus *TpmStatus) hasSuccess() bool {
	return tpmStatus.success
}

// isOwned checks whether TPM has been cleared or not.
func (tpmStatus *TpmStatus) isOwned() (bool, error) {
	if len(tpmStatus.statusMap) == 0 {
		return false, errors.Reason("tpm status is owned: not initialized").Err()
	}
	return tpmStatus.statusMap["is_owned"] == "true", nil
}

// SimpleReboot executes a simple reboot command using a command
// runner for a DUT.
func SimpleReboot(ctx context.Context, run execs.Runner, timeout time.Duration, info *execs.ExecInfo) error {
	rebootCmd := "reboot"
	log.Debugf(ctx, "Simple Rebooter : %s", rebootCmd)
	out, _ := run(ctx, timeout, rebootCmd)
	log.Debugf(ctx, "Stdout: %s", out)
	if restartErr := WaitForRestart(ctx, info); restartErr != nil {
		return errors.Annotate(restartErr, "simple reboot").Err()
	}
	return nil
}
