// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package device

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// TestGetPools tests that GetPools passes an appropriately annotated name to the
func TestGetPools(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	c := &mockGetDeviceInfoClient{}
	fakeMachine := &models.MachineLSE{
		Lse: &models.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &models.ChromeOSMachineLSE{
				ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &models.ChromeOSDeviceLSE{
						Device: &models.ChromeOSDeviceLSE_Dut{
							Dut: &lab.DeviceUnderTest{
								Pools: []string{"aaaa"},
							},
						},
					},
				},
			},
		},
	}
	c.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
		ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
		Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
			ChromeOsDeviceData: &models.ChromeOSDeviceData{
				LabConfig: fakeMachine,
			},
		},
	}
	expectedPools := []string{"aaaa"}
	actualPools, err := GetPools(ctx, c, "a")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(expectedPools, actualPools); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestGetDeviceInfo tests the GetDeviceInfo function with various scenarios.
func TestGetDeviceInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testCases := []struct {
		name           string
		hostname       string
		mockSetup      func(*mockGetDeviceInfoClient)
		wantDeviceInfo *DeviceInfo
		wantErrMsg     string
	}{
		{
			name:       "client is nil",
			hostname:   "host1",
			mockSetup:  nil, // Client will be nil in the test
			wantErrMsg: "get device info: client is nil",
		},
		{
			name:     "GetDeviceData returns error",
			hostname: "host_err",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataErr = errors.New("ufs network error")
			},
			wantErrMsg: "get device info: fail to get device data for \"host_err\": ufs network error",
		},
		{
			name:     "Scheduling Unit - Happy Path",
			hostname: "su_host1",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT,
					Resource: &ufsAPI.GetDeviceDataResponse_SchedulingUnit{
						SchedulingUnit: &models.SchedulingUnit{
							Name:  "schedulingunits/su1-id",
							Pools: []string{"pool_su"},
						},
					},
				}
			},
			wantDeviceInfo: &DeviceInfo{Name: "schedulingunits/su1-id", ID: "schedulingunits/su1-id", Pools: []string{"pool_su"}},
		},
		{
			name:     "Scheduling Unit - Nil SU",
			hostname: "su_host_nil",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT,
					Resource: &ufsAPI.GetDeviceDataResponse_SchedulingUnit{
						SchedulingUnit: nil,
					},
				}
			},
			wantErrMsg: "get device info: scheduling unit \"su_host_nil\" is empty",
		},
		{
			name:     "ChromeOS Device - DUT - Happy Path",
			hostname: "dut_host1",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
						ChromeOsDeviceData: &models.ChromeOSDeviceData{
							LabConfig: &models.MachineLSE{
								Name: "lse/dut1",
								Lse: &models.MachineLSE_ChromeosMachineLse{
									ChromeosMachineLse: &models.ChromeOSMachineLSE{
										ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
											DeviceLse: &models.ChromeOSDeviceLSE{Device: &models.ChromeOSDeviceLSE_Dut{Dut: &lab.DeviceUnderTest{Pools: []string{"pool_dut"}}}},
										},
									},
								},
							},
							Machine: &models.Machine{Name: "machines/machine_dut1", SerialNumber: "DUTSN001", Device: &models.Machine_ChromeosMachine{ChromeosMachine: &models.ChromeOSMachine{BuildTarget: "board_dut", Model: "model_dut"}}},
						},
					},
				}
			},
			wantDeviceInfo: &DeviceInfo{Name: "lse/dut1", ID: "machines/machine_dut1", Board: "board_dut", Model: "model_dut", Pools: []string{"pool_dut"}},
		},
		{
			name:     "ChromeOS Device - Labstation - Happy Path",
			hostname: "labstation_host1",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
						ChromeOsDeviceData: &models.ChromeOSDeviceData{
							LabConfig: &models.MachineLSE{
								Name: "lse/labstation1",
								Lse: &models.MachineLSE_ChromeosMachineLse{
									ChromeosMachineLse: &models.ChromeOSMachineLSE{
										ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
											DeviceLse: &models.ChromeOSDeviceLSE{Device: &models.ChromeOSDeviceLSE_Labstation{Labstation: &lab.Labstation{Pools: []string{"pool_lab"}}}},
										},
									},
								},
							},
							Machine: &models.Machine{Name: "machines/machine_lab1", Device: &models.Machine_ChromeosMachine{ChromeosMachine: &models.ChromeOSMachine{BuildTarget: "board_lab", Model: "model_lab"}}},
						},
					},
				}
			},
			wantDeviceInfo: &DeviceInfo{Name: "lse/labstation1", ID: "machines/machine_lab1", Board: "board_lab", Model: "model_lab", Pools: []string{"pool_lab"}},
		},
		{
			name:     "ChromeOS Device - DevBoard (Andreiboard) - Happy Path",
			hostname: "dev_andrei_host1",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
						ChromeOsDeviceData: &models.ChromeOSDeviceData{
							LabConfig: &models.MachineLSE{
								Name: "lse/dev_andrei1",
								Lse: &models.MachineLSE_ChromeosMachineLse{
									ChromeosMachineLse: &models.ChromeOSMachineLSE{
										ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
											DeviceLse: &models.ChromeOSDeviceLSE{Device: &models.ChromeOSDeviceLSE_Devboard{Devboard: &lab.Devboard{Pools: []string{"pool_dev"}}}},
										},
									},
								},
							},
							Machine: &models.Machine{Name: "machines/machine_dev_andrei1", Device: &models.Machine_Devboard{Devboard: &models.Devboard{Board: &models.Devboard_Andreiboard{Andreiboard: &models.Andreiboard{}}}}},
						},
					},
				}
			},
			wantDeviceInfo: &DeviceInfo{Name: "lse/dev_andrei1", ID: "machines/machine_dev_andrei1", Board: "andreiboard", Model: "andreiboard", Pools: []string{"pool_dev"}},
		},
		// Similar tests for Icetower and Dragonclaw can be added here.
		{
			name:     "ChromeOS Device - Nil ChromeOSDeviceData",
			hostname: "cros_nil_host",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
						ChromeOsDeviceData: nil,
					},
				}
			},
			wantErrMsg: "get device info: chromeos \"cros_nil_host\" is empty",
		},
		{
			name:     "ChromeOS Device - No DUT/Labstation/Devboard",
			hostname: "cros_unsupported_type_host",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
						ChromeOsDeviceData: &models.ChromeOSDeviceData{ // Valid data, but DeviceLse has no specific type
							LabConfig: &models.MachineLSE{Name: "lse/unknown", Lse: &models.MachineLSE_ChromeosMachineLse{ChromeosMachineLse: &models.ChromeOSMachineLSE{ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{DeviceLse: &models.ChromeOSDeviceLSE{ /* No DUT, Labstation, or Devboard */ }}}}},
							Machine:   &models.Machine{Name: "machines/unknown"},
						},
					},
				}
			},
			wantErrMsg: "get device info: type of chromeos \"cros_unsupported_type_host\" is not supported",
		},
		{
			name:     "Attached Device - Happy Path",
			hostname: "attached_host1",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_AttachedDeviceData{
						AttachedDeviceData: &ufsAPI.AttachedDeviceData{
							Machine:   &models.Machine{Name: "machines/attached1", Device: &models.Machine_AttachedDevice{AttachedDevice: &models.AttachedDevice{BuildTarget: "board_attached", Model: "model_attached"}}},
							LabConfig: &models.MachineLSE{Hostname: "lse/attached1"},
						},
					},
				}
			},
			wantDeviceInfo: &DeviceInfo{Name: "lse/attached1", ID: "machines/attached1", Board: "board_attached", Model: "model_attached"},
		},
		{
			name:     "Attached Device - Nil AttachedDeviceData",
			hostname: "attached_nil_host",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE,
					Resource: &ufsAPI.GetDeviceDataResponse_AttachedDeviceData{
						AttachedDeviceData: nil,
					},
				}
			},
			wantErrMsg: "get device info: attached device \"attached_nil_host\" is empty",
		},
		{
			name:     "Unsupported Resource Type",
			hostname: "unsupported_host",
			mockSetup: func(mc *mockGetDeviceInfoClient) {
				mc.GetDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
					ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_UNSPECIFIED,
				}
			},
			wantErrMsg: "get device info: unsupported device type \"unsupported_host\"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var client GetPoolsClient
			mockClient := &mockGetDeviceInfoClient{}
			if tc.mockSetup != nil {
				tc.mockSetup(mockClient)
				client = mockClient
			} else if tc.name == "client is nil" {
				client = nil
			} else {
				// Default mock if no specific setup, though most tests will have one.
				client = mockClient
			}

			gotDeviceInfo, err := GetDeviceInfo(ctx, client, tc.hostname)

			if tc.wantErrMsg != "" {
				if err == nil {
					t.Errorf("GetDeviceInfo() returned no error, want error containing %q", tc.wantErrMsg)
				} else if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("GetDeviceInfo() error = %v, want error containing %q", err, tc.wantErrMsg)
				}
				if gotDeviceInfo != nil {
					t.Errorf("GetDeviceInfo() returned deviceInfo %v, want nil when error is expected", gotDeviceInfo)
				}
			} else {
				if err != nil {
					t.Errorf("GetDeviceInfo() returned error %v, want no error", err)
				}
				if diff := cmp.Diff(tc.wantDeviceInfo, gotDeviceInfo); diff != "" {
					t.Errorf("GetDeviceInfo() deviceInfo mismatch (-want +got):\n%s", diff)
				}
				if mockClient.CapturedHostname != tc.hostname && client != nil {
					t.Errorf("GetDeviceData was called with hostname %q, want %q", mockClient.CapturedHostname, tc.hostname)
				}
			}
		})
	}
}

// mockGetDeviceInfoClient is a mock for the GetPoolsClient interface,
// specifically tailored for testing GetDeviceInfo.
type mockGetDeviceInfoClient struct {
	// Response to return from GetDeviceData.
	GetDeviceDataResponse *ufsAPI.GetDeviceDataResponse
	// Error to return from GetDeviceData.
	GetDeviceDataErr error

	// To satisfy the GetPoolsClient interface, not directly used by GetDeviceInfo.
	GetMachineLSEResponse *models.MachineLSE
	GetMachineLSEErr      error

	// Records the hostname passed to GetDeviceData.
	CapturedHostname string
}

func (m *mockGetDeviceInfoClient) GetMachineLSE(ctx context.Context, in *ufsAPI.GetMachineLSERequest, opts ...grpc.CallOption) (*models.MachineLSE, error) {
	return m.GetMachineLSEResponse, m.GetMachineLSEErr
}

func (m *mockGetDeviceInfoClient) GetDeviceData(ctx context.Context, in *ufsAPI.GetDeviceDataRequest, opts ...grpc.CallOption) (*ufsAPI.GetDeviceDataResponse, error) {
	m.CapturedHostname = in.GetHostname()
	if m.GetDeviceDataErr != nil {
		return nil, m.GetDeviceDataErr
	}
	return m.GetDeviceDataResponse, nil
}
