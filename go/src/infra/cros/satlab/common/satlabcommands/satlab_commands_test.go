// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package satlabcommands

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// TestGetHostIPShouldSuccess test `GetHostIP` function.
func TestGetHostIPShouldSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectedIP := "127.0.0.1"
	commandExecutor := &executor.FakeCommander{CmdOutput: expectedIP}

	res, err := GetHostIP(ctx, commandExecutor)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if res != expectedIP {
		t.Errorf("Expected %v, got %v", expectedIP, res)
	}
}

// TestGetHostIPShouldFail test `GetHostIP` function.
func TestGetHostIPShouldFail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectedError := errors.New("exec command failed")
	commandExecutor := &executor.FakeCommander{Err: expectedError}

	res, err := GetHostIP(ctx, commandExecutor)

	// Assert
	if err == nil {
		t.Errorf("Should return error, but got no error")
	}

	if res != "" {
		t.Errorf("Expected %v, got %v", "", res)
	}
}

func macAddressCommandHelper(hostname, macAddress, cmdOutput string) *executor.FakeCommander {
	return &executor.FakeCommander{
		FakeFn: func(in *exec.Cmd) ([]byte, error) {
			cmd := strings.Join(in.Args, " ")
			if in.Path == paths.GetHostIPScript {
				return []byte(hostname), nil
			} else if cmd == fmt.Sprintf("%s exec dhcp cat %s", paths.DockerPath, fmt.Sprintf(paths.NetInfoPathTemplate, "eth0")) {
				return []byte(macAddress), nil
			} else if cmd == fmt.Sprint(fmt.Sprintf("%s exec dhcp ip route show", paths.DockerPath)) {
				return []byte(
					fmt.Sprintf("%v/24 dev eth0 scope link  src %v", hostname, hostname),
				), nil
			} else if in.Path == paths.Grep {
				return []byte(hostname), nil
			}
			return nil, fmt.Errorf("handle command: %v", in.Path)
		},
		CmdOutput: cmdOutput,
	}

}

// TestGetMacAddressShouldSuccess test `GetMacAddress` function.
func TestGetMacAddressShouldSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// fake data
	hostname := "127.0.0.1"
	expectedMacAddress := "aa:bb:cc:dd:ee:ff"
	commandExecutor := macAddressCommandHelper(
		hostname,
		expectedMacAddress,
		fmt.Sprintf("%v/24 dev eth0 scope link  src %v", hostname, hostname),
	)

	res, err := GetMacAddress(ctx, commandExecutor)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if res != expectedMacAddress {
		t.Errorf("Expected %v, got %v", expectedMacAddress, res)
	}
}

// TestGetMacAddressShouldFailWhenCommandExecutorFailed test `GetMacAddress` function.
func TestGetMacAddressShouldFailWhenCommandExecutorFailed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectedError := errors.New("exec command failed")
	commandExecutor := &executor.FakeCommander{Err: expectedError}

	res, err := GetMacAddress(ctx, commandExecutor)

	// Assert
	if err == nil {
		t.Errorf("Should return error, but got no error")
	}

	if res != "" {
		t.Errorf("Expected %v, got %v", "", res)
	}
}

// TestGetMacAddressShouldFailWhenGetNICNameFailed test `GetMacAddress` function.
func TestGetMacAddressShouldFailWhenGetNICNameFailed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// fake data
	hostname := "127.0.0.1"
	macAddress := "aa:bb:cc:dd:ee:ff"
	commandExecutor := macAddressCommandHelper(hostname, macAddress, "")

	res, err := GetMacAddress(ctx, commandExecutor)

	// Assert
	if err == nil {
		t.Errorf("Should return error, but got no error")
	}

	if res != "" {
		t.Errorf("Expected %v, got %v", "", res)
	}
}

// TestGetSatlabStartTimeShouldSuccess test `GetSatlabStartTime` function.
func TestGetSatlabStartTimeShouldSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	timeObjForTest := time.Now()
	commandExecutor := &executor.FakeCommander{CmdOutput: fmt.Sprintf("'%v'", timeObjForTest.Format(time.RFC3339Nano))}

	res, err := GetSatlabStartTime(ctx, commandExecutor)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	expectedStartTime := timestamppb.New(timeObjForTest)
	if diff := cmp.Diff(expectedStartTime, res, cmpopts.IgnoreUnexported(timestamp.Timestamp{})); diff != "" {
		t.Errorf("Expected %v, got %v", expectedStartTime, res)
	}
}

// TestGetSatlabStartTimeShouldFailWhenCommandExecutorFailed test `GetSatlabStartTime` function.
func TestGetSatlabStartTimeShouldFailWhenCommandExecutorFailed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectedError := errors.New("exec command failed")
	commandExecutor := &executor.FakeCommander{Err: expectedError}

	res, err := GetSatlabStartTime(ctx, commandExecutor)

	// Assert
	if err == nil {
		t.Errorf("Should return error, but got no error")
	}

	if res != nil {
		t.Errorf("Expected %v, got %v", nil, res)
	}
}

// TestGetSatlabStartTimeShouldFailCommandOutputIsEmpty test `GetSatlabStartTime` function.
func TestGetSatlabStartTimeShouldFailCommandOutputIsEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	commandExecutor := &executor.FakeCommander{CmdOutput: ""}

	res, err := GetSatlabStartTime(ctx, commandExecutor)

	// Assert
	if err == nil {
		t.Errorf("Expected error")
	}

	if res != nil {
		t.Errorf("Expected %v, got %v", nil, res)
	}
}

func Test_GetSatlabVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// fake some data
	f := &executor.FakeCommander{
		FakeFn: func(_ *exec.Cmd) ([]byte, error) {
			return []byte(`LABEL=beta
SSH_PORT=22
COMMON_CORE_LABEL=R-2.24.0
BUILD_VERSION=R-4.2.3`), nil
		},
	}

	res, err := GetSatlabVersion(ctx, f)

	if err != nil {
		t.Errorf("unexpected error: %v\n", err)
	}

	expected := "R-4.2.3"

	if res != expected {
		t.Errorf("unexpected result, expected: %v, got %v\n", expected, res)
	}
}

func TestIsUpdateAvailable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name          string
		cmdOutput     string
		cmdError      error
		expectedValue bool
		expectedError bool
		errorMsg      string
	}{
		{
			name:          "Update Available",
			cmdOutput:     "true\n",
			cmdError:      nil,
			expectedValue: true,
			expectedError: false,
		},
		{
			name:          "Update Not Available",
			cmdOutput:     "false\n",
			cmdError:      nil,
			expectedValue: false,
			expectedError: false,
		},
		{
			name:          "Command Execution Error",
			cmdOutput:     "",
			cmdError:      errors.New("command failed"),
			expectedValue: false,
			expectedError: true,
			errorMsg:      "get is update available: command failed",
		},
		{
			name:          "Invalid Boolean Output",
			cmdOutput:     "maybe\n",
			cmdError:      nil,
			expectedValue: false,
			expectedError: true,
			errorMsg:      `strconv.ParseBool: parsing "maybe": invalid syntax`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandExecutor := &executor.FakeCommander{CmdOutput: tt.cmdOutput, Err: tt.cmdError}
			actualValue, err := IsUpdateAvailable(ctx, commandExecutor)

			if actualValue != tt.expectedValue {
				t.Errorf("IsUpdateAvailable() value = %v, want %v", actualValue, tt.expectedValue)
			}

			if (err != nil) != tt.expectedError {
				t.Errorf("IsUpdateAvailable() error = %v, expectedError %v", err, tt.expectedError)
			} else if err != nil && err.Error() != tt.errorMsg {
				t.Errorf("IsUpdateAvailable() error message = %q, want %q", err.Error(), tt.errorMsg)
			}
		})
	}
}
