// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package buildbucket

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// TestValidateTaskName tests that task names are validated correctly.
func TestValidateTaskName(t *testing.T) {
	t.Parallel()
	ftt.Run("validate", t, func(t *ftt.Test) {
		assert.Loosely(t, ValidateTaskName(""), should.NotBeNil)
		assert.Loosely(t, ValidateTaskName("audit_rpm"), should.BeNil)
		assert.Loosely(t, ValidateTaskName("deep_recovery"), should.BeNil)
		assert.Loosely(t, ValidateTaskName("audit____"), should.NotBeNil)
	})
}

var TaskNameToBuilderPerVersionCases = []struct {
	want     string
	taskName TaskName
	version  CIPDVersion
}{
	{"audit-rpm", AuditRPM, CIPDProd},
	{"audit-rpm-latest", AuditRPM, CIPDLatest},
	{"audit-storage", AuditStorage, CIPDProd},
	{"audit-storage-latest", AuditStorage, CIPDLatest},
	{"audit-servo-usb-key", AuditUSB, CIPDProd},
	{"audit-servo-usb-key-latest", AuditUSB, CIPDLatest},
	{"repair", Recovery, CIPDProd},
	{"repair-latest", Recovery, CIPDLatest},
	{"repair", MHRecovery, CIPDProd},
	{"repair-latest", MHRecovery, CIPDLatest},
	{"repair", DeepRecovery, CIPDProd},
	{"repair-latest", DeepRecovery, CIPDLatest},
	{"deploy", Deploy, CIPDProd},
	{"deploy-latest", Deploy, CIPDLatest},
	{"mh_deploy", MHDeploy, CIPDProd},
	{"mh_deploy-latest", MHDeploy, CIPDLatest},
	{"custom", Custom, CIPDProd},
	{"custom-latest", Custom, CIPDLatest},
	{"custom", InvalidTaskName, CIPDProd},
	{"custom-latest", InvalidTaskName, CIPDLatest},
}

func TestTaskNameToBuilderPerVersion(t *testing.T) {
	for i, c := range TaskNameToBuilderPerVersionCases {
		name := fmt.Sprintf("case: %d", i)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := TaskNameToBuilderNamePerVersion(c.taskName, c.version)
			if got != c.want {
				t.Errorf("received wrong value: wanted %q but got %q", c.want, got)
			}
		})
	}
}

func TestNormalizeTaskName(t *testing.T) {
	var cases = []struct {
		name      string
		in        []string
		out       TaskName
		expectErr bool
	}{
		{"audit-usb", []string{"verify-servo-usb-drive", "usb-drive", "audit-usb", "audit_usb"}, AuditUSB, false},
		{"audit storage", []string{"verify-dut-storage", "storage", "audit-storage", "audit_storage"}, AuditStorage, false},
		{"adit rpm", []string{"verify-rpm-config", "rpm config", "audit-rpm", "audit_rpm"}, AuditRPM, false},
		{"repair", []string{"repair", "recovery"}, Recovery, false},
		{"repair MH", []string{"mhrepair", "mh_recovery", "mh-recovery", "mh-repair", "mh_repair"}, MHRecovery, false},
		{"deep repair", []string{"deep-repair", "deep_repair"}, DeepRecovery, false},
		{"deploy", []string{"deploy"}, Deploy, false},
		{"deploy MH", []string{"mh-deploy", "mh_deploy", "mhdeploy"}, MHDeploy, false},
		{"dry-run", []string{"dry_run", "dry-run"}, DryRun, false},
		{"custom", []string{"custom"}, Custom, false},
		{"bad", []string{""}, InvalidTaskName, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, in := range c.in {
				tn, err := NormalizeTaskName(in)
				if c.expectErr && err == nil {
					t.Errorf("TestNormalizeTaskName: %q: unexpected pass when expecetd error", c.name)
				}
				if tn != c.out {
					t.Errorf("TestNormalizeTaskName: %q: wanted %q but got %q", c.name, c.out, tn)
				}
			}
		})
	}
}
