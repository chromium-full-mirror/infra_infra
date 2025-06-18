// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"slices"
	"testing"

	api "go.chromium.org/chromiumos/config/go/test/api"
)

// Blank pool name should cause default params to be used.
func TestConfigServiceBlankPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// mock request with all needed information. UFS lookup is not needed.
	in := &api.StartDevboardServiceRequest{
		ContainerName: "container",
	}

	s := devboardService{}
	s.configDevboardServiceFromRequest(ctx, in, "")

	if s.testbedType != "" {
		t.Errorf("want \"\", got %q", s.testbedType)
	}
	if s.gscSerial != "" {
		t.Errorf("want \"\", got %q", s.gscSerial)
	}
	if s.debuggerSerial != "" {
		t.Errorf("want \"\", got %q", s.debuggerSerial)
	}
	if s.port != DEFAULT_DEVBOARD_PORT {
		t.Errorf("want %s, got %s", DEFAULT_DEVBOARD_PORT, s.port)
	}
	if s.containerImage != DEFAULT_DEVBOARD_IMAGE {
		t.Errorf("want %s, got %s", DEFAULT_DEVBOARD_IMAGE, s.containerImage)
	}
}

// Pool name that is not known should cause default params to be used.
func TestConfigServiceUnknownPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// mock request with all needed information. UFS lookup is not needed.
	in := &api.StartDevboardServiceRequest{
		ContainerName: "container",
	}

	s := devboardService{}
	s.configDevboardServiceFromRequest(ctx, in, "somepool")

	if s.testbedType != "" {
		t.Errorf("want \"\", got %q", s.testbedType)
	}
	if s.gscSerial != "" {
		t.Errorf("want \"\", got %q", s.gscSerial)
	}
	if s.debuggerSerial != "" {
		t.Errorf("want \"\", got %q", s.debuggerSerial)
	}
	if s.port != DEFAULT_DEVBOARD_PORT {
		t.Errorf("want %s, got %s", DEFAULT_DEVBOARD_PORT, s.port)
	}
	if s.containerImage != DEFAULT_DEVBOARD_IMAGE {
		t.Errorf("want %s, got %s", DEFAULT_DEVBOARD_IMAGE, s.containerImage)
	}
}

// Default pool configs should be used.
func TestConfigServiceFoundPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// mock request with all needed information. UFS lookup is not needed.
	in := &api.StartDevboardServiceRequest{
		ContainerName: "container",
	}

	for pool := range devboardServicePools {
		if pool == "" {
			// Skip default pool, already covered by BlankPool test.
			continue
		}
		s := devboardService{}
		s.configDevboardServiceFromRequest(ctx, in, pool)

		poolReq := devboardServicePools[pool]
		if s.containerName != "container" {
			t.Errorf("want %s, got %s", "container", s.containerName)
		}
		if s.testbedType != poolReq.TestbedType {
			t.Errorf("want %s, got %s", poolReq.TestbedType, s.testbedType)
		}
		if s.gscSerial != poolReq.GscSerial {
			t.Errorf("want %s, got %s", poolReq.GscSerial, s.gscSerial)
		}
		if s.debuggerSerial != poolReq.DebuggerSerial {
			t.Errorf("want %s, got %s", poolReq.DebuggerSerial, s.debuggerSerial)
		}
		if s.port != poolReq.ServicePort {
			t.Errorf("want %s, got %s", poolReq.ServicePort, s.port)
		}
		if s.containerImage != poolReq.ContainerImage {
			t.Errorf("want %s, got %s", poolReq.ContainerImage, s.containerImage)
		}
		if s.debug != poolReq.Debug {
			t.Errorf("want %t, got %t", poolReq.Debug, s.debug)
		}
	}
}

// Overriding pool configs should work.
func TestConfigServiceOverridePool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// mock request with all needed information. UFS lookup is not needed.
	in := &api.StartDevboardServiceRequest{
		ContainerName:  "container",
		TestbedType:    "my-testbed-type",
		GscSerial:      "my-gsc-serial",
		DebuggerSerial: "my-debugger-serial",
		ServicePort:    "393939",
		ContainerImage: "myimage",
		Debug:          true,
	}

	for pool := range devboardServicePools {
		if pool == "" {
			// Skip default pool, already covered by BlankPool test.
			continue
		}
		s := devboardService{}
		s.configDevboardServiceFromRequest(ctx, in, pool)

		if s.containerName != "container" {
			t.Errorf("want %s, got %s", "container", s.containerName)
		}
		if s.testbedType != "my-testbed-type" {
			t.Errorf("want %s, got %s", "my-testbed-type", s.testbedType)
		}
		if s.gscSerial != "my-gsc-serial" {
			t.Errorf("want %s, got %s", "my-gsc-serial", s.gscSerial)
		}
		if s.debuggerSerial != "my-debugger-serial" {
			t.Errorf("want %s, got %s", "my-debugger-serial", s.debuggerSerial)
		}
		if s.port != "393939" {
			t.Errorf("want %s, got %s", "393939", s.port)
		}
		if s.containerImage != "myimage" {
			t.Errorf("want %s, got %s", "myimage", s.containerImage)
		}
		if s.debug != true {
			t.Errorf("want %t, got %t", true, s.debug)
		}
	}
}

func TestGenerateEnvVars(t *testing.T) {
	t.Parallel()

	s := devboardService{port: "393939", gscSerial: "1234-5678",
		debuggerSerial: "abcdefg", testbedType: "new-type"}

	vars := s.generateEnvVars()

	if len(vars) != 4 {
		t.Fatalf("want %d, got %d", 4, len(vars))
	}

	portVar := "DEVBOARDSVC_PORT=393939"
	if vars[0] != portVar {
		t.Errorf("want %s, got %s", portVar, vars[0])
	}

	gscVar := "GSC_SERIAL=1234-5678"
	if vars[1] != gscVar {
		t.Errorf("want %s, got %s", gscVar, vars[1])
	}

	debuggerVar := "DEBUGGER_SERIAL=abcdefg"
	if vars[2] != debuggerVar {
		t.Errorf("want %s, got %s", debuggerVar, vars[2])
	}

	testbedVar := "TESTBED=new-type"
	if vars[3] != testbedVar {
		t.Errorf("want %s, got %s", testbedVar, vars[3])
	}
}

func TestContainerCmd(t *testing.T) {
	t.Parallel()

	s := devboardService{}
	if !slices.Equal(devboardServiceStartCmd, s.containerCmd()) {
		t.Errorf("want %s, got %s", devboardServiceStartCmd, s.containerCmd())
	}

	s = devboardService{debug: true}
	if !slices.Equal(devboardServiceTailCmd, s.containerCmd()) {
		t.Errorf("want %s, got %s", devboardServiceTailCmd, s.containerCmd())
	}
}
