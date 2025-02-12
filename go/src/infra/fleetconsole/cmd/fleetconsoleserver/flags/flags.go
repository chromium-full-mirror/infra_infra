// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package flags defines flags for the fleetconsoleserver.
package flags

import (
	"flag"
)

const pgConnectionString = `postgresql://postgres:password@localhost:5432/fleet_console_db`

var DeviceManagerAddr = flag.String("dm-addr", "", "Device Manager address to use. Uses production address by default")
var UseLocalDeviceManager = flag.Bool("use-local-dm", false, "Uses insecure connection to device manager. Default address is localhost:8800. Can be overwritten by dm-addr flag.")
var UfsAddr = flag.String("ufs-addr", "", "UFS address to use. Uses production address by default")
var UseLocalUfs = flag.Bool("use-local-ufs", false, "Uses insecure connection to UFS. Default address is localhost:8800. Can be overwritten by ufs-addr flag.")
var DBSecret = flag.String("db-uri-secret", "devsecret-text://"+pgConnectionString, "Path to db secret")
var UseDevDB = flag.Bool("use-dev-db", false, "Connect to the dev db from localhost, for more info read README.md")
