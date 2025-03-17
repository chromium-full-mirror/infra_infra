// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

// DeviceTypeForValidations specifies device-types and flag if validation is supported.
var DeviceTypeForValidations = map[string]bool{
	"cros":              true,
	"androidos":         false,
	"camera_box_tablet": true,
	"wifi_router":       false,
}
