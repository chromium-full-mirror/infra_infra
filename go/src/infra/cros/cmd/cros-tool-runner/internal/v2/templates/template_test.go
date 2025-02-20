// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

// mockCmdExecutor mocks cmdExecutor for testing
type mockCmdExecutor struct {
	executeFunc func(ctx context.Context, cmd commands.Command) (string, string, error)
}

func (m *mockCmdExecutor) Execute(ctx context.Context, cmd commands.Command) (string, string, error) {
	return m.executeFunc(ctx, cmd)
}

func getMockCmdExecutorWithSuccess(port string) cmdExecutor {
	return &mockCmdExecutor{
		executeFunc: func(ctx context.Context, cmd commands.Command) (string, string, error) {
			return port + "\n", "", nil
		},
	}
}

func getMockCmdExecutorWithError(errMsg string) cmdExecutor {
	return &mockCmdExecutor{
		executeFunc: func(ctx context.Context, cmd commands.Command) (string, string, error) {
			return "", "", errors.New(errMsg)
		},
	}
}

func check(t *testing.T, a interface{}, b interface{}) {
	if !cmp.Equal(a, b) {
		t.Fatalf("%v should match %v", a, b)
	}
}

func TestDefaultDiscoverPort_errorPropagated(t *testing.T) {
	executor := getMockCmdExecutorWithError("something wrong when execute command")
	request := getCrosProvisionTemplateRequest("mynet")
	_, err := defaultDiscoverPort(executor, request)

	if err == nil {
		t.Errorf("Expect error")
	}
}

func TestDefaultDiscoverPort_hostNetwork_populateAllFields(t *testing.T) {
	expected := &api.Container_PortBinding{
		ContainerPort: int32(42),
		Protocol:      protocolTcp,
		HostIp:        localhostIp,
		HostPort:      int32(42),
	}
	executor := getMockCmdExecutorWithSuccess("42")
	request := getCrosProvisionTemplateRequest("host")
	binding, err := defaultDiscoverPort(executor, request)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	check(t, binding.String(), expected.String())
}
