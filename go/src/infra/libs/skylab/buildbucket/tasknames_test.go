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
