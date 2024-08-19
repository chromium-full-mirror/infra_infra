// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"fmt"
	"strings"

	"go.chromium.org/chromiumos/infra/proto/go/lab"

	ca "infra/libs/fleet/protos"
	fleet "infra/libs/fleet/protos/go"
)

// Host, project and branch to get dhcpd.conf file
var host string = "chrome-internal.googlesource.com"
var project string = "chromeos/chromeos-admin"
var branch string = "master"
var path string = "puppet/modules/lab/files/dhcp-server/dhcpd.conf"

// GetHostname returns the hostname of input ChromeOSDevice.
func GetHostname(d *lab.ChromeOSDevice) string {
	switch t := d.GetDevice().(type) {
	case *lab.ChromeOSDevice_Dut:
		return d.GetDut().GetHostname()
	case *lab.ChromeOSDevice_Labstation:
		return d.GetLabstation().GetHostname()
	default:
		panic(fmt.Sprintf("Unknown device type: %v", t))
	}
}

// SanitizeChopsAsset removes all the trailing and leading whitespaces in
// all ChopsAsset proto string fields in the input slice
func SanitizeChopsAsset(a []*ca.ChopsAsset) []*ca.ChopsAsset {
	for idx, asset := range a {
		a[idx] = trimWhiteSpaceInChopsAsset(asset)
	}
	return a
}

// trimWhitespaceInChopsAsset trims trailing and leading whitespace in
// ChopsAsset proto
func trimWhiteSpaceInChopsAsset(a *ca.ChopsAsset) *ca.ChopsAsset {
	if a == nil {
		return a
	}
	a.Id = strings.TrimSpace(a.Id)
	a.Location = trimWhiteSpaceInLocation(a.Location)
	return a
}

// trimWhitespaceInLocation trims trailing and leading whitespace in Location
func trimWhiteSpaceInLocation(a *fleet.Location) *fleet.Location {
	if a == nil {
		return a
	}
	a.Lab = strings.TrimSpace(a.Lab)
	a.Aisle = strings.TrimSpace(a.Aisle)
	a.Row = strings.TrimSpace(a.Row)
	a.Rack = strings.TrimSpace(a.Rack)
	a.Shelf = strings.TrimSpace(a.Shelf)
	a.Position = strings.TrimSpace(a.Position)
	return a
}
