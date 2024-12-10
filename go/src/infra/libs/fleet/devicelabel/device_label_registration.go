// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"infra/libs/fleet"
	ufspb "infra/unifiedfleet/api/v1/models"
)

type Registration struct {
	name          string
	schedulableID string
	source        fleet.DeviceLabel_SOURCE
	reasonToAdd   string
	owner         string

	// The func to set the value of this device label
	getValue applyValueFunc
}

type applyValueFunc func(*ufspb.ChromeOSDeviceData) (protoreflect.ProtoMessage, error)

var labelRegs = []*Registration{
	{
		name:          "arc",
		schedulableID: "label-arc",
		source:        fleet.DeviceLabel_SOURCE_MANUAL_INPUT,
		reasonToAdd:   "Decided by model, from DT's manual input or HaRT if the former doesn't exist",
		getValue:      applyArc,
	},
}
