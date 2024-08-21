// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cron

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/impl/memory"

	"infra/cros/botsregulator/internal/clients"
	"infra/cros/botsregulator/internal/regulator"
	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

func TestMigrate(t *testing.T) {
	t.Run("Happy Path", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		files := map[string]string{
			"migration.cfg": `
			config {
				min_cloudbots_percentage: 30
				min_low_risk_models_percentage: 25
				min_large_memory_percentage: 20
				low_risk_models: "model-1"
				exclude_duts: "dut-2"
				exclude_pools: "pool-2"
				overrides {
				  board: "board-2"
				  model: "model-2"
				  percentage: 0
				}
				large_memory_overrides {
				  board: "board-2"
				  model: "model-2"
				  percentage: 0
				}
			  }
			`,
		}
		configSets := map[config.Set]memory.Files{"services/${appid}": files}

		mockClient := memory.New(configSets)
		mockUFS := clients.NewMockUFSClient(mockCtrl)
		ctx := context.Background()
		ctx = context.WithValue(ctx, clients.MockConfigClientKey, mockClient)
		ctx = context.WithValue(ctx, clients.MockUFSClientKey, mockUFS)

		opts := &regulator.RegulatorOptions{
			BPI:       "bpi.endpoint",
			UFS:       "ufs.enpoint",
			Hive:      "cloudbots",
			CfID:      "cloudbots-dev",
			Namespace: "os",
			Swarming:  "swarming.endpoint",
		}

		ctxWithNS := clients.SetUFSNamespace(ctx, "os")
		gomock.InOrder(
			mockUFS.EXPECT().BatchListMachines(ctxWithNS, []string{"zone=ZONE_SFO36_OS"}, 0, false, false).Return([]protoadapt.MessageV1{
				&ufspb.Machine{
					Name: "machines/machine-1",
					Device: &ufspb.Machine_ChromeosMachine{
						ChromeosMachine: &ufspb.ChromeOSMachine{
							BuildTarget: "board-1",
							Model:       "model-1",
						},
					}},
				&ufspb.Machine{
					Name: "machines/machine-2",
					Device: &ufspb.Machine_ChromeosMachine{
						ChromeosMachine: &ufspb.ChromeOSMachine{
							BuildTarget: "board-1",
							Model:       "model-2",
						},
					},
				},
				&ufspb.Machine{
					Name: "machines/machine-3",
					Device: &ufspb.Machine_ChromeosMachine{
						ChromeosMachine: &ufspb.ChromeOSMachine{
							BuildTarget: "board-1",
							Model:       "model-2",
						},
					},
				},
				&ufspb.Machine{
					Name: "machines/machine-4",
					Device: &ufspb.Machine_ChromeosMachine{
						ChromeosMachine: &ufspb.ChromeOSMachine{
							BuildTarget: "board-2",
							Model:       "model-2",
						},
					},
				},
				&ufspb.Machine{
					Name: "machines/machine-5",
					Device: &ufspb.Machine_ChromeosMachine{
						ChromeosMachine: &ufspb.ChromeOSMachine{
							BuildTarget: "board-2",
							Model:       "model-3",
						},
					},
				},
			}, nil),
			mockUFS.EXPECT().BatchListMachineLSEs(ctxWithNS, []string{"zone=ZONE_SFO36_OS"}, 0, false, false).Return([]protoadapt.MessageV1{
				&ufspb.MachineLSE{
					Name: "machineLSEs/dut-1",
					Machines: []string{
						"machine-1",
					},
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hive: "e",
										},
									},
								},
							},
						},
					},
				},
				&ufspb.MachineLSE{
					Name: "machineLSEs/dut-2",
					Machines: []string{
						"machine-2",
					},
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hive: "cloudbots",
										},
									},
								},
							},
						},
					}},
				&ufspb.MachineLSE{
					Name: "machineLSEs/dut-3",
					Machines: []string{
						"machine-3",
					},
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hive: "cloudbots",
										},
									},
								},
							},
						},
					}},
				&ufspb.MachineLSE{
					Name: "machineLSEs/dut-4",
					Machines: []string{
						"machine-4",
					},
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hive: "e",
										},
									},
								},
							},
						},
					}},
				&ufspb.MachineLSE{
					Name: "machineLSEs/dut-5",
					Machines: []string{
						"machine-5",
					},
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hive: "cloudbots",
											Pools: []string{
												"pool-2",
											},
										},
									},
								},
							},
						},
					}},
			}, nil),
			mockUFS.EXPECT().UpdateMachineLSE(ctxWithNS, &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: &ufspb.MachineLSE{
					Name:     "machineLSEs/dut-1",
					Hostname: "dut-1",
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hostname: "dut-1",
											Hive:     "cloudbots-large",
											Peripherals: &chromeosLab.Peripherals{
												Chameleon:     &chromeosLab.Chameleon{},
												Servo:         &chromeosLab.Servo{},
												Rpm:           &chromeosLab.OSRPM{},
												Audio:         &chromeosLab.Audio{},
												Wifi:          &chromeosLab.Wifi{},
												Touch:         &chromeosLab.Touch{},
												CameraboxInfo: &chromeosLab.Camerabox{},
												Dolos:         &chromeosLab.Dolos{},
											},
										},
									},
								},
							},
						},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"dut.hive"},
				},
			}),
			mockUFS.EXPECT().UpdateMachineLSE(ctxWithNS, &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: &ufspb.MachineLSE{
					Name:     "machineLSEs/dut-3",
					Hostname: "dut-3",
					Lse: &ufspb.MachineLSE_ChromeosMachineLse{
						ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
							ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
								DeviceLse: &ufspb.ChromeOSDeviceLSE{
									Device: &ufspb.ChromeOSDeviceLSE_Dut{
										Dut: &chromeosLab.DeviceUnderTest{
											Hostname: "dut-3",
											Hive:     "cloudbots-large",
											Peripherals: &chromeosLab.Peripherals{
												Chameleon:     &chromeosLab.Chameleon{},
												Servo:         &chromeosLab.Servo{},
												Rpm:           &chromeosLab.OSRPM{},
												Audio:         &chromeosLab.Audio{},
												Wifi:          &chromeosLab.Wifi{},
												Touch:         &chromeosLab.Touch{},
												CameraboxInfo: &chromeosLab.Camerabox{},
												Dolos:         &chromeosLab.Dolos{},
											},
										},
									},
								},
							},
						},
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"dut.hive"},
				},
			}),
		)

		err := Migrate(ctx, opts, &LastSeenConfig{})
		if err != nil {
			t.Fatalf("should not error: %v", err)
		}
	})
}
