// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tlw

import (
	"fmt"
	"testing"
)

var extractDutNameTestCases = []struct {
	caseName string
	in       string
	out      string
}{
	{"empty", "", ""},
	{"dut", "dut-name", "dut-name"},
	{"ap1", "dut1-row1-host1-router", "dut1-row1-host1"},
	{"ap2", "dut1-row1-host1-pcap", "dut1-row1-host1"},
	{"btpeer1", "dut1-row1-host1-btpeer1", "dut1-row1-host1"},
	{"btpeer2", "dut1-row1-host1-btpeer2", "dut1-row1-host1"},
	{"btpeer3", "dut1-row1-host1-btpeer3", "dut1-row1-host1"},
	{"btpeer4", "dut1-row1-host1-btpeer4", "dut1-row1-host1"},
	{"btpeer5", "dut1-row1-host1-btpeer5", "dut1-row1-host1"},
	{"btpeer6", "dut1-row1-host1-btpeer6", "dut1-row1-host1"},
}

func TestExtractDutName(t *testing.T) {
	t.Parallel()
	for _, tc := range extractDutNameTestCases {
		tc := tc
		name := fmt.Sprintf("case %s", tc.caseName)
		t.Run(name, func(t *testing.T) {
			got := extractDutName(tc.in)
			if tc.out != got {
				t.Errorf("extractDutName(deviceName) returned %q but expected %q", got, tc.out)
			}
		})
	}
}
