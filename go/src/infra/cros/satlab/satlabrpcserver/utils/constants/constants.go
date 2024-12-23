// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package constants

import (
	"time"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"

	"infra/cros/satlab/common/services/build_service"
)

// F64Epsilon Machine epsilon value for f64
const F64Epsilon = 2.2204460492503131e-16

// SSHKeyPath the path where ssh key
const SSHKeyPath = "/home/moblab/.ssh/testing_rsa"

// SSHUser the username of DUT ssh
const SSHUser = "root"

// SSHPort the port of DUT
const SSHPort = "22"

// SSHConnectionTimeout timeout of ssh connection
const SSHConnectionTimeout = time.Second * 3

// SSHRetryDelay retry delay of ssh
const SSHRetryDelay = time.Millisecond * 300

// SSHMaxRetry the retry time
const SSHMaxRetry = 1

// VPDKeySerialNumber VPD key for serial number
const VPDKeySerialNumber = "serial_number"

// VPDKeyEthernetMAC VPD key for ethernet mac
const VPDKeyEthernetMAC = "ethernet_mac"

// GetPeripheralInfoCommand the command of get peripheral information
const GetPeripheralInfoCommand = "fwupdmgr get-devices --json"

// LogDirectory the path of log directory
const LogDirectory = "/var/log/satlab"

// GCSObjectURLTemplate the template of log url
const GCSObjectURLTemplate = "https://console.developers.google.com/storage/browser/_details/%s"

const ListFirmwareCommand = "fwid=`timeout 5 crossystem fwid`;" +
	"model=`timeout 5 cros_config / name`;" +
	"fw_update=`timeout 20 chromeos-firmwareupdate --manifest`;" +
	"printf \"{\\\"fwid\\\": \\\"%s\\\",\\\"model\\\": \\\"%s\\\", \\\"fw_update\\\":%s}\" $fwid $model \"$fw_update\""

const UpdateFirmwareCommand = "/usr/sbin/chromeos-firmwareupdate --mode autoupdate --force"

const GrepLSBReleaseCommand = "timeout 2 cat /etc/lsb-release"

const CCDStatusCommand = "gsctool -aI"

const ChromeosTestImageReleaseTrack = "chromeos_release_track=testimage-channel"

const ChromeosReleaseBoard = "CHROMEOS_RELEASE_BOARD="

var GetModelCommands = []string{
	"cros_config / test-label",
	"cros_config / name",
}

const GSCSerialNumberCommand = "timeout 2 trunks_send --sysinfo | grep DEV_ | sed 's/.*://g' | sed 's/0x//g' | tr ' ' '-' | tr 'a-z' 'A-Z' | sed -e 's/^[-]*//'"
const ServoUSBConnectorCommand = "timeout 2 cat /sys/bus/usb/devices/*/idVendor | grep -cx 04b4"
const GetGSCSerialAndServoUSB = "gsc_serial=`" + GSCSerialNumberCommand + "`;" +
	"servo_usb_count=`" + ServoUSBConnectorCommand + "`;" +
	"printf \"{\\\"gsc_serial\\\": \\\"%s\\\",\\\"servo_usb_count\\\": %s}\" $gsc_serial $servo_usb_count"

var ToResponseBuildStatusMap = map[build_service.BuildStatus]pb.BuildItem_BuildStatus{
	build_service.AVAILABLE: pb.BuildItem_BUILD_STATUS_PASS,
	build_service.FAILED:    pb.BuildItem_BUILD_STATUS_FAIL,
	build_service.RUNNING:   pb.BuildItem_BUILD_STATUS_RUNNING,
	build_service.ABORTED:   pb.BuildItem_BUILD_STATUS_ABORTED,
}
