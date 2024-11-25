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
