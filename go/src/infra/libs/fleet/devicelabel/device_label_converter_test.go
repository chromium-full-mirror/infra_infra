// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
)

// TestConvert tests that Convert can successfully converts a UFS entry to a label-based representation.
func TestConvert(t *testing.T) {
	t.Parallel()

	device, err := ConvertChromeOS(fakeChromeOSData)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, device, should.NotBeNil)
	assert.Loosely(t, len(device.GetDeviceLabels()), should.Equal(1))
	l := device.GetDeviceLabels()[0]
	verifyBool(t, l.GetValue(), false)
}

func verifyBool(t *testing.T, v *anypb.Any, expected bool) {
	actualV, err := v.UnmarshalNew()
	assert.Loosely(t, err, should.BeNil)
	msg, ok := actualV.(*wrapperspb.BoolValue)
	assert.Loosely(t, ok, should.Equal(true))
	assert.Loosely(t, msg.GetValue(), should.Equal(expected))
}

var fakeChromeOSData = &ufspb.ChromeOSDeviceData{
	Machine: &ufspb.Machine{
		Name: "fake-machine",
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "fizz-labstation",
			},
		},
	},
}
