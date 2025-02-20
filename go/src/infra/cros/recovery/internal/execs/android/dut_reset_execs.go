// Copyright 2022 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/android/adb"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// reconnectOfflineDutExec reconnects offline DUT.
func reconnectOfflineDutExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.ResetUsbDevice(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "reconnect offline dut").Err()
	}
	return nil
}

// restartADBDAsRootExec restarts adbd as root on the device.
func restartADBDAsRootExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.RestartADBDAsRoot(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "restart adbd as root").Err()
	}
	// Android device may be not available or flaky for a short period of time after switch to root adb mode.
	// So we want to wait 20 seconds here to avoid timing issue causes later action fails.
	time.Sleep(20 * time.Second)
	return nil
}

// unrootADBDExec restarts adbd on the device without root permissions.
func unrootADBDExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.UnrootADBD(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "unroot adbd").Err()
	}
	return nil
}

// removeScreenLockExec removes lock from DUT screen.
func removeScreenLockExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.RemoveScreenLock(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "remove screen lock").Err()
	}
	return nil
}

// rebootDutExec reboots DUT.
func rebootDutExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.RebootDevice(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "dut reboot").Err()
	}
	return nil
}

// enableTestHarnessExec enables test harness mode on DUT.
func enableTestHarnessExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.EnableDeviceTestHarnessMode(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "dut reset").Err()
	}
	return nil
}

// waitTillDutOfflineExec waits till DUT disconnects from associated host.
func waitTillDutOfflineExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	actionArgs := info.GetActionArgs(ctx)
	timeout := actionArgs.AsDuration(ctx, "timeout", 60, time.Second)
	err := adb.WaitForDeviceState(ctx, adb.Unknown, 1, timeout, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "wait for offline dut").Err()
	}
	return nil
}

// waitTillDutOnlineExec waits till DUT online.
func waitTillDutOnlineExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	timeout := actionArgs.AsDuration(ctx, "timeout", 600, time.Second)
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.WaitForDeviceState(ctx, adb.Device, 3, timeout, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "wait for online dut").Err()
	}
	return nil
}

// enableWiFiExec enables WiFi on DUT.
func enableWiFiExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	retryInterval := actionArgs.AsDuration(ctx, "retry_interval", 5, time.Second)
	timeout := actionArgs.AsDuration(ctx, "timeout", 60, time.Second)
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.EnableWiFi(ctx, retryInterval, timeout, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "enable wifi").Err()
	}
	return nil
}

// connectToWiFiNetworkExec connects DUT to WiFi network.
func connectToWiFiNetworkExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	if !actionArgs.Has("wifi_ssid") {
		return errors.Reason("invalid number of arguments: wifi ssid is required").Err()
	}
	if !actionArgs.Has("wifi_security") {
		return errors.Reason("invalid number of arguments: wifi security type is required").Err()
	}
	serialNumber := info.GetAndroid().GetSerialNumber()
	ssid := actionArgs.AsString(ctx, "wifi_ssid", "")
	securityType := actionArgs.AsString(ctx, "wifi_security", "")
	password := actionArgs.AsString(ctx, "wifi_password", "")
	retryInterval := actionArgs.AsDuration(ctx, "retry_interval", 5, time.Second)
	timeout := actionArgs.AsDuration(ctx, "timeout", 60, time.Second)
	err := adb.ConnectToWiFiNetwork(ctx, retryInterval, timeout, newRunner(info), info.NewLogger(), serialNumber, ssid, securityType, password)
	if err != nil {
		return errors.Annotate(err, "connect wifi network").Err()
	}
	return nil
}

func resetPublicKeyExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	publicKeyFile := actionArgs.AsString(ctx, "public_key_file", "")
	if publicKeyFile == "" {
		return errors.Reason("reset public key: adb public key location is missing").Err()
	}
	publicKey := actionArgs.AsString(ctx, "public_key", "")
	if publicKey == "" {
		return errors.Reason("reset public key: adb public key value is missing").Err()
	}
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.ResetADBDPublicKey(ctx, newRunner(info), info.NewLogger(), serialNumber, publicKeyFile, publicKey)
	if err != nil {
		return errors.Annotate(err, "reset public key").Err()
	}
	return nil
}

func init() {
	execs.Register("android_reconnect_offline_dut", reconnectOfflineDutExec)
	execs.Register("android_restart_adbd_as_root", restartADBDAsRootExec)
	execs.Register("android_unroot_adbd", unrootADBDExec)
	execs.Register("android_remove_screen_lock", removeScreenLockExec)
	execs.Register("android_dut_reboot", rebootDutExec)
	execs.Register("android_enable_test_harness", enableTestHarnessExec)
	execs.Register("android_wait_for_offline_dut", waitTillDutOfflineExec)
	execs.Register("android_wait_for_online_dut", waitTillDutOnlineExec)
	execs.Register("android_enable_wifi", enableWiFiExec)
	execs.Register("android_connect_wifi_network", connectToWiFiNetworkExec)
	execs.Register("android_reset_public_key", resetPublicKeyExec)
}
