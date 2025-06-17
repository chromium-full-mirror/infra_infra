// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// FlossEnabled returns true if floss is enabled on the DUT.
func FlossEnabled(ctx context.Context, r components.Runner, timeout time.Duration) bool {
	// cmd will either exit with nonzero code if floss is not enabled or
	// a single DBus value similar to: '\s+boolean\s+true'
	// e.x.
	//    boolean true
	cmd := flossDBusCmd("GetFlossEnabled")
	output, err := r(ctx, timeout, cmd)
	if err != nil {
		return false
	}

	// check that returned DBus value is true
	enabledValue := []string{"boolean", "true"}
	lines := strings.Split(output, "\n")
	if len(lines) == 1 {
		return outputHasValue(lines[0], enabledValue)
	}
	return false
}

// HasAdapterBlueZ checks if a bluetooth adapter is detected using the BlueZ DBus service.
func HasAdapterBlueZ(ctx context.Context, r components.Runner, timeout time.Duration) (bool, error) {
	// cmd will either exit with nonzero code if bluetooth is not detected or will return
	// a single DBus value similar to: '\s*variant\s+boolean'
	// Note: --print-reply=literal only returns the body of the reply so no need to strip header.
	// e.x.
	//     variant       boolean true
	const cmd = `dbus-send --print-reply=literal ` +
		`--system --dest=org.bluez /org/bluez/hci0 ` +
		`org.freedesktop.DBus.Properties.Get ` +
		`string:"org.bluez.Adapter1" string:"Powered"`
	output, err := r(ctx, timeout, cmd)
	if err != nil {
		return false, errors.Annotate(err, "has adapter BlueZ").Err()
	}

	// check that returned DBus value is true
	enabledValue := []string{"variant", "boolean"}
	lines := strings.Split(output, "\n")
	if len(lines) == 1 {
		return outputHasValue(lines[0], enabledValue), nil
	}
	return false, nil
}

// HasAdapterFloss checks if a bluetooth adapter is detected using the Floss DBus service.
// DBus function GetAvailableAdapters will check if a adapter is detected. Running DBus functions Start
// and GetAdapterEnabled will detect adapters in non-working state
func HasAdapterFloss(ctx context.Context, r components.Runner, timeout time.Duration) (bool, error) {
	// GetAvailableAdapters returns an array of DBus properties for the detected bluetooth adapters.
	// Presence of "enabled variant" indicate that a adapter is present.
	// The boolean reflects the power state of the adapter and can be ignored.
	// e.x.
	// array [
	//  array [
	//    dict entry(
	//      enabled            variant                boolean true
	//    )
	//    dict entry(
	//      hci_interface            variant                int32 0
	//    )
	//  ]
	// ]
	getAdapterCmd := flossDBusCmd("GetAvailableAdapters")
	// This line will be present in output if enabled adapter is found
	expectedOutput := []string{"enabled", "variant", "boolean"}

	output, err := r(ctx, timeout, getAdapterCmd)
	if err != nil {
		return false, errors.Annotate(err, "has adapter floss").Err()
	}
	if !outputHasValue(output, expectedOutput) {
		return false, nil
	}
	// Some bluetooth adapters can go into a non-responsive state where they don't respond to any commands, even though
	// they appear in GetAvailableAdapters. We can detect these by powering them on by Start command and then checking
	// the state via GetAdapterEnabled cmd.
	// DBus cmd dbus-send --system --print-reply --dest=org.chromium.bluetooth.Manager org/chromium/bluetooth/Manager org.chromium.bluetooth.Manager.Start int32:0
	// This command with try to power on the adapter. This command doesn't generate any output and always succeeds,
	// hence we need to use GetAdapterEnabled to confirm.
	// The adapter index will always be 0 (hci0) unless a external adapter is plugged-in, which is not supported.
	startCmd := flossDBusCmd("Start", "int32:0")
	if _, err := r(ctx, timeout, startCmd); err != nil {
		return false, errors.Annotate(err, "floss adapter start").Err()
	}

	// Wait for 2 seconds for adapter to Start
	time.Sleep(2 * time.Second)

	// Dbus cmd dbus-send --system --print-reply --dest=org.chromium.bluetooth.Manager /org/chromium/bluetooth/Manager org.chromium.bluetooth.Manager.GetAdapterEnabled int32:0
	// This command will check if the adapter is powered on or not. The output will of the format
	// boolean true
	// Power state should be true if the adapter responded to the earlier Start command.
	expectedOutput = []string{"boolean", "true"}

	getEnabledCmd := flossDBusCmd("GetAdapterEnabled", "int32:0")
	output, err = r(ctx, timeout, getEnabledCmd)
	if err != nil {
		return false, errors.Annotate(err, "floss adapter enabled").Err()
	}

	return outputHasValue(output, expectedOutput), nil
}

// flossDBusCmd constructs commands to floss bluetooth manager.
// It can optionally take D-Bus method arguments.
func flossDBusCmd(method string, cmdArgs ...string) string {
	const service = "org.chromium.bluetooth.Manager"
	const path = "/org/chromium/bluetooth/Manager"
	const iface = "org.chromium.bluetooth.Manager"

	params := fmt.Sprintf("--system --print-reply=literal --dest=%s %s %s.%s", service, path, iface, method)
	if len(cmdArgs) > 0 {
		params = params + " " + strings.Join(cmdArgs, " ")
	}
	return "dbus-send " + params
}

// isLineStartWithSameWords returns true if the split text matches the provided string array
// up to the length of the provided string array.
func isLineStartWithSameWords(line string, match []string) bool {
	fields := strings.Fields(line)
	length := len(match)
	if len(fields) > length {
		fields = fields[0:length]
	}
	return reflect.DeepEqual(fields, match)
}

// outputHasValue check whether value is present in the output
func outputHasValue(output string, value []string) bool {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if isLineStartWithSameWords(line, value) {
			return true
		}
	}
	return false
}
