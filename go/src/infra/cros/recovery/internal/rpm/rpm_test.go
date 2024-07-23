// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpm

import (
	"context"
	"strings"
	"testing"
)

func TestSetPowerStateSentry(t *testing.T) {
	// Function not implemented
	ctx := context.Background()
	for _, tc := range []struct {
		input     *RPMPowerRequest
		expectErr bool
		errString string
	}{
		{
			&RPMPowerRequest{
				Hostname:          "",
				PowerUnitHostname: "",
				PowerunitOutlet:   "",
				State:             "",
				Type:              "",
			},
			true,
			"Not implemented",
		},
	} {
		err := setPowerStateSentry(ctx, tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("setPowerStateSentry(%q) unexpectedly succeeded", tc.input)
			}
			if !strings.Contains(err.Error(), tc.errString) {
				t.Errorf("setPowerStateSentry(%q) expected error: %v got: %v", tc.input, tc.errString, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("setPowerStateSentry(%q) failed: %v", tc.input, err)
			}
		}
	}
}

func TestSetPowerStateIP9850(t *testing.T) {
	// Function not implemented
	ctx := context.Background()
	for _, tc := range []struct {
		input     *RPMPowerRequest
		expectErr bool
		errString string
	}{
		{
			&RPMPowerRequest{
				Hostname:          "",
				PowerUnitHostname: "",
				PowerunitOutlet:   "",
				State:             "",
				Type:              "",
			},
			true,
			"Not implemented",
		},
	} {
		err := setPowerStateIP9850(ctx, tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("setPowerStateIP9850(%q) unexpectedly succeeded", tc.input)
			}
			if !strings.Contains(err.Error(), tc.errString) {
				t.Errorf("setPowerStateIP9850(%q) expected error: %v got: %v", tc.input, tc.errString, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("setPowerStateIP9850(%q) failed: %v", tc.input, err)
			}
		}
	}
}
