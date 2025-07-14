// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package flags defines flags for the fleetconsoleserver.
package flags

import (
	"flag"
)

var DeviceManagerAddr = flag.String("dm-addr", "", "Device Manager address to use. Uses production address by default")
var UseLocalDeviceManager = flag.Bool("use-local-dm", false, "Uses insecure connection to device manager. Default address is localhost:8800. Can be overwritten by dm-addr flag.")
var UfsAddr = flag.String("ufs-addr", "", "UFS address to use. Uses production address by default")
var UseLocalUfs = flag.Bool("use-local-ufs", false, "Uses insecure connection to UFS. Default address is localhost:8800. Can be overwritten by ufs-addr flag.")

// TODO (b/394429368): Determine correct addresses
var AdminServiceAddr = flag.String("admin-service-addr", "chromeos-skylab-bot-fleet.appspot.com", "Admin Service address for managing fleet admin tasks. Uses production address by default")
var InventoryNamespace = flag.String("inventory-namespace", "os", "Inventory namespace to use. Default namespace for OS devices")
var CIPDVersion = flag.String("cipd-version", "", "CIPD version to use. Uses production version by default")
