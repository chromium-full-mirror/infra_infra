// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dutidentity

import (
	"testing"

	"go.chromium.org/infra/cros/cmd/cft/dut/cros-dut/dutssh"
)

type FakeCmdExecutor struct {
	CmdResults map[string]*dutssh.CmdResult
}

func (e FakeCmdExecutor) RunCmd(cmd string) (*dutssh.CmdResult, error) {
	result, exists := e.CmdResults[cmd]
	if exists {
		return result, nil
	}
	// cros_config returns 1 for non-existent properties, so mimic that
	return cmdResult("", 1), nil
}

func cmdResult(stdout string, returnCode int32) *dutssh.CmdResult {
	return &dutssh.CmdResult{
		StdOut:     stdout,
		ReturnCode: returnCode,
	}
}

func TestNoFirmwareNameErrors(t *testing.T) {
	fakeCmdExecutor := FakeCmdExecutor{
		map[string]*dutssh.CmdResult{},
	}

	errorMessage := DetectDeviceConfigID(fakeCmdExecutor).GetFailure().ErrorMessage
	if len(errorMessage) == 0 {
		t.Fatalf("Expected failure for missing fw name")
	}
}

func TestInvalidSkuFormatErrors(t *testing.T) {
	fakeFwName := "Fake"
	invalidSku := "NaN"
	fakeCmdExecutor := FakeCmdExecutor{
		map[string]*dutssh.CmdResult{
			"cros_config /identity smbios-name-match": cmdResult(fakeFwName, 0),
			"cros_config /identity sku-id":            cmdResult(invalidSku, 0),
		},
	}

	errorMessage := DetectDeviceConfigID(fakeCmdExecutor).GetFailure().ErrorMessage
	if len(errorMessage) == 0 {
		t.Fatalf("Expected failure for invalid sku format")
	}
}
