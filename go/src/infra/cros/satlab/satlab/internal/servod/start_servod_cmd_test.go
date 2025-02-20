// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package servod

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/satlab/common/site"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsCros "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// FakeUFSClient is a mockUFS client that returns hardcoded data. It can also return bad/non-existent data
type FakeUFSClient struct {
	badData bool
}

// GetMachine returns a hardcoded machine.
// If the fake UFS client calling it was designated to have bad data, instead returns machine with all default fields.
func (f *FakeUFSClient) GetMachine(context.Context, *ufsApi.GetMachineRequest, ...grpc.CallOption) (*ufsModels.Machine, error) {
	if f.badData {
		return &ufsModels.Machine{}, nil
	}

	chromeOsMachine := ufsModels.ChromeOSMachine{
		BuildTarget: "ufsBoard",
		Model:       "ufsModel",
	}

	m := ufsModels.Machine_ChromeosMachine{ChromeosMachine: &chromeOsMachine}

	return &ufsModels.Machine{
		Device: &m,
	}, nil
}

func (c *FakeUFSClient) GetMachineLSE(ctx context.Context, req *ufsApi.GetMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error) {
	if c.badData {
		return &ufsModels.MachineLSE{}, nil
	}

	servo := ufsCros.Servo{
		ServoSerial:         "ufsSerial",
		DockerContainerName: "ufsDockerContainerName",
	}

	return &ufsModels.MachineLSE{
		Machines: []string{"ufsAssetId"},
		Lse: &ufsModels.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufsModels.ChromeOSMachineLSE{
				ChromeosLse: &ufsModels.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufsModels.ChromeOSDeviceLSE{
						Device: &ufsModels.ChromeOSDeviceLSE_Dut{
							Dut: &ufsCros.DeviceUnderTest{
								Peripherals: &ufsCros.Peripherals{
									Servo: &servo,
								},
							},
						},
					},
				},
			},
		},
	}, nil
}
func (c *FakeUFSClient) UpdateMachineLSE(ctx context.Context, req *ufsApi.UpdateMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error) {
	return nil, nil
}

// TestStartServodCmd tests the innerRun function of our command with fake UFS and docker clients, comparing the args we use to actually launch docker w/what is expected
// It tests input entirely, partially, and not at all user given
func TestStartServodCmd(t *testing.T) {
	t.Parallel()

	type test struct {
		inputCommand          *startServodRun
		expectedContainerName string
		expectedContainerArgs docker.ContainerArgs
	}

	tests := []test{
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: false},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", true, ufsCros.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board"},
			"ufsDockerContainerName",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "ufsModel", "ufsSerial", true, ufsCros.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""}),
		},
		{
			&startServodRun{host: "eli"},
			"ufsDockerContainerName",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "ufsBoard", "ufsModel", "ufsSerial", true, ufsCros.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: true},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", false, ufsCros.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: true},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", false, ufsCros.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: false, servoSetup: "regular"},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", true, ufsCros.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: false, servoSetup: "dual_v4"},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", true, ufsCros.ServoSetupType_SERVO_SETUP_DUAL_V4, "", false, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: false, servoSetup: "dual_v4", useRecMode: true},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", true, ufsCros.ServoSetupType_SERVO_SETUP_DUAL_V4, "", true, ""}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: false, servoSetup: "dual_v4", dockerTag: "beta"},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", true, ufsCros.ServoSetupType_SERVO_SETUP_DUAL_V4, "", false, "beta"}),
		},
		{
			&startServodRun{host: "eli", board: "board", model: "model", servoSerial: "serial", servodContainerName: "docker_servod", noServodProcess: false, servoSetup: "dual_v4", useRecMode: true, dockerTag: "beta"},
			"docker_servod",
			*buildServodContainerArgs(ServodContainerOptions{"docker_servod", "board", "model", "serial", true, ufsCros.ServoSetupType_SERVO_SETUP_DUAL_V4, "", true, "beta"}),
		},
	}

	for _, tc := range tests {
		fakeDockerClient := NewFakeDockerClient(false, false)

		err := tc.inputCommand.runOrchestratedCommand(context.Background(), &fakeDockerClient, &FakeUFSClient{})
		if err != nil {
			t.Errorf("Unexpected err: %v", err)
		}

		if diff := cmp.Diff(fakeDockerClient.runningContainers[tc.expectedContainerName], tc.expectedContainerArgs); diff != "" {
			t.Errorf("Docker container launched with unexpected diff: %s", diff)
		}
	}
}

// TestStartServodCmdFailedFetch tests behavior when we get data from UFS but it is not complete
func TestStartServodCmdFailedFetch(t *testing.T) {
	t.Parallel()

	fakeDockerClient := NewFakeDockerClient(false, false)
	fakeUFSClient := FakeUFSClient{badData: true}

	err := (&startServodRun{host: "eli"}).runOrchestratedCommand(context.Background(), &fakeDockerClient, &fakeUFSClient)

	if err == nil {
		t.Errorf("Expected command to error")
	}
}

// fakeHostBoxGetter is a function of type `HostBoxIdentifierGetter` injected to make sure we use it correctly
func fakeHostBoxGetter() (string, error) {
	return "1234", nil
}

// TestValidate tests behavior of validate under different user inputs
func TestValidate(t *testing.T) {
	t.Parallel()

	type test struct {
		inputCommand   *startServodRun
		inputArgs      []string
		expectedOutput *startServodRun
		expectedErr    bool
	}

	tests := []test{
		{&startServodRun{host: ""}, []string{}, nil, true},
		{&startServodRun{host: "eli"}, []string{}, &startServodRun{host: "satlab-1234-eli"}, false},
		{&startServodRun{host: "eli", commonFlags: site.CommonFlags{SatlabID: "5678"}}, []string{}, &startServodRun{host: "satlab-5678-eli"}, false},
		{&startServodRun{host: "satlab-1234-eli"}, []string{}, &startServodRun{host: "satlab-1234-eli"}, false},
		{&startServodRun{host: "satlab-1234-eli"}, []string{"bad"}, nil, true},
	}

	for _, tc := range tests {
		err := tc.inputCommand.validate("1234", tc.inputArgs)
		if tc.expectedErr != (err != nil) {
			t.Errorf("Expected tc to error: %t, but was the opposite for input %+v", tc.expectedErr, tc)
		}

		if !tc.expectedErr {
			if diff := cmp.Diff(tc.inputCommand.host, tc.expectedOutput.host); diff != "" {
				t.Errorf("unexpected diff: %s for input %+v", diff, tc.inputCommand)
			}
		}
	}
}
