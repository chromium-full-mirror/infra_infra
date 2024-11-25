// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"strings"
)

// Type represents type of version data per type of target device.
type Type string

const (
	UnspecifiedType Type = ""
	// Chrome OS.
	CrOSType Type = "cros"
	// AP/PCAP peripheral.
	WifiRouterType Type = "wifi_router"
	// Tablet for camera-box.
	CameraBoxTabletType Type = "camera_box_tablet"
	// Android for ChromeOS.
	AndroidOSType Type = "androidos"
)

// ToType converts string to Type.
func ToType(t string) Type {
	if t != "" {
		tt := Type(strings.ToLower(t))
		switch tt {
		case CrOSType, WifiRouterType, CameraBoxTabletType, AndroidOSType:
			return tt
		}
	}
	return CrOSType
}

func defaultTypeIfEmpty(t Type, deviceName string) Type {
	if t == UnspecifiedType {
		t = CrOSType
	}
	if t == CrOSType && strings.HasSuffix(deviceName, "-tablet") {
		t = CameraBoxTabletType
	}
	return t
}

func toDeviceType(t Type) string {
	return string(t)
}
