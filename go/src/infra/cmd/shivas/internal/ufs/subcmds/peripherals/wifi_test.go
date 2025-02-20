// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"strings"
	"testing"

	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

func TestWifiCleanAndValidateFlags(t *testing.T) {
	// Test invalid flags
	errTests := []struct {
		cmd  *manageWifiCmd
		want []string
	}{
		{
			cmd:  &manageWifiCmd{},
			want: []string{errDUTMissing, errNoRouter},
		},
		{
			cmd:  &manageWifiCmd{routers: [][]string{{"hostname: "}}},
			want: []string{errDUTMissing, errNoRouter, errEmptyHostname},
		},
		{
			cmd:  &manageWifiCmd{routers: [][]string{{"hostname:h1 "}, {"hostname:h1"}}, dutName: "d1"},
			want: []string{errDuplicateHostname},
		},
	}

	for _, tt := range errTests {
		err := tt.cmd.cleanAndValidateFlags()
		if err == nil {
			t.Errorf("cleanAndValidateFlags = nil; want errors: %v", tt.want)
			continue
		}
		for _, errStr := range tt.want {
			if !strings.Contains(err.Error(), errStr) {
				t.Errorf("cleanAndValidateFlags = %q; want err %q included", err, errStr)
			}
		}
	}

	// Test valid flags with hostname cleanup
	c := &manageWifiCmd{
		dutName: "d",
		routers: [][]string{
			{
				"hostname:h1",
				"model:test",
				"supported_feature:WIFI_ROUTER_FEATURE_IEEE_802_11_A",
				"supported_feature:WIFI_ROUTER_FEATURE_IEEE_802_11_B",
			},
			{
				"hostname:h2",
			},
		},
		mode: actionAdd,
	}
	if err := c.cleanAndValidateFlags(); err != nil {
		t.Errorf("cleanAndValidateFlags = %v; want nil", err)
	}
	wantRouters := 2
	if n := len(c.routers); n != wantRouters {
		t.Errorf("len(c.routers) = %d; want %d", n, wantRouters)
	}

}

func TestAddWifiRouters(t *testing.T) {
	cmd := &manageWifiCmd{
		dutName: "d",
		routers: [][]string{
			{
				"hostname:h1",
				"model:test",
				"supported_feature:WIFI_ROUTER_FEATURE_IEEE_802_11_A",
				"supported_feature:WIFI_ROUTER_FEATURE_IEEE_802_11_B",
			},
			{
				"hostname:h2",
			},
		},
		mode: actionAdd,
	}
	if err := cmd.cleanAndValidateFlags(); err != nil {
		t.Errorf("unexpected cleanAndValidateFlags() failure = %v", err)
	}

	// Test adding a duplicate and a valid BTP
	current := &lab.Wifi{
		WifiRouters: []*lab.WifiRouter{
			{
				Hostname: "h1",
			},
		},
	}

	if _, err := cmd.addWifiRouters(current, cmd.dutName); err == nil {
		t.Errorf("addWifiRouters(%v) succeeded, expect duplication failure", current)
	}

	// Test adding two valid routers.
	current = &lab.Wifi{
		WifiRouters: []*lab.WifiRouter{
			{
				Hostname: "h3",
			},
		},
	}
	out, err := cmd.addWifiRouters(current, cmd.dutName)
	if err != nil {
		t.Errorf("addWifiRouters(%v) = %v, expect success", current, err)
	}
	wantRouters := 3
	if len(out.GetWifiRouters()) != wantRouters {
		t.Errorf("addWifiRouters(%v) = %v, want total wifirouters %d", current, out.GetWifiRouters(), wantRouters)
	}
}

func TestDeleteWifiRouters(t *testing.T) {
	cmd := &manageWifiCmd{
		dutName: "d",
		routers: [][]string{
			{
				"hostname:h1",
				"model:test",
				"supported_feature:WIFI_ROUTER_FEATURE_IEEE_802_11_A",
				"supported_feature:WIFI_ROUTER_FEATURE_IEEE_802_11_B",
			},
			{
				"hostname:h2",
			},
		},
		mode: actionDelete,
	}
	if err := cmd.cleanAndValidateFlags(); err != nil {
		t.Errorf("unexpected cleanAndValidateFlags() failure = %v", err)
	}

	// Test deleting two non-existent BTPs
	current := &lab.Wifi{
		WifiRouters: []*lab.WifiRouter{
			{
				Hostname: "h3",
			},
		},
	}

	if _, err := cmd.deleteWifiRouters(current, cmd.dutName); err == nil {
		t.Errorf("deleteWifiRouters(%v) succeeded, expected non-existent delete failure", current)
	}

	// Test deleting 2 of 3 routers
	current = &lab.Wifi{
		WifiRouters: []*lab.WifiRouter{
			{
				Hostname: "h1",
			},
			{
				Hostname: "h2",
			},
			{
				Hostname: "h3",
			},
		},
	}

	out, err := cmd.deleteWifiRouters(current, cmd.dutName)
	if err != nil {
		t.Errorf("deleteWifiRouters(%v) = %v, expect success", current, err)
	}
	want := "h3"
	if len(out.GetWifiRouters()) != 1 || out.GetWifiRouters()[0].GetHostname() != want {
		t.Fatalf("deleteWifiRouters(%v) = %v, want %s", current, out.GetWifiRouters(), want)
	}
}

func TestReplaceWifiRouters(t *testing.T) {
	cmd := &manageWifiCmd{
		dutName: "d",
		routers: [][]string{
			{
				"hostname:h1",
				"model:test",
				"supported_feature:WIFI_ROUTER_FEATURE_UNKNOWN",
			},
			{"hostname:h2"},
		},
		mode: actionReplace,
	}
	if err := cmd.cleanAndValidateFlags(); err != nil {
		t.Errorf("unexpected cleanAndValidateFlags() failure = %v", err)
	}

	// Test replace two non-existent BTPs
	current := &lab.Wifi{
		WifiRouters: []*lab.WifiRouter{
			{
				Hostname: "h3",
			},
		},
	}
	want := 2
	if out, _ := cmd.replaceWifiRouters(current, cmd.dutName); len(out.GetWifiRouters()) != want {
		t.Errorf("replaceWifiRouters(%v) = %v, want %d replacing failure", current, out, want)
	}

}
