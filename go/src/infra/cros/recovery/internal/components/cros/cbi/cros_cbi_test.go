// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cbi

import (
	"reflect"
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// TestBuildCBILocation tests that the BuildCBILocation function can correctly
// parse the output of the `ectool locatechip` command to get the port and
// address of CBI on a DUT and return it as a CBILocation. Does not actually
// invoke the `ectool locatechip` command.
func TestBuildCBILocation(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		locateCBIOutput     string
		expectedCBILocation *CBILocation
	}{
		{
			"Bus: I2C; Port: 0; Address: 0x50 (7-bit format)",
			&CBILocation{
				port:    "0",
				address: "0x50",
			},
		},
		{
			"Bus: I2C; Port: 9999; Address: 0xF00BAD (7-bit format)",
			&CBILocation{
				port:    "9999",
				address: "0xF00BAD",
			},
		},
		{
			"Port: 9999; Address: 0xF00BAD",
			&CBILocation{
				port:    "9999",
				address: "0xF00BAD",
			},
		},
		{
			"Port: 9999; 0xF00BAD",
			nil,
		},
		{
			"Port: 9999; 0xF00BAD",
			nil,
		},
		{
			"Port: 9999",
			nil,
		},
		{
			"Address: 0xF00BAD",
			nil,
		},
		{
			"",
			nil,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.locateCBIOutput, func(t *testing.T) {
			t.Parallel()
			cbiLocation, _ := buildCBILocation(tt.locateCBIOutput)
			if !reflect.DeepEqual(cbiLocation, tt.expectedCBILocation) {
				t.Errorf(
					"Expected CBI Location %+v\n but got %+v\n",
					tt.expectedCBILocation,
					cbiLocation)
			}
		})
	}
}

// TestParseBytesFromCBIContents tests that the output of calls to the
// `ectool i2cxfer` command can be properly broken down into a slice of hex
// bytes. e.g. "0x43" or "00"
func TestParseBytesFromCBIContents(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		cbiContents      string
		numBytesToRead   int
		expectedHexBytes []string
	}{
		{
			"Read bytes: 0x43 0x1 00 0xff",
			4,
			[]string{"0x43", "0x1", "00", "0xff"},
		},
		{
			"Read bytes: 0x43 0x1 00 0xff",
			2,
			[]string{"0x43", "0x1"},
		},
		{
			"Read bytes: 0x43",
			2,
			nil,
		},
		{
			"junk",
			1,
			nil,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.cbiContents, func(t *testing.T) {
			t.Parallel()
			hexBytes, _ := parseBytesFromCBIContents(tt.cbiContents, tt.numBytesToRead)
			if !reflect.DeepEqual(hexBytes, tt.expectedHexBytes) {
				t.Errorf(
					"Expected Hex Bytes %+v\n but got %+v\n",
					tt.expectedHexBytes,
					hexBytes)
			}
		})
	}
}

// TestContainsCBIMagic tests whether the RawContents of a CBI proto start with
// the CBI magic bytes.
func TestContainsCBIMagic(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		cbi          *labapi.Cbi
		expectedBool bool
	}{
		{
			&labapi.Cbi{RawContents: cbiMagic}, true,
		},
		{
			&labapi.Cbi{RawContents: "junk"}, false,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.cbi.GetRawContents(), func(t *testing.T) {
			t.Parallel()
			actualBool := ContainsCBIMagic(tt.cbi)
			if actualBool != tt.expectedBool {
				t.Errorf(
					"Expected %t\n but got %t\n",
					tt.expectedBool,
					actualBool)
			}
		})
	}
}
