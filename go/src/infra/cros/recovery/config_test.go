// Copyright 2021 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"testing"

	"go.chromium.org/infra/cros/recovery/config"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// Testing plans here as that required access to execs and we try to avoid extra linking.

// verifyConfig verifies that configuration can be parsed and contains all execs present in library.
// Fail if:
// 1) Cannot parse by loader,
// 2) Missing dependency, condition or recovery action used in the actions,
// 3) Used unknown exec function.
func verifyConfig(name string, t *testing.T, c *config.Configuration) {
	ctx := context.Background()
	p, err := config.Load(ctx, mustCreateConfigJSON(c), execs.Exist)
	if err != nil {
		t.Errorf("%q :unexpected failed with error: %s", name, err)
	}
	if p == nil {
		t.Errorf("%q default config is empty", name)
	}
}

// TestLabstationRepairConfig verifies the labstation repair configuration.
func TestLabstationRepairConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("labstation-repair", t, config.LabstationRepairConfig())
}

// TestLabstationDeployConfig verifies the labstation deploy configuration.
func TestLabstationDeployConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("labstation-deploy", t, config.LabstationDeployConfig())
}

// TestCrosRepairConfig verifies the cros repair configuration.
func TestCrosRepairConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("dut-repair", t, config.CrosRepairConfig())
}

// TestMHRepairConfig verifies the ATE repair configuration.
func TestMHRepairConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("mh-repair", t, config.MHRepairConfig())
}

// TestCrosDeployConfig verifies the cros deploy configuration.
func TestCrosDeployConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("dut-deploy", t, config.CrosDeployConfig())
}

// TestMHDeployConfig verifies the MH deploy configuration.
func TestMHDeployConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("mh-deploy", t, config.MHDeployConfig())
}

// TestAndroidRepairConfig verifies the android repair configuration.
func TestAndroidRepairConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("android-repair", t, config.AndroidRepairConfig())
}

// TestAndroidDeployConfig verifies the android deploy configuration.
func TestAndroidDeployConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("android-deploy", t, config.AndroidDeployConfig())
}

// TestBtpeerProvisionConfig verifies the btpeer provision config.
func TestBtpeerProvisionConfig(t *testing.T) {
	t.Parallel()
	verifyConfig("btpeer-provision", t, config.ProvisionBtpeerConfig("test_url"))
}

func mustCreateConfigJSON(c *config.Configuration) io.Reader {
	b, err := json.Marshal(c)
	if err != nil {
		log.Fatalf("Failed to create JSON config: %v", err)
	}
	return bytes.NewBuffer(b)
}
