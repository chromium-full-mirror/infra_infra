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
		return splitEquals(lines[0], enabledValue)
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
		return splitEquals(lines[0], enabledValue), nil
	}
	return false, nil
}

// HasAdapterFloss checks if a bluetooth adapter is detected using the Floss DBus service.
func HasAdapterFloss(ctx context.Context, r components.Runner, timeout time.Duration) (bool, error) {
	// cmd returns an array of DBus properties for the detected bluetooth adapters
	// The boolean reflects the power state of the adapter and can be ignored
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
	cmd := flossDBusCmd("GetAvailableAdapters")
	output, err := r(ctx, timeout, cmd)
	if err != nil {
		return false, errors.Annotate(err, "has adapter floss").Err()
	}

	// check that a single enabled adapter is found
	enabledValue := []string{"enabled", "variant", "boolean"}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if splitEquals(line, enabledValue) {
			return true, nil
		}
	}
	return false, nil
}

// flossDBusCmd constructs commands to floss bluetooth manager.
func flossDBusCmd(method string) string {
	const service = "org.chromium.bluetooth.Manager"
	const path = "/org/chromium/bluetooth/Manager"
	const iface = " org.chromium.bluetooth.Manager"
	return fmt.Sprintf("dbus-send --print-reply=literal --system --dest=%s %s %s.%s", service, path, iface, method)
}

// splitEquals returns true if the split text matches the provided string array
// up to the length of the provided string array.
func splitEquals(line string, match []string) bool {
	fields := strings.Fields(line)
	length := len(match)
	if len(fields) > length {
		fields = fields[0:length]
	}
	return reflect.DeepEqual(fields, match)
}
