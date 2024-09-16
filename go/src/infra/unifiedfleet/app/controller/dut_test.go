// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"testing"
	"time"

	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"

	"go.chromium.org/chromiumos/config/go/api"
	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsmanufacturing "infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/model/state"
)

func mockDUT(hostname, machine, servoHost, servoSerial, rpm, rpmOutlet string, servoPort int32, pools []string, dockerContainer string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:     hostname,
		Hostname: hostname,
		Machines: []string{machine},
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Dut{
							Dut: &chromeosLab.DeviceUnderTest{
								Hostname: hostname,
								Peripherals: &chromeosLab.Peripherals{
									Servo: &chromeosLab.Servo{
										ServoHostname:       servoHost,
										ServoPort:           servoPort,
										ServoSerial:         servoSerial,
										DockerContainerName: dockerContainer,
									},
									Rpm: &chromeosLab.OSRPM{
										PowerunitName:   rpm,
										PowerunitOutlet: rpmOutlet,
										PowerunitType:   chromeosLab.OSRPM_TYPE_UNKNOWN,
									},
								},
								Pools: pools,
							},
						},
					},
				},
			},
		},
		// Output only fields. Set defaults for use in comparision.
		Zone:          "ZONE_CHROMEOS6",
		Realm:         "@internal:ufs/os-atl",
		ResourceState: ufspb.State_STATE_REGISTERED,
	}
}

func addMockDolosToDUT(machinelse *ufspb.MachineLSE, dolosHost, dolosSerialCable, rpm, rpmOutlet string, DolosSerialUsb string) {
	machinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Dolos = &chromeosLab.Dolos{
		Hostname:    dolosHost,
		SerialCable: dolosSerialCable,
		SerialUsb:   DolosSerialUsb,
		Rpm: &chromeosLab.OSRPM{
			PowerunitName:   rpm,
			PowerunitOutlet: rpmOutlet,
		},
	}
}

func mockLabstation(hostname, machine string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:     hostname,
		Hostname: hostname,
		Machines: []string{machine},
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Labstation{
							Labstation: &chromeosLab.Labstation{
								Hostname: hostname,
							},
						},
					},
				},
			},
		},
	}
}

func mockFieldMask(opts ...string) *field_mask.FieldMask {
	return &field_mask.FieldMask{
		Paths: opts,
	}
}

func createValidDUTWithLabstation(ctx context.Context, t *ftt.Test, dutName, dutMachine, labstationName, labstationMachine string) error {
	machine1 := &ufspb.Machine{
		Name: labstationMachine,
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS6,
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "test",
				Model:       "test",
			},
		},
	}
	machine2 := &ufspb.Machine{
		Name: dutMachine,
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS6,
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "test",
				Model:       "test",
			},
		},
	}
	_, err := registration.CreateMachine(ctx, machine1)
	if err != nil {
		return err
	}
	_, err = registration.CreateMachine(ctx, machine2)
	if err != nil {
		return err
	}
	labstation1 := mockLabstation(labstationName, labstationMachine)
	_, err = CreateLabstation(ctx, labstation1)
	if err != nil {
		return err
	}
	dut1 := mockDUT(dutName, dutMachine, labstationName, "serial-1", dutName+"-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
	_, err = CreateDUT(ctx, dut1)
	if err != nil {
		return err
	}
	changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/"+dutName)
	if err != nil {
		return err
	}
	assert.Loosely(t, changes, should.HaveLength(1))
	msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/"+dutName)
	if err != nil {
		return err
	}
	assert.Loosely(t, msgs, should.HaveLength(1))
	assert.Loosely(t, msgs[0].Delete, should.BeFalse)
	return nil
}

func createValidDUTWithDolos(ctx context.Context, t *ftt.Test, dutName, dutMachine, labstationName, labstationMachine string) error {
	machine1 := &ufspb.Machine{
		Name: labstationMachine,
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS6,
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "test",
				Model:       "test",
			},
		},
	}
	machine2 := &ufspb.Machine{
		Name: dutMachine,
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS6,
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "test",
				Model:       "test",
			},
		},
	}
	_, err := registration.CreateMachine(ctx, machine1)
	if err != nil {
		return err
	}
	_, err = registration.CreateMachine(ctx, machine2)
	if err != nil {
		return err
	}
	labstation1 := mockLabstation(labstationName, labstationMachine)
	_, err = CreateLabstation(ctx, labstation1)
	if err != nil {
		return err
	}
	dut1 := mockDUT(dutName, dutMachine, labstationName, "serial-dolos", dutName+"-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
	addMockDolosToDUT(dut1, labstationName, "dolos-serial-cable", "dolos-power1", ".A2", "dolos-serial-usb")
	_, err = CreateDUT(ctx, dut1)
	if err != nil {
		return err
	}
	changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/"+dutName)
	if err != nil {
		return err
	}
	assert.Loosely(t, changes, should.HaveLength(1))
	msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/"+dutName)
	if err != nil {
		return err
	}
	assert.Loosely(t, msgs, should.HaveLength(1))
	assert.Loosely(t, msgs[0].Delete, should.BeFalse)
	return nil
}

func TestCreateDUT(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = withAuthorizedAtlUser(ctx)
	ftt.Run("CreateDUT", t, func(t *ftt.Test) {
		t.Run("CreateDUT - With non-existent Labstation", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-1", "machine-10", "labstation-1", "serial-1", "dut-1-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("labstation-1 not found in the system"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})
		t.Run("CreateDUT - With non-existent device config", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-20",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name: "machine-21",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "buildtest",
						Model:       "modeltest",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			labstation1 := mockLabstation("labstation-1", "machine-20")
			_, err = CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-2", "machine-21", "labstation-1", "serial-1", "dut-2-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("No device config"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})
		t.Run("CreateDUT - With port conflict on labstation", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-30",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name: "machine-40",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine3 := &ufspb.Machine{
				Name: "machine-50",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)
			labstation1 := mockLabstation("labstation-3", "machine-30")
			_, err = CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-3", "machine-40", "labstation-3", "serial-2", "dut-3-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			dut2 := mockDUT("dut-4", "machine-50", "labstation-3", "serial-3", "dut-4-power-1", ".A2", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Port: 9999 in labstation-3 is already in use by dut-3"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
		})
		t.Run("CreateDUT - With non-existent Dolos host(labstation)", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-520",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name: "machine-521",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)
			_, merr = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, merr, should.BeNil)
			labstation1 := mockLabstation("labstation-520", "machine-521")
			_, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-520", "machine-520", "labstation-520", "serial-520", "dut-520-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "labstation-521", "dolos-serial-cable", "dolos-power1", ".A2", "dolos-serial-usb")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("labstation-521 not found in the system"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-520")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-520")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})
		t.Run("CreateDUT - Happy Path", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-90",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name: "machine-00",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)
			_, merr = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, merr, should.BeNil)
			labstation1 := mockLabstation("labstation-5", "machine-90")
			_, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-7", "machine-00", "labstation-5", "serial-1", "dut-7-power-3", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "labstation-5", "dolos-serial-cable", "dolos-power1", ".A2", "dolos-serial-usb")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
		})
		t.Run("CreateDUT - Existing DUT", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-01",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name: "machine-02",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)
			_, merr = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, merr, should.BeNil)
			labstation1 := mockLabstation("labstation-6", "machine-01")
			_, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-8", "machine-02", "labstation-6", "serial-1", "dut-8-power-3", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("MachineLSE dut-8 already exists in the system"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("CreateDUT - Existing machine", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-03",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)
			labstation1 := mockLabstation("labstation-7", "machine-03")
			_, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-9", "machine-03", "labstation-7", "serial-1", "dut-9-power-3", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Host dut-9 cannot be created because there are other hosts which are referring this machine machine-03"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
		})
		t.Run("CreateDUT - RPM powerunit_name and powerunit_outlet conflict", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-101",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			machine2 := &ufspb.Machine{
				Name: "machine-102",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-16", "machine-101", "", "", "dut-16-power-1", ".A1", 0, nil, "")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			dut2 := mockDUT("dut-17", "machine-102", "", "", "dut-16-power-1", ".A1", 0, nil, "")
			_, err = CreateDUT(ctx, dut2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by dut-16"))
		})
		t.Run("CreateDUT - Skip labstation check if docker container is given", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-103",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-18", "machine-103", "labstation-x", "serial-x", "dut-16-power-1", ".A1", 9988, nil, "docker-1")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestUpdateDUT(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = withAuthorizedAtlUser(ctx)
	ftt.Run("UpdateDUT", t, func(t *ftt.Test) {

		t.Run("UpdateDUT - With non-existent dut", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			dut1 := mockDUT("dut-1", "machine-10", "labstation-1", "serial-1", "dut-1-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			// dut-1 doesn't exist. Should fail.
			_, err := UpdateDUT(ctx, dut1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to get existing MachineLSE"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("UpdateDUT - With non-existent machine", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-1", "machine-20", "labstation-1", "machine-10")
			assert.Loosely(t, err, should.BeNil)
			// Update DUT machine to a non existent one. This should fail.
			dut1 := mockDUT("dut-1", "machine-20-fake", "labstation-1", "serial-1", "dut-1-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Machine with MachineID machine-20-fake in the system"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-1")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With existing deployed machine", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-2", "machine-40", "labstation-2", "machine-30")
			assert.Loosely(t, err, should.BeNil)
			// Update DUT machine to labstations machine (machine-30). Should fail.
			dut1 := mockDUT("dut-2", "machine-30", "labstation-2", "serial-1", "dut-2-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Host dut-2 cannot be updated because there is another host labstation-2 which is referring this machine machine-30"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid name mask", func(t *ftt.Test) {
			err := createValidDUTWithLabstation(ctx, t, "dut-3-name", "machine-60-name", "labstation-3-name", "machine-50-name")
			assert.Loosely(t, err, should.BeNil)
			// Update with name mask.
			dut1 := mockDUT("dut-3-name", "", "", "", "", "", int32(0), nil, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("name"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("name cannot be updated"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-name")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-name")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-name")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid update_time mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-time", "machine-60-time", "labstation-3-time", "machine-50-time")
			assert.Loosely(t, err, should.BeNil)
			// Update with update_time mask.
			dut1 := mockDUT("dut-3-time", "machine-60-time", "labstation-3-time", "serial-1", "dut-3-time-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("update-time"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("is not valid"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-time")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-time")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-time")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid machine mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-machine", "machine-60-machine", "labstation-3-machine", "machine-50-machine")
			assert.Loosely(t, err, should.BeNil)
			// Update with machine mask and no machines.
			dut1 := mockDUT("dut-3-machine", "", "labstation-3-machine", "serial-1", "dut-3-machine-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("machines"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("machines field cannot be empty/nil"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-machine")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-machine")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-machine")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid dut hostname mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-hostname", "machine-60-hostname", "labstation-3-hostname", "machine-50-hostname")
			assert.Loosely(t, err, should.BeNil)
			// Update with dut hostname mask.
			dut1 := mockDUT("dut-3-hostname", "machine-60-hostname", "labstation-3-hostname", "dut-3-hostname-serial-1", "power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.hostname"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("hostname cannot be updated"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-hostname")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-hostname")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-hostname")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid dut pools mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-pools", "machine-60-pools", "labstation-3-pools", "machine-50-pools")
			assert.Loosely(t, err, should.BeNil)
			// Update with dut pools mask and valid pools.
			dut1 := mockDUT("dut-3-pools", "machine-60-pools", "labstation-3-pools", "serial-1", "dut-3-pools-power-1", ".A1", int32(9999), []string{"DUT_POOL_CQ", "DUT_POOL_QUOTA"}, "")
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.pools"))
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-pools")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-pools")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].OldValue, should.Equal("[DUT_POOL_QUOTA]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[DUT_POOL_CQ DUT_POOL_QUOTA]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-pools")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeFalse)
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-pools")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid servo host mask (delete host and update port)", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-non-v3-host", "machine-60-non-v3-host", "labstation-3-non-v3-host", "machine-50-non-v3-host")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-non-v3-host", "machine-60-non-v3-host", "", "", "", "", int32(9999), nil, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.hostname", "dut.servo.port"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot update servo port. Servo host is being reset."))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-non-v3-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-non-v3-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that labstation-3-non-v3-host wasn't changed after last update.
			ls9, err := GetMachineLSE(ctx, "labstation-3-non-v3-host")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-3-non-v3-host",
					ServoPort:     int32(9999),
					ServoSerial:   "serial-1",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-non-v3-host")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid servo host mask (delete host and update serial)", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-non-v3-serial", "machine-60-non-v3-serial", "labstation-3-non-v3-serial", "machine-50-non-v3-serial")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-non-v3-serial", "machine-60-non-v3-serial", "", "dut-3-non-v3-serial-serial-2", "", "", int32(0), nil, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.hostname", "dut.servo.serial"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot update servo serial. Servo host is being reset"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-non-v3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-non-v3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that labstation-3-non-v3-serial wasn't changed after last update.
			ls9, err := GetMachineLSE(ctx, "labstation-3-non-v3-serial")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-3-non-v3-serial",
					ServoPort:     int32(9999),
					ServoSerial:   "serial-1",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-non-v3-serial")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid servo serial mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-serial", "machine-60-serial", "labstation-3-serial", "machine-50-serial")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-serial", "machine-60-serial", "labstation-3-serial", "serial-2", "dut-3-serial-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.serial"))
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-serial")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].OldValue, should.Equal("serial-1"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("serial-2"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			// Verify that labstation-3-serial has updated servo.
			ls9, err := GetMachineLSE(ctx, "labstation-3-serial")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the servo was included in the new labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				{
					ServoHostname: "labstation-3-serial",
					ServoPort:     int32(9999),
					ServoSerial:   "serial-2",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid servo port mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-port", "machine-60-port", "labstation-3-port", "machine-50-port")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo port mask to port 9988.
			dut1 := mockDUT("dut-3-port", "machine-60-port", "labstation-3-port", "serial-1", "dut-3-port-power-1", ".A1", int32(9988), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.port"))
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-3-port")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].OldValue, should.Equal("9999"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("9988"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-3-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-3-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			// Verify that labstation-3-port has updated servo.
			ls9, err := GetMachineLSE(ctx, "labstation-3-port")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the moved servo was included in the new labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-3-port",
					ServoPort:     int32(9988),
					ServoSerial:   "serial-1",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With servo port out of range", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-out-of-range-port", "machine-60-out-of-range-port", "labstation-3-out-of-range-port", "machine-50-out-of-range-port")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-out-of-range-port", "", "", "", "", "", int32(1111), nil, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.port"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Servo port 1111 is invalid"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-out-of-range-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-out-of-range-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that labstation-3-out-of-range-port-servo has no servos.
			ls9, err := GetMachineLSE(ctx, "labstation-3-out-of-range-port")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the servo port wasn't updated on labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.HaveLength(1))
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()[0].GetServoPort(), should.Equal(int32(9999)))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-out-of-range-port")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Remove servo port and auto assign", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-port-auto-assign", "machine-60-port-auto-assign", "labstation-3-port-auto-assign", "machine-50-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-port-auto-assign", "machine-60-port-auto-assign", "labstation-3-port-auto-assign", "serial-1", "dut-3-port-auto-assign-power-1", ".A1", int32(9001), []string{"DUT_POOL_QUOTA"}, "")
			// Change servo port to 9001.
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.port"))
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].OldValue, should.Equal("9999"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("9001"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			// Verify that labstation-3-port-auto-assign-servo has correct servo.
			ls9, err := GetMachineLSE(ctx, "labstation-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the servo was updated in labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.HaveLength(1))
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()[0].GetServoPort(), should.Equal(int32(9001)))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))

			// No port given. Will get 9999 auto assigned to the servo.
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().ServoPort = int32(0)
			// Change servo port to 9999.
			resp, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.port"))
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			resp.UpdateTime = nil
			// Update servo port to 9999 as it was autoassigned.
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().ServoPort = int32(9999)
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err = GetMachineLSE(ctx, "dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[2].OldValue, should.Equal("9001"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("9999"))

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			// Verify that labstation-3-port-auto-assign-servo has correct servo.
			ls9, err = GetMachineLSE(ctx, "labstation-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the servo was updated in labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.HaveLength(1))
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()[0].GetServoPort(), should.Equal(int32(9999)))
			s, err = state.GetStateRecord(ctx, "hosts/dut-3-port-auto-assign")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid servo mask (delete servo)", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-del", "machine-60-del", "labstation-3-del", "machine-50-del")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-del", "machine-60-del", "", "", "dut-3-del-power-1", ".A1", int32(0), []string{"DUT_POOL_QUOTA"}, "")
			// Remove servo from DUT.
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Servo = nil
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.hostname"))
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-del")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-del")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-3-del"))
			assert.Loosely(t, changes[1].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal("9999"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("0"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("serial-1"))
			assert.Loosely(t, changes[3].NewValue, should.BeEmpty)

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-del")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			// Verify that labstation-3-del-servo has no servos.
			ls9, err := GetMachineLSE(ctx, "labstation-3-del")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the servo was deleted in labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.HaveLength(0))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-del")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid servo mask (delete servo) [Servo V3]", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-del-servo", "machine-60-del-servo", "labstation-3-del-serv", "machine-50-del-servo")
			assert.Loosely(t, err, should.BeNil)
			// Update with servo host mask and no servo host.
			dut1 := mockDUT("dut-3-del-servo", "machine-60-del-servo", "", "", "dut-3-del-servo-power-1", ".A1", int32(0), []string{"DUT_POOL_QUOTA"}, "")
			// Remove servo from DUT.
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Servo = nil
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.hostname"))
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			resp.UpdateTime = nil
			// Proto compare as dut1 doesn't contain servo and delete is successful.
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-del-servo")
			assert.Loosely(t, err, should.BeNil)
			// Clear update time to compare the protos
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-del-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-3-del-serv"))
			assert.Loosely(t, changes[1].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal("9999"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("0"))
			// No change in servo serial recorded for servo V3.
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-del-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-del-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid rpm host mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-rpm-host", "machine-60-rpm-host", "labstation-3-rpm-host", "machine-50-rpm-host")
			assert.Loosely(t, err, should.BeNil)
			// Update with rpm host mask and no rpm host.
			dut1 := mockDUT("dut-3-rpm-host", "machine-60-rpm-host", "labstation-3-rpm-host", "dut-3-rpm-host-serial-1", "", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.rpm.host", "dut.rpm.outlet"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Deleting rpm host deletes everything. Cannot update outlet."))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-rpm-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-rpm-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-rpm-host")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid rpm outlet mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-rpm-outlet", "machine-60-rpm-outlet", "labstation-3-rpm-outlet", "machine-50-rpm-outlet")
			assert.Loosely(t, err, should.BeNil)
			// Update with rpm outlet mask and no rpm outlet.
			dut1 := mockDUT("dut-3-rpm-outlet", "", "", "", "", "", int32(0), nil, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.rpm.outlet"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot remove rpm outlet. Please delete rpm"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-rpm-outlet")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-rpm-outlet")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-rpm-outlet")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid rpm mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-rpm", "machine-60-rpm", "labstation-3-rpm", "machine-50-rpm")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-3-rpm", "machine-60-rpm", "labstation-3-rpm", "serial-1", "dut-3-rpm-power-2", ".A2", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.rpm.outlet", "dut.rpm.host"))
			assert.Loosely(t, err, should.BeNil)
			// Remove update time to compare proto
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-rpm")
			assert.Loosely(t, err, should.BeNil)
			dut2.UpdateTime = nil
			// Remove update time to compare proto
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-rpm")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[1].OldValue, should.Equal("dut-3-rpm-power-1"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("dut-3-rpm-power-2"))
			assert.Loosely(t, changes[2].OldValue, should.Equal(".A1"))
			assert.Loosely(t, changes[2].NewValue, should.Equal(".A2"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-rpm")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-rpm")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid rpm mask to delete rpm", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3-rpm-del", "machine-60-rpm-del", "labstation-3-rpm-del", "machine-50-rpm-del")
			assert.Loosely(t, err, should.BeNil)
			// Update with rpm host mask and no rpm.
			dut1 := mockDUT("dut-3-rpm-del", "machine-60-rpm-del", "labstation-3-rpm-del", "serial-1", "", "", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Rpm = nil
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.rpm.host"))
			assert.Loosely(t, err, should.BeNil)
			// Remove update time to compare proto
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-3-rpm-del")
			assert.Loosely(t, err, should.BeNil)
			// Remove update time to compare proto
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3-rpm-del")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[1].OldValue, should.Equal("dut-3-rpm-del-power-1"))
			assert.Loosely(t, changes[1].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal(".A1"))
			assert.Loosely(t, changes[2].NewValue, should.BeEmpty)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3-rpm-del")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3-rpm-del")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid mask", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-3", "machine-60", "labstation-3", "machine-50")
			assert.Loosely(t, err, should.BeNil)
			// Update with invalid masks.
			dut1 := mockDUT("dut-3", "machine-60", "labstation-3", "serial-1", "dut-3-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("invalid-mask-1", "invalid-mask-2"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("is not valid"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Servo port conflict", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-01",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1.
			err = createValidDUTWithLabstation(ctx, t, "dut-6", "machine-00", "labstation-5", "machine-02")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-7", "machine-01", "labstation-5", "serial-2", "dut-7-power-1", ".A1", int32(9998), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			// Update port to 9999 creating conflict with dut-6 servo.
			dut2 := mockDUT("dut-7", "machine-01", "labstation-5", "serial-2", "dut-7-power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			// Maskless update.
			_, err = UpdateDUT(ctx, dut2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Port: 9999 in labstation-5 is already in use by dut-6"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that labstation-5 has 2 servos. And wasn't changed after last update.
			ls9, err := GetMachineLSE(ctx, "labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-5",
					ServoPort:     int32(9999),
					ServoSerial:   "serial-1",
				},
				// dut-9 servo should remain on port 9998.
				{
					ServoHostname: "labstation-5",
					ServoPort:     int32(9998),
					ServoSerial:   "serial-2",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-7")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			s, err = state.GetStateRecord(ctx, "hosts/dut-6")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Servo serial conflict", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-03",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err = createValidDUTWithLabstation(ctx, t, "dut-8", "machine-04", "labstation-6", "machine-05")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-9", "machine-03", "labstation-6", "serial-2", "dut-9-power-1", ".A1", int32(9998), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			// Update dut-9 servo with servo serial of dut-8 (serial-1 created by createValidDUTWithLabstation).
			dut2 := mockDUT("dut-9", "machine-03", "labstation-6", "serial-1", "dut-9-power-1", ".A1", int32(9997), []string{"DUT_POOL_QUOTA"}, "")
			// Maskless update.
			_, err = UpdateDUT(ctx, dut2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Servo serial serial-1 exists"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that labstation-6 has 2 servos. And wasn't changed after last update.
			ls9, err := GetMachineLSE(ctx, "labstation-6")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-6",
					ServoPort:     int32(9999),
					ServoSerial:   "serial-1",
				},
				// dut-9 servo should remain serial-2.
				{
					ServoHostname: "labstation-6",
					ServoPort:     int32(9998),
					ServoSerial:   "serial-2",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-8")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			s, err = state.GetStateRecord(ctx, "hosts/dut-9")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Move servo to different labstation", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-10", "machine-06", "labstation-7", "machine-07")
			assert.Loosely(t, err, should.BeNil)
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err = createValidDUTWithLabstation(ctx, t, "dut-11", "machine-08", "labstation-8", "machine-09")
			assert.Loosely(t, err, should.BeNil)
			// Update the servo serial of the dut to avoid conflict with labstation-7.
			dut2 := mockDUT("dut-11", "", "", "serial-2", "", "", int32(0), nil, "")
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.serial"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			dut2 = mockDUT("dut-11", "", "labstation-7", "", "", "", int32(9998), nil, "")
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.hostname", "dut.servo.port"))
			assert.Loosely(t, err, should.BeNil)
			// Verify that labstation-8 has no servos left on it.
			ls8, err := GetMachineLSE(ctx, "labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ls8.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.HaveLength(0))
			// Verify that labstation-7 has 2 servos
			ls9, err := GetMachineLSE(ctx, "labstation-7")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the moved servo was included in the new labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-7",
					ServoPort:     int32(9999),
					ServoSerial:   "serial-1",
				},
				// The new servo can be obtained from last DUT update's result.
				resp.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo(),
			}))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-11")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-11")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err := state.GetStateRecord(ctx, "hosts/dut-10")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			s, err = state.GetStateRecord(ctx, "hosts/dut-11")
			assert.Loosely(t, err, should.BeNil)
			// State should remain same.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Delete and add servo", func(t *ftt.Test) {
			// Create a DUT with labstation. Also creates servo with port: 9999 and serial: serial-1
			err := createValidDUTWithLabstation(ctx, t, "dut-12", "machine-11", "labstation-9", "machine-12")
			assert.Loosely(t, err, should.BeNil)
			// Update the servo serial of the dut to avoid conflict with labstation-7.
			dut2 := mockDUT("dut-12", "", "", "", "", "", int32(0), nil, "")
			_, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.hostname"))
			assert.Loosely(t, err, should.BeNil)
			// Verify that labstation-9 has no servos left on it.
			ls9, err := GetMachineLSE(ctx, "labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.HaveLength(0))
			// Add the servo back.
			dut2 = mockDUT("dut-12", "", "labstation-9", "serial-2", "", "", int32(9901), nil, "")
			_, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.hostname", "dut.servo.serial", "dut.servo.port"))
			assert.Loosely(t, err, should.BeNil)
			// Verify that labstation-9 has servo
			ls9, err = GetMachineLSE(ctx, "labstation-9")
			assert.Loosely(t, err, should.BeNil)
			// Verify that the moved servo was included in the new labstation.
			assert.Loosely(t, ls9.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.Resemble([]*chromeosLab.Servo{
				// Servo generated by createValidDUTWithLabstation.
				{
					ServoHostname: "labstation-9",
					ServoPort:     int32(9901),
					ServoSerial:   "serial-2",
				},
			}))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-12")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(7))
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-9"))
			assert.Loosely(t, changes[1].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal("9999"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("0"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("serial-1"))
			assert.Loosely(t, changes[3].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[4].NewValue, should.Equal("labstation-9"))
			assert.Loosely(t, changes[4].OldValue, should.BeEmpty)
			assert.Loosely(t, changes[5].NewValue, should.Equal("9901"))
			assert.Loosely(t, changes[5].OldValue, should.Equal("0"))
			assert.Loosely(t, changes[6].NewValue, should.Equal("serial-2"))
			assert.Loosely(t, changes[6].OldValue, should.BeEmpty)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-12")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err := state.GetStateRecord(ctx, "hosts/dut-12")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - RPM powerunit_name and powerunit_outlet conflict", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-101",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			machine2 := &ufspb.Machine{
				Name: "machine-102",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-16", "machine-101", "", "", "dut-16-power-1", ".A1", 0, nil, "")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			dut2 := mockDUT("dut-17", "machine-102", "", "", "dut-17-power-1", ".A1", 0, nil, "")
			_, err = inventory.CreateMachineLSE(ctx, dut2)
			assert.Loosely(t, err, should.BeNil)
			// Update rpm powerunit_name to to dut-16-power-1 creating conflict with dut-16 rpm powerunit_name.
			dut3 := mockDUT("dut-17", "machine-102", "", "", "dut-16-power-1", ".A1", int32(0), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by dut-16"))
		})

		t.Run("UpdateDUT - RPM powerunit_name and powerunit_outlet conflict with multiple DUTs", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-201",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			machine2 := &ufspb.Machine{
				Name: "machine-202",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-216", "machine-201", "", "", "dut-216-power-1", ".A1", 0, nil, "")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			dut2 := mockDUT("dut-217", "machine-202", "", "", "dut-216-power-1", ".A1", 0, nil, "")
			_, err = inventory.CreateMachineLSE(ctx, dut2)
			assert.Loosely(t, err, should.BeNil)
			// Both dut-216 and dut-217 have same RPM info.
			// Update to dut-216 or dut-217 without change in rpm info will fail.
			dut3 := mockDUT("dut-216", "machine-201", "", "", "dut-216-power-1", ".A1", int32(0), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by dut-217"))

			dut4 := mockDUT("dut-217", "machine-202", "", "", "dut-216-power-1", ".A1", int32(0), []string{"DUT_POOL_QUOTA"}, "")
			_, err = UpdateDUT(ctx, dut4, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by dut-216"))
		})
		t.Run("UpdateDUT - Add chameleon to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-13", "machine-13", "labstation-10", "machine-14")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-13")
			assert.Loosely(t, err, should.BeNil)
			// Add chameleon to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chameleon = &chromeosLab.Chameleon{
				ChameleonPeripherals: []chromeosLab.ChameleonType{chromeosLab.ChameleonType_CHAMELEON_TYPE_HDMI, chromeosLab.ChameleonType_CHAMELEON_TYPE_DP},
				AudioBoard:           true,
			}
			dut2.UpdateTime = nil
			// Update chameleon with correct paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.chameleon.type", "dut.chameleon.audioboard"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Validate the proto after update.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Chameleon type recorded.
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[CHAMELEON_TYPE_HDMI CHAMELEON_TYPE_DP]"))
			// Chameleon audioboard recorded.
			assert.Loosely(t, changes[2].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-13")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Remove chameleon from DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-14", "machine-15", "labstation-11", "machine-16")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-14")
			assert.Loosely(t, err, should.BeNil)
			// Add chameleon to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chameleon = &chromeosLab.Chameleon{
				ChameleonPeripherals: []chromeosLab.ChameleonType{chromeosLab.ChameleonType_CHAMELEON_TYPE_HDMI, chromeosLab.ChameleonType_CHAMELEON_TYPE_DP},
				AudioBoard:           true,
			}
			dut2.UpdateTime = nil
			// Update the DUT with proper mask.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.chameleon.type", "dut.chameleon.audioboard"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Validate proto after update.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[CHAMELEON_TYPE_HDMI CHAMELEON_TYPE_DP]"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Delete chameleon
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chameleon = nil
			// UpdateDUT with at least one of the chameleon paths
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.chameleon.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Validate proto after update.
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(5))
			// Verify Chameleon types deleted
			assert.Loosely(t, changes[3].NewValue, should.Equal("[]"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("[CHAMELEON_TYPE_HDMI CHAMELEON_TYPE_DP]"))
			// Verify audiobox reset
			assert.Loosely(t, changes[4].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[4].OldValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Test the same thing but with empty chameleon struct. Expectation is that UFS ignores the empty struct given and sets chameleon to nil on delete.
			// Add chameleon to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chameleon = &chromeosLab.Chameleon{
				ChameleonPeripherals: []chromeosLab.ChameleonType{chromeosLab.ChameleonType_CHAMELEON_TYPE_HDMI, chromeosLab.ChameleonType_CHAMELEON_TYPE_DP},
				AudioBoard:           true,
			}
			dut2.UpdateTime = nil
			// Update the DUT with proper mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.chameleon.type", "dut.chameleon.audioboard"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Validate proto after update.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(7))
			assert.Loosely(t, changes[5].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[5].NewValue, should.Equal("[CHAMELEON_TYPE_HDMI CHAMELEON_TYPE_DP]"))
			assert.Loosely(t, changes[6].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[6].NewValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(4))
			s, err = state.GetStateRecord(ctx, "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Delete chameleon
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chameleon = &chromeosLab.Chameleon{}
			// UpdateDUT with at least one of the chameleon paths
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.chameleon.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Chameleon should be assigned to nil and not empty struct.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chameleon = nil
			// Validate proto after update.
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(9))
			// Verify Chameleon types deleted
			assert.Loosely(t, changes[7].NewValue, should.Equal("[]"))
			assert.Loosely(t, changes[7].OldValue, should.Equal("[CHAMELEON_TYPE_HDMI CHAMELEON_TYPE_DP]"))
			// Verify audiobox reset
			assert.Loosely(t, changes[8].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[8].OldValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(5))
			s, err = state.GetStateRecord(ctx, "hosts/dut-14")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))

		})

		t.Run("UpdateDUT - Add wifi to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-15", "machine-17", "labstation-12", "machine-18")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-15")
			assert.Loosely(t, err, should.BeNil)
			// Add wifi setup to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Wifi = &chromeosLab.Wifi{
				AntennaConn: chromeosLab.Wifi_CONN_CONDUCTIVE,
				Router:      chromeosLab.Wifi_ROUTER_802_11AX,
				Wificell:    true,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.wifi.antennaconn", "dut.wifi.router", "dut.wifi.wificell"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-15")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("CONN_UNKNOWN"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("CONN_CONDUCTIVE"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("true"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("ROUTER_UNSPECIFIED"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("ROUTER_802_11AX"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-15")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-15")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Remove wifi from DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-160", "machine-19", "labstation-13", "machine-21")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-160")
			assert.Loosely(t, err, should.BeNil)
			// Add wifi to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Wifi = &chromeosLab.Wifi{
				AntennaConn: chromeosLab.Wifi_CONN_CONDUCTIVE,
				Router:      chromeosLab.Wifi_ROUTER_802_11AX,
				Wificell:    true,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.wifi.antennaconn", "dut.wifi.router", "dut.wifi.wificell"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify the proto returned.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-160")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("CONN_UNKNOWN"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("CONN_CONDUCTIVE"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("true"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("ROUTER_UNSPECIFIED"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("ROUTER_802_11AX"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-160")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-160")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Delete/reset wifi in DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Wifi = nil
			// Update DUT with at least one of the wifi masks.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.wifi.antennaconn"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-160")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(7))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[4].NewValue, should.Equal("CONN_UNKNOWN"))
			assert.Loosely(t, changes[4].OldValue, should.Equal("CONN_CONDUCTIVE"))
			assert.Loosely(t, changes[5].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[5].OldValue, should.Equal("true"))
			assert.Loosely(t, changes[6].NewValue, should.Equal("ROUTER_UNSPECIFIED"))
			assert.Loosely(t, changes[6].OldValue, should.Equal("ROUTER_802_11AX"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-160")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-160")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Add carrier to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-170", "machine-22", "labstation-14", "machine-23")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-170")
			assert.Loosely(t, err, should.BeNil)
			// Add carrier to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Carrier = "GenericCarrier"
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.carrier"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-170")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.BeEmpty)
			assert.Loosely(t, changes[1].NewValue, should.Equal("GenericCarrier"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-170")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-170")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Remove carrier from DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-18", "machine-24", "labstation-15", "machine-25")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-18")
			assert.Loosely(t, err, should.BeNil)
			// Add carrier to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Carrier = "GenericCarrier"
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.carrier"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-18")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.BeEmpty)
			assert.Loosely(t, changes[1].NewValue, should.Equal("GenericCarrier"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-18")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-18")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Delete/reset carrier in DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Carrier = ""
			// Update DUT with carrier mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.carrier"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-18")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal("GenericCarrier"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-18")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-18")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Set chaos on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-19", "machine-26", "labstation-16", "machine-27")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-19")
			assert.Loosely(t, err, should.BeNil)
			// Add chaos to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chaos = true
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.chaos"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-19")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-19")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-19")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Reset chaos on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-20", "machine-28", "labstation-17", "machine-29")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-20")
			assert.Loosely(t, err, should.BeNil)
			// Set chaos on the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chaos = true
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.chaos"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-20")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// reset chaos in DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Chaos = false
			// Update DUT with chaos mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.chaos"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-20")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Set usb smarthub on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-21", "machine-31", "labstation-18", "machine-32")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-21")
			assert.Loosely(t, err, should.BeNil)
			// Set usb smarthub on DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().SmartUsbhub = true
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.usb.smarthub"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-21")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-21")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-21")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Reset usb smarthub on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-22", "machine-33", "labstation-19", "machine-34")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-22")
			assert.Loosely(t, err, should.BeNil)
			// Set smart usb hub on the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().SmartUsbhub = true
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.usb.smarthub"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-22")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-22")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-22")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// reset usb smart hub on DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().SmartUsbhub = false
			// Update DUT with usb smart hub mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.usb.smarthub"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-22")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-22")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-22")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Add camera to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-23", "machine-35", "labstation-20", "machine-36")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-23")
			assert.Loosely(t, err, should.BeNil)
			// Add camera to DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().ConnectedCamera = []*chromeosLab.Camera{
				{
					CameraType: chromeosLab.CameraType_CAMERA_HUDDLY,
				},
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.camera.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-23")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[camera_type:CAMERA_HUDDLY]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-23")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-23")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Remove camera from DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-24", "machine-37", "labstation-21", "machine-38")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-24")
			assert.Loosely(t, err, should.BeNil)
			// Add camera to DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().ConnectedCamera = []*chromeosLab.Camera{
				{
					CameraType: chromeosLab.CameraType_CAMERA_HUDDLY,
				},
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.camera.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-24")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[camera_type:CAMERA_HUDDLY]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-24")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-24")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Remove camera from DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().ConnectedCamera = nil
			// Update DUT with camera type mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.camera.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-24")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.Equal("[]"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("[camera_type:CAMERA_HUDDLY]"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-24")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-24")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Add cable to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-25", "machine-39", "labstation-22", "machine-41")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-25")
			assert.Loosely(t, err, should.BeNil)
			// Add cable to DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Cable = []*chromeosLab.Cable{
				{
					Type: chromeosLab.CableType_CABLE_USBAUDIO,
				},
				{
					Type: chromeosLab.CableType_CABLE_USBPRINTING,
				},
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.cable.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-25")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[type:CABLE_USBAUDIO type:CABLE_USBPRINTING]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-25")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-25")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Remove cables from DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-26", "machine-42", "labstation-23", "machine-43")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-26")
			assert.Loosely(t, err, should.BeNil)
			// Add cable to DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Cable = []*chromeosLab.Cable{
				{
					Type: chromeosLab.CableType_CABLE_USBAUDIO,
				},
				{
					Type: chromeosLab.CableType_CABLE_USBPRINTING,
				},
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.cable.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-26")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[type:CABLE_USBAUDIO type:CABLE_USBPRINTING]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-26")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-26")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Remove cables from DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Cable = nil
			// Update DUT with camera type mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.cable.type"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-26")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.Equal("[]"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("[type:CABLE_USBAUDIO type:CABLE_USBPRINTING]"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-26")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-26")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Set touch mimo on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-27", "machine-44", "labstation-24", "machine-45")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-27")
			assert.Loosely(t, err, should.BeNil)
			// Set touch mimo on DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Touch = &chromeosLab.Touch{
				Mimo: true,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.touch.mimo"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-27")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("<nil>"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("mimo:true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-27")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-27")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Reset touch mimo on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-28", "machine-46", "labstation-25", "machine-47")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-28")
			assert.Loosely(t, err, should.BeNil)
			// Set touch mimo on DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Touch = &chromeosLab.Touch{
				Mimo: true,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.touch.mimo"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-28")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("<nil>"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("mimo:true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-28")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-28")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Remove cables from DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Touch = nil
			// Update DUT with camera type mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.touch.mimo"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-28")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.Equal("<nil>"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("mimo:true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-28")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-28")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Set camera box on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-29", "machine-48", "labstation-26", "machine-49")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-29")
			assert.Loosely(t, err, should.BeNil)
			// Set camera box on DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Camerabox = true
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.camerabox"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-29")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-29")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-29")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Reset camera box on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-31", "machine-51", "labstation-27", "machine-52")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-31")
			assert.Loosely(t, err, should.BeNil)
			// Set camera box on DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Camerabox = true
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.camerabox"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-31")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-31")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-31")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Remove cables from DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Camerabox = false
			// Update DUT with camera type mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.camerabox"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-31")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-31")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-31")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Add audio to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-32", "machine-53", "labstation-28", "machine-54")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-32")
			assert.Loosely(t, err, should.BeNil)
			// Add audio config to DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Audio = &chromeosLab.Audio{
				AudioBox:   true,
				Atrus:      true,
				AudioCable: true,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.audio.box", "dut.audio.atrus", "dut.audio.cable"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-32")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("true"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-32")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-32")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Delete audio on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-33", "machine-57", "labstation-29", "machine-56")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-33")
			assert.Loosely(t, err, should.BeNil)
			// Add audio config to DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Audio = &chromeosLab.Audio{
				AudioBox:   true,
				Atrus:      true,
				AudioCable: true,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.audio.box", "dut.audio.atrus", "dut.audio.cable"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-33")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("true"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("true"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("false"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-33")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-33")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Remove audio from DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Audio = nil
			// Update DUT with correct mask. Note just one mask is enough to delete audio if Audio = nil
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.audio.box"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-33")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(7))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[4].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[4].OldValue, should.Equal("true"))
			assert.Loosely(t, changes[5].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[5].OldValue, should.Equal("true"))
			assert.Loosely(t, changes[6].NewValue, should.Equal("false"))
			assert.Loosely(t, changes[6].OldValue, should.Equal("true"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-33")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-33")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Add camerabox to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-34", "machine-58", "labstation-30", "machine-59")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-34")
			assert.Loosely(t, err, should.BeNil)
			// Add camerabox config to DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().CameraboxInfo = &chromeosLab.Camerabox{
				Light:  chromeosLab.Camerabox_LIGHT_LED,
				Facing: chromeosLab.Camerabox_FACING_BACK,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.camerabox.facing", "dut.camerabox.light"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-34")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("FACING_UNKNOWN"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("FACING_BACK"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("LIGHT_UNKNOWN"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("LIGHT_LED"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-34")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-34")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Delete camerabox on DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-35", "machine-61", "labstation-31", "machine-62")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-35")
			assert.Loosely(t, err, should.BeNil)
			// Add camerabox config to DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().CameraboxInfo = &chromeosLab.Camerabox{
				Light:  chromeosLab.Camerabox_LIGHT_NOLED,
				Facing: chromeosLab.Camerabox_FACING_FRONT,
			}
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.camerabox.facing", "dut.camerabox.light"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-35")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("FACING_UNKNOWN"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("FACING_FRONT"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("LIGHT_UNKNOWN"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("LIGHT_NOLED"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-35")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-35")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Remove audio from DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().CameraboxInfo = nil
			// Update DUT with correct mask. Note just one mask is enough to delete camerabox.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.camerabox.facing"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-35")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(5))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[3].NewValue, should.Equal("FACING_UNKNOWN"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("FACING_FRONT"))
			assert.Loosely(t, changes[4].NewValue, should.Equal("LIGHT_UNKNOWN"))
			assert.Loosely(t, changes[4].OldValue, should.Equal("LIGHT_NOLED"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-35")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-35")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})
		t.Run("UpdateDUT - Replace bad servo with good one", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-36", "machine-71", "labstation-32", "machine-72")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-36")
			assert.Loosely(t, err, should.BeNil)
			dut2.UpdateTime = nil
			// Delete the labstation
			err = inventory.DeleteMachineLSE(ctx, "labstation-32")
			assert.Loosely(t, err, should.BeNil)
			// Create another valid labstation
			labstation1 := mockLabstation("labstation-33", "machine-72")
			_, err = CreateMachineLSE(ctx, labstation1, nil)
			assert.Loosely(t, err, should.BeNil)
			// Update servo on DUT to point to new labstation
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Servo = &chromeosLab.Servo{
				ServoHostname: "labstation-33",
				ServoPort:     int32(9990),
				ServoSerial:   "serial-2",
			}
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.hostname", "dut.servo.port", "dut.servo.serial"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			// Check the servo changes were recorded.
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-36")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[1].NewValue, should.Equal("labstation-33"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-32"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("9990"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("9999"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("serial-2"))
			assert.Loosely(t, changes[3].OldValue, should.Equal("serial-1"))
			// Two snapshots, one at registration another at update
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-36")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-36")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})
		t.Run("UpdateDUT - Replace servo on misconfigured labstation (serial conflict)", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-37", "machine-81", "labstation-34", "machine82")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-37")
			assert.Loosely(t, err, should.BeNil)
			servo := dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
			// Misconfigure labstation by deleting servo entry in DUT but not labstation
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Servo = nil
			_, err = inventory.BatchUpdateMachineLSEs(ctx, []*ufspb.MachineLSE{dut2})
			assert.Loosely(t, err, should.BeNil)
			// Add the same servo back (serial number conflict on labstation)
			servo.ServoPort = int32(9977)
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Servo = servo
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.hostname", "dut.servo.port", "dut.servo.serial"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			dut2.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			// Check the servo changes were recorded.
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-37")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[1].NewValue, should.Equal("labstation-34"))
			assert.Loosely(t, changes[1].OldValue, should.BeEmpty)
			assert.Loosely(t, changes[2].NewValue, should.Equal("9977"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("0"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("serial-1"))
			assert.Loosely(t, changes[3].OldValue, should.BeEmpty)
			// Two snapshots, one at registration another at update
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-37")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-37")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})
		t.Run("UpdateDUT - Replace Labstation with docker container", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-38", "machine-82", "labstation-35", "machine83")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-38")
			assert.Loosely(t, err, should.BeNil)
			servo := dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
			servo.ServoHostname = "local_labstation"
			servo.DockerContainerName = "docker-1"
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Servo = servo
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.servo.hostname", "dut.servo.dockerContainer"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			dut2.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			// Check the servo changes were recorded.
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-38")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-35"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("local_labstation"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("docker-1"))
			assert.Loosely(t, changes[2].OldValue, should.BeEmpty)
			// Two snapshots, one at registration another at update
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-38")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-38")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			lab2, err := GetMachineLSE(ctx, "labstation-35")
			assert.Loosely(t, err, should.BeNil)
			servos := lab2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()
			assert.Loosely(t, servos, should.HaveLength(0))
		})
		t.Run("UpdateDUT - Replace docker container with non-existent labstation", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-104",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-39", "machine-104", "labstation-x", "serial-x", "dut-39-power-1", ".A1", 9988, nil, "docker-1")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().DockerContainerName = ""
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.dockerContainer"))
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("UpdateDUT - Replace docker container with existing labstation", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-105",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			machine1 = &ufspb.Machine{
				Name: "machine-106",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-40", "machine-105", "labstation-x", "serial-x", "dut-40-power-1", ".A1", 9988, nil, "docker-1")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			// Create a labstation
			lab1 := mockLabstation("labstation-y", "machine-106")
			_, err = inventory.CreateMachineLSE(ctx, lab1)
			assert.Loosely(t, err, should.BeNil)
			// Update the dut to use labstation instead of servod on docker
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().DockerContainerName = ""
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().ServoHostname = "labstation-y"
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.hostname", "dut.servo.dockerContainer"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			dut1.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			// Check the servo changes were recorded.
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-40")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("labstation-x"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("labstation-y"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("docker-1"))
			assert.Loosely(t, changes[1].NewValue, should.BeEmpty)
			// One snapshot at registration
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-40")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-40")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut1.GetResourceState()))
			lab2, err := GetMachineLSE(ctx, "labstation-y")
			assert.Loosely(t, err, should.BeNil)
			servos := lab2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()
			assert.Loosely(t, servos, should.HaveLength(1))
		})
		t.Run("UpdateDUT - Change servo docker container", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-107",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-41", "machine-107", "host-x", "serial-x", "dut-41-power-1", ".A1", 9988, nil, "docker-1")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			// Update the docker container name
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().DockerContainerName = "docker-2"
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.dockerContainer"))
			assert.Loosely(t, err, should.BeNil)
			// Check the servo changes were recorded.
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-41")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("docker-1"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("docker-2"))
			// One snapshot at registration
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-41")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})
		t.Run("UpdateDUT - Change servo type", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-108",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-42", "machine-108", "host-x", "serial-x", "dut-42-power-1", ".A1", 9988, nil, "docker-1")
			_, err = inventory.CreateMachineLSE(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			// Update the servo setup type
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().ServoSetup = chromeosLab.ServoSetupType_SERVO_SETUP_DUAL_V4
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.servo.setup"))
			assert.Loosely(t, err, should.BeNil)
			// Check the servo changes were recorded.
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-42")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("SERVO_SETUP_REGULAR"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("SERVO_SETUP_DUAL_V4"))
			// One snapshot at registration
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-42")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("UpdateDUT - Add Starfish slot mapping to DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-43", "machine-109", "labstation-36", "machine-84")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-43")
			assert.Loosely(t, err, should.BeNil)
			// Add Starfish slot mapping to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().StarfishSlotMapping = "GenericMapping"
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.starfishSlotMapping"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-43")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.BeEmpty)
			assert.Loosely(t, changes[1].NewValue, should.Equal("GenericMapping"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-43")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-43")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - Remove Starfish slot mapping from DUT", func(t *ftt.Test) {
			// Create a DUT with labstation.
			err := createValidDUTWithLabstation(ctx, t, "dut-44", "machine-110", "labstation-37", "machine-85")
			assert.Loosely(t, err, should.BeNil)
			dut2, err := GetMachineLSE(ctx, "dut-44")
			assert.Loosely(t, err, should.BeNil)
			// Add Starfish slot mapping to the DUT.
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().StarfishSlotMapping = "GenericMapping"
			dut2.UpdateTime = nil
			// Update DUT with proper paths.
			resp, err := UpdateDUT(ctx, dut2, mockFieldMask("dut.starfishSlotMapping"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			// Verify that the returned proto is updated.
			assert.Loosely(t, dut2, should.Resemble(resp))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-44")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.BeEmpty)
			assert.Loosely(t, changes[1].NewValue, should.Equal("GenericMapping"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-44")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-44")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
			// Delete/reset Starfish slot mapping in DUT
			dut2.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().StarfishSlotMapping = ""
			// Update DUT with Starfish slot mapping mask.
			resp, err = UpdateDUT(ctx, dut2, mockFieldMask("dut.starfishSlotMapping"))
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-44")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify that the changes were recorded by the history.
			assert.Loosely(t, changes[2].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal("GenericMapping"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-44")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			s, err = state.GetStateRecord(ctx, "hosts/dut-44")
			assert.Loosely(t, err, should.BeNil)
			// State should be unchanged.
			assert.Loosely(t, s.GetState(), should.Equal(dut2.GetResourceState()))
		})

		t.Run("UpdateDUT - With invalid dolos mask (delete host and update serial cable)", func(t *ftt.Test) {
			err := createValidDUTWithDolos(ctx, t, "dut-dolos-1", "machine-dolos-1", "labstation-dolos-1", "machine-dolos-host1")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-dolos-1", "machine-dolos-1", "labstation-dolos-1", "serial-dolos", "dut-dolos-1-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "", "dolos-serial-cable", "dolos-power1", ".A2", "dolos-serial-usb")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.hostname", "dut.dolos.serial.cable"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot update dolos serial cable. Dolos host is being reset"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that dut-dolos-1 wasn't changed after last update.
			newDut, err := GetMachineLSE(ctx, "dut-dolos-1")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, newDut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetDolos(), should.Resemble(&chromeosLab.Dolos{
				Hostname:    "labstation-dolos-1",
				SerialCable: "dolos-serial-cable",
				SerialUsb:   "dolos-serial-usb",
				Rpm: &chromeosLab.OSRPM{
					PowerunitName:   "dolos-power1",
					PowerunitOutlet: ".A2",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-1")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With invalid dolos mask (delete host and update serial usb)", func(t *ftt.Test) {
			err := createValidDUTWithDolos(ctx, t, "dut-dolos-2", "machine-dolos-2", "labstation-dolos-2", "machine-dolos-host2")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-dolos-2", "machine-dolos-2", "labstation-dolos-2", "serial-dolos", "dut-dolos-2-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "", "dolos-serial-cable", "dolos-power1", ".A2", "dolos-serial-usb")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.hostname", "dut.dolos.serial.usb"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot update dolos serial usb. Dolos host is being reset"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Verify that dut-dolos-2 wasn't changed after last update.
			newDut, err := GetMachineLSE(ctx, "dut-dolos-2")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, newDut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetDolos(), should.Resemble(&chromeosLab.Dolos{
				Hostname:    "labstation-dolos-2",
				SerialCable: "dolos-serial-cable",
				SerialUsb:   "dolos-serial-usb",
				Rpm: &chromeosLab.OSRPM{
					PowerunitName:   "dolos-power1",
					PowerunitOutlet: ".A2",
				},
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-2")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Add dolos to a DUT(created without dolos)", func(t *ftt.Test) {
			err := createValidDUTWithLabstation(ctx, t, "dut-dolos-3", "machine-dolos-3", "labstation-dolos-3", "machine-dolos-host3")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-dolos-3", "machine-dolos-3", "labstation-dolos-3", "serial-dolos", "dut-dolos-3-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "labstation-dolos-3", "dolos-serial-cable", "dolos-power1", ".A2", "dolos-serial-usb")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.hostname", "dut.dolos.serial.usb", "dut.dolos.serial.cable"))
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			// Verify that dut-dolos-3 changed as expected after last update.
			newDut, err := GetMachineLSE(ctx, "dut-dolos-3")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, newDut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetDolos(), should.Resemble(&chromeosLab.Dolos{
				Hostname:    "labstation-dolos-3",
				SerialCable: "dolos-serial-cable",
				SerialUsb:   "dolos-serial-usb",
			}))
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-3")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - Remove the dolos device from a DUT", func(t *ftt.Test) {
			err := createValidDUTWithDolos(ctx, t, "dut-dolos-4", "machine-dolos-4", "labstation-dolos-4", "machine-dolos-host4")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-dolos-4", "machine-dolos-4", "labstation-dolos-4", "serial-dolos", "dut-dolos-4-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "", "", "", "", "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.hostname"))
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(6))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			// Verify that dut-dolos-4 changed as expected after last update.
			newDut, err := GetMachineLSE(ctx, "dut-dolos-4")
			assert.Loosely(t, err, should.BeNil)
			// Verify that nothing was changed on labstation.
			assert.Loosely(t, newDut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetDolos(), should.BeNil)
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-4")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered. No change.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDolosDUT - With invalid rpm outlet mask", func(t *ftt.Test) {
			err := createValidDUTWithDolos(ctx, t, "dut-dolos-5", "machine-dolos-5", "labstation-dolos-5", "machine-dolos-host5")
			assert.Loosely(t, err, should.BeNil)
			// Update with rpm outlet mask and no rpm outlet.
			dut1 := mockDUT("dut-dolos-5", "machine-dolos-5", "labstation-dolos-5", "serial-dolos", "dut-dolos-5-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "", "", "", "", "")
			_, err = UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.rpm.outlet"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot remove dolos rpm outlet. Please delete dolos rpm"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-5")
			assert.Loosely(t, err, should.BeNil)
			// State should be set to registered.
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDUT - With valid dolos rpm mask", func(t *ftt.Test) {
			err := createValidDUTWithDolos(ctx, t, "dut-dolos-6", "machine-dolos-6", "labstation-dolos-6", "machine-dolos-host6")
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-dolos-6", "machine-dolos-6", "labstation-dolos-6", "serial-dolos", "dut-dolos-6-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "labstation-dolos-6", "dolos-serial-cable", "dolos-power2", ".A3", "dolos-serial-usb")
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.rpm.host", "dut.dolos.rpm.outlet"))
			assert.Loosely(t, err, should.BeNil)
			// Remove update time to compare proto
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-dolos-6")
			assert.Loosely(t, err, should.BeNil)
			dut2.UpdateTime = nil
			// Remove update time to compare proto
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[1].OldValue, should.Equal("dolos-power1"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("dolos-power2"))
			assert.Loosely(t, changes[2].OldValue, should.Equal(".A2"))
			assert.Loosely(t, changes[2].NewValue, should.Equal(".A3"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("UpdateDolosDUT - With valid rpm mask to delete rpm", func(t *ftt.Test) {
			err := createValidDUTWithDolos(ctx, t, "dut-dolos-7", "machine-dolos-7", "labstation-dolos-7", "machine-dolos-host7")
			assert.Loosely(t, err, should.BeNil)
			// Update with rpm host mask and no rpm.
			dut1 := mockDUT("dut-dolos-7", "machine-dolos-7", "labstation-dolos-7", "serial-dolos", "dut-dolos-7-power-dolos", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			addMockDolosToDUT(dut1, "labstation-dolos-7", "dolos-serial-cable", "", ".A2", "dolos-serial-usb")
			dut1.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetDolos().Rpm = nil
			resp, err := UpdateDUT(ctx, dut1, mockFieldMask("dut.dolos.rpm.host"))
			assert.Loosely(t, err, should.BeNil)
			// Remove update time to compare proto
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Resemble(dut1))
			dut2, err := GetMachineLSE(ctx, "dut-dolos-7")
			assert.Loosely(t, err, should.BeNil)
			// Remove update time to compare proto
			dut2.UpdateTime = nil
			assert.Loosely(t, dut2, should.Resemble(dut1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-dolos-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[1].OldValue, should.Equal("dolos-power1"))
			assert.Loosely(t, changes[1].NewValue, should.Equal(""))
			assert.Loosely(t, changes[2].OldValue, should.Equal(".A2"))
			assert.Loosely(t, changes[2].NewValue, should.Equal(""))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-dolos-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			s, err := state.GetStateRecord(ctx, "hosts/dut-dolos-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

	})
}

func TestGetChromeOSDeviceData(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = useTestingCfg(ctx)
	ctx = gologger.StdConfig.Use(ctx)
	ctx = logging.SetLevel(ctx, logging.Error)

	machine := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: "test",
				BuildTarget:    "test",
				Model:          "test",
				Hwid:           "test",
				Sku:            "100",
			},
		},
	}
	registration.CreateMachine(ctx, machine)

	dutMachinelse := mockDutMachineLSE("lse-1")
	dutMachinelse.Machines = []string{"machine-1"}
	inventory.CreateMachineLSE(ctx, dutMachinelse)

	dutState := mockDutState("machine-1", "lse-1")
	UpdateDutState(ctx, dutState)

	mfgCfgBase := &ufsmanufacturing.ManufacturingConfig{
		ManufacturingId: &ufsmanufacturing.ConfigID{Value: "test"},
		HwidComponent:   []string{"test_component/test_component_value"},
	}

	hwidMockData := mockHwidData()

	flatConfig := &payload.FlatConfig{
		HwDesign: &api.Design{
			Id: &api.DesignId{
				Value: "test",
			},
			ProgramId: &api.ProgramId{
				Value: "test",
			},
			Name: "test",
		},
		HwDesignConfig: &api.Design_Config{
			Id: &api.DesignConfigId{
				Value: "test:100",
			},
		},
	}
	configuration.UpdateFlatConfig(ctx, flatConfig)

	attr := mockDutAttribute("attr-design", "hw_design.id.value")
	configuration.UpdateDutAttribute(ctx, attr)

	ftt.Run("TestGetChromeOSDevicedata", t, func(t *ftt.Test) {
		t.Run("GetChromeOSDevicedata - id happy path", func(t *ftt.Test) {
			resp, err := GetChromeOSDeviceData(ctx, "machine-1", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machine))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutState))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfgBase))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(hwidMockData))
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetSchedulableLabels(), should.ContainKey("hw-test-component"))
			assert.Loosely(t, resp.GetSchedulableLabels()["hw-test-component"].GetLabelValues(), should.Resemble([]string{"test_component_value"}))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
			assert.Loosely(t, resp.GetDutV1().GetCommon().GetLabels().GetStability(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - hostname happy path", func(t *ftt.Test) {
			resp, err := GetChromeOSDeviceData(ctx, "", "lse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machine))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutState))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfgBase))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(hwidMockData))
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetSchedulableLabels(), should.ContainKey("hw-test-component"))
			assert.Loosely(t, resp.GetSchedulableLabels()["hw-test-component"].GetLabelValues(), should.Resemble([]string{"test_component_value"}))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - turn off schedulable label flag", func(t *ftt.Test) {
			cfgLst := &config.Config{
				HwidServiceTrafficRatio: 1.0,
				EnableBoxsterLabels:     false,
			}
			noSchedLabelsCtx := config.Use(ctx, cfgLst)

			resp, err := GetChromeOSDeviceData(noSchedLabelsCtx, "", "lse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machine))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutState))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfgBase))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(hwidMockData))
			assert.Loosely(t, resp.GetSchedulableLabels(), should.Resemble(map[string]*ufspb.SchedulableLabelValues(nil)))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeFalse)
		})

		t.Run("GetChromeOSDevicedata - InvV2 errors", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "testerr",
						Model:       "testerr",
						Hwid:        "testerr",
						Sku:         "100",
					},
				},
			}
			registration.CreateMachine(ctx, machine)

			dutMachinelse := mockDutMachineLSE("lse-2")
			dutMachinelse.Machines = []string{"machine-2"}
			inventory.CreateMachineLSE(ctx, dutMachinelse)

			dutState := mockDutState("machine-2", "lse-2")
			UpdateDutState(ctx, dutState)

			flatConfig := &payload.FlatConfig{
				HwDesign: &api.Design{
					Id: &api.DesignId{
						Value: "testerr",
					},
					ProgramId: &api.ProgramId{
						Value: "testerr",
					},
					Name: "testerr",
				},
				HwDesignConfig: &api.Design_Config{
					Id: &api.DesignConfigId{
						Value: "testerr:100",
					},
				},
			}
			configuration.UpdateFlatConfig(ctx, flatConfig)

			resp, err := GetChromeOSDeviceData(ctx, "", "lse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machine))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutState))
			assert.Loosely(t, resp.GetDeviceConfig(), should.BeNil)
			assert.Loosely(t, resp.GetManufacturingConfig(), should.BeNil)
			assert.Loosely(t, resp.GetHwidData(), should.BeNil)
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"testerr"})
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - data not found", func(t *ftt.Test) {
			// DutState, DeviceConfig, ManufacturingConfig, HwidData,
			// SchedulableLabels all do not exist for "machine-3"
			machine := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "testerr",
						Model:       "testerr",
						Hwid:        "testerr",
						Sku:         "100",
					},
				},
			}
			registration.CreateMachine(ctx, machine)

			dutMachinelse := mockDutMachineLSE("lse-3")
			dutMachinelse.Machines = []string{"machine-3"}
			inventory.CreateMachineLSE(ctx, dutMachinelse)

			flatConfig := &payload.FlatConfig{
				HwDesign: &api.Design{
					Id: &api.DesignId{
						Value: "testerr",
					},
					ProgramId: &api.ProgramId{
						Value: "testerr",
					},
					Name: "testerr",
				},
				HwDesignConfig: &api.Design_Config{
					Id: &api.DesignConfigId{
						Value: "testerr:100",
					},
				},
			}
			configuration.UpdateFlatConfig(ctx, flatConfig)

			resp, err := GetChromeOSDeviceData(ctx, "", "lse-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machine))
			assert.Loosely(t, resp.GetDutState(), should.BeNil)
			assert.Loosely(t, resp.GetDeviceConfig(), should.BeNil)
			assert.Loosely(t, resp.GetManufacturingConfig(), should.BeNil)
			assert.Loosely(t, resp.GetHwidData(), should.BeNil)
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"testerr"})
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - machine not found by hostname", func(t *ftt.Test) {
			dutMachinelse := mockDutMachineLSE("lse-4")
			dutMachinelse.Machines = []string{"machine-4"}
			inventory.CreateMachineLSE(ctx, dutMachinelse)

			resp, err := GetChromeOSDeviceData(ctx, "", "lse-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.BeNil)
			assert.Loosely(t, resp.GetDutState(), should.BeNil)
			assert.Loosely(t, resp.GetDeviceConfig(), should.BeNil)
			assert.Loosely(t, resp.GetManufacturingConfig(), should.BeNil)
			assert.Loosely(t, resp.GetHwidData(), should.BeNil)
			assert.Loosely(t, resp.GetSchedulableLabels(), should.Resemble(map[string]*ufspb.SchedulableLabelValues(nil)))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeFalse)
		})

		t.Run("GetChromeOSDevicedata - machine not found by id", func(t *ftt.Test) {
			dutMachinelse := mockDutMachineLSE("lse-5")
			dutMachinelse.Machines = []string{"machine-5"}
			inventory.CreateMachineLSE(ctx, dutMachinelse)

			resp, err := GetChromeOSDeviceData(ctx, "machine-5", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelse))
			assert.Loosely(t, resp.GetMachine(), should.BeNil)
			assert.Loosely(t, resp.GetDutState(), should.BeNil)
			assert.Loosely(t, resp.GetDeviceConfig(), should.BeNil)
			assert.Loosely(t, resp.GetManufacturingConfig(), should.BeNil)
			assert.Loosely(t, resp.GetHwidData(), should.BeNil)
			assert.Loosely(t, resp.GetSchedulableLabels(), should.Resemble(map[string]*ufspb.SchedulableLabelValues(nil)))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeFalse)
		})

		t.Run("GetChromeOSDevicedata - machinelse not found Error", func(t *ftt.Test) {
			resp, err := GetChromeOSDeviceData(ctx, "machine-6", "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("NotFound"))
		})

		t.Run("GetChromeOSDevicedata - happy path; hwid out of date, server fails, and use the cached info", func(t *ftt.Test) {
			// Mock server fails with test-no-server so use expired data found in datastore
			// instead.
			expiredTime := time.Now().Add(-2 * time.Hour).UTC()
			expiredHwidData := mockHwidDataNoServer()
			fakeUpdateHwidData(ctx, expiredHwidData, "test-no-server", expiredTime)

			machineExp := &ufspb.Machine{
				Name: "machine-using-exp",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test-no-server",
						Sku:            "100",
					},
				},
			}
			registration.CreateMachine(ctx, machineExp)

			mfgCfg := &ufsmanufacturing.ManufacturingConfig{
				ManufacturingId: &ufsmanufacturing.ConfigID{Value: "test-no-server"},
			}

			dutMachinelseExp := mockDutMachineLSE("lse-using-exp")
			dutMachinelseExp.Machines = []string{"machine-using-exp"}
			inventory.CreateMachineLSE(ctx, dutMachinelseExp)

			dutStateExp := mockDutState("machine-using-exp", "lse-using-exp")
			UpdateDutState(ctx, dutStateExp)

			resp, err := GetChromeOSDeviceData(ctx, "machine-using-exp", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelseExp))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machineExp))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutStateExp))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfg))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(expiredHwidData))
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			assert.Loosely(t, resp.GetSchedulableLabels(), should.NotContainKey("hw-test-component"))
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
			assert.Loosely(t, resp.GetDutV1().GetCommon().GetLabels().GetStability(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - happy path; hwid out of date and new cache", func(t *ftt.Test) {
			// Datastore data is expired. Query hwid server, use values, and update cache.
			expiredTime := time.Now().Add(-2 * time.Hour).UTC()
			expiredHwidData := &ufspb.HwidData{
				Sku:     "test-sku-exp",
				Variant: "test-variant-exp",
				Hwid:    "test-hwid-exp",
				DutLabel: &ufspb.DutLabel{
					PossibleLabels: []string{
						"test-possible-1",
						"test-possible-2",
					},
					Labels: []*ufspb.DutLabel_Label{
						{
							Name:  "test-label-1",
							Value: "test-value-1",
						},
						{
							Name:  "Sku",
							Value: "test-sku-exp",
						},
						{
							Name:  "variant",
							Value: "test-variant-exp",
						},
					},
				},
			}
			const hwid = "test"
			fakeUpdateHwidData(ctx, expiredHwidData, hwid, expiredTime)

			machineHwid := &ufspb.Machine{
				Name: "machine-using-hwid-server",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test",
						Sku:            "100",
					},
				},
			}
			registration.CreateMachine(ctx, machineHwid)

			dutMachinelseHwid := mockDutMachineLSE("lse-using-hwid-server")
			dutMachinelseHwid.Machines = []string{"machine-using-hwid-server"}
			inventory.CreateMachineLSE(ctx, dutMachinelseHwid)

			dutStateHwid := mockDutState("machine-using-hwid-server", "lse-using-hwid-server")
			UpdateDutState(ctx, dutStateHwid)

			hwidEnt, err := configuration.GetHwidData(ctx, hwid)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, hwidEnt, should.NotBeNil)
			assert.Loosely(t, hwidEnt.Updated, should.HappenBefore(expiredTime.Add(1*time.Millisecond)))

			resp, err := GetChromeOSDeviceData(ctx, "machine-using-hwid-server", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelseHwid))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machineHwid))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutStateHwid))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfgBase))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(hwidMockData))
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetSchedulableLabels(), should.ContainKey("hw-test-component"))
			assert.Loosely(t, resp.GetSchedulableLabels()["hw-test-component"].GetLabelValues(), should.Resemble([]string{"test_component_value"}))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
			assert.Loosely(t, resp.GetDutV1().GetCommon().GetLabels().GetStability(), should.BeTrue)

			hwidEnt, err = configuration.GetHwidData(ctx, hwid)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, hwidEnt, should.NotBeNil)
			assert.Loosely(t, hwidEnt.Updated, should.HappenBefore(time.Now().UTC().Add(1*time.Second)))
		})

		t.Run("GetChromeOSDevicedata - throttle hwid server traffic and no data in datastore", func(t *ftt.Test) {
			// Try to get data from datastore but no data. Throttle traffic to hwid
			// server. HwidData should be nil. "test-no-cached-hwid-data" returns
			// valid fake, but HwidClient should not be called due to throttle.
			cfgLst := &config.Config{
				HwidServiceTrafficRatio: 0,
				EnableBoxsterLabels:     true,
			}
			trafficCtx := config.Use(ctx, cfgLst)

			machineThrottle := &ufspb.Machine{
				Name: "machine-throttle-hwid",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test-no-cached-hwid-data",
						Sku:            "100",
					},
				},
			}
			registration.CreateMachine(ctx, machineThrottle)

			dutMachinelseThrottle := mockDutMachineLSE("lse-throttle-hwid")
			dutMachinelseThrottle.Machines = []string{"machine-throttle-hwid"}
			inventory.CreateMachineLSE(ctx, dutMachinelseThrottle)

			dutStateThrottle := mockDutState("machine-throttle-hwid", "lse-throttle-hwid")
			UpdateDutState(ctx, dutStateThrottle)

			resp, err := GetChromeOSDeviceData(trafficCtx, "machine-throttle-hwid", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelseThrottle))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machineThrottle))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutStateThrottle))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.BeNil)
			assert.Loosely(t, resp.GetHwidData(), should.BeNil)
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetSchedulableLabels(), should.NotContainKey("hw-test-component"))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
			assert.Loosely(t, resp.GetDutV1().GetCommon().GetLabels().GetStability(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - normal hwid server traffic and no data in datastore", func(t *ftt.Test) {
			// Try to get data from datastore but no data. No throttle traffic to
			// hwid server. HwidData should return fake data using
			// "test-no-cached-hwid-data".
			machineNoThrottle := &ufspb.Machine{
				Name: "machine-no-throttle-hwid",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test-no-cached-hwid-data",
						Sku:            "100",
					},
				},
			}
			registration.CreateMachine(ctx, machineNoThrottle)

			dutMachinelseNoThrottle := mockDutMachineLSE("lse-no-throttle-hwid")
			dutMachinelseNoThrottle.Machines = []string{"machine-no-throttle-hwid"}
			inventory.CreateMachineLSE(ctx, dutMachinelseNoThrottle)

			dutStateNoThrottle := mockDutState("machine-no-throttle-hwid", "lse-no-throttle-hwid")
			UpdateDutState(ctx, dutStateNoThrottle)

			hwidNoCachedMockData := mockHwidData()
			hwidNoCachedMockData.Hwid = "test-no-cached-hwid-data"

			mfgCfg := &ufsmanufacturing.ManufacturingConfig{
				ManufacturingId: &ufsmanufacturing.ConfigID{Value: "test-no-cached-hwid-data"},
				HwidComponent:   []string{"test_component/test_component_value"},
			}

			resp, err := GetChromeOSDeviceData(ctx, "machine-no-throttle-hwid", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelseNoThrottle))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machineNoThrottle))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutStateNoThrottle))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfg))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(hwidNoCachedMockData))
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetSchedulableLabels(), should.ContainKey("hw-test-component"))
			assert.Loosely(t, resp.GetSchedulableLabels()["hw-test-component"].GetLabelValues(), should.Resemble([]string{"test_component_value"}))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
			assert.Loosely(t, resp.GetDutV1().GetCommon().GetLabels().GetStability(), should.BeTrue)
		})

		t.Run("GetChromeOSDevicedata - legacy DutLabel data in datastore", func(t *ftt.Test) {
			// Try to get data from datastore but DutLabel data is in datastore
			// instead of HwidData proto. GetHwidData should return HwidData proto
			// from datastore even when using "test-legacy-hwid-data". Fake server
			// should not be called since there is data in datastore.
			machineLegacyHwid := &ufspb.Machine{
				Name: "machine-legacy-hwid",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test-legacy-hwid-data",
						Sku:            "100",
					},
				},
			}
			registration.CreateMachine(ctx, machineLegacyHwid)

			dutMachinelseLegacyHwid := mockDutMachineLSE("lse-legacy-hwid")
			dutMachinelseLegacyHwid.Machines = []string{"machine-legacy-hwid"}
			inventory.CreateMachineLSE(ctx, dutMachinelseLegacyHwid)

			dutStateLegacyHwid := mockDutState("machine-legacy-hwid", "lse-legacy-hwid")
			UpdateDutState(ctx, dutStateLegacyHwid)

			// Mock data to store DutLabel in the datastore entity. The hwid key used
			// is "test-legacy-hwid-data".
			legacyMockData := mockDutLabel()
			entResp, err := fakeUpdateLegacyHwidData(ctx, legacyMockData, "test-legacy-hwid-data", time.Now().UTC())
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entResp, should.NotBeNil)

			hwidCachedLegacyData := &ufspb.HwidData{
				Sku:      "test-legacy-sku",
				Variant:  "test-legacy-variant",
				Hwid:     "test-legacy-hwid-data",
				DutLabel: legacyMockData,
			}

			mfgCfg := &ufsmanufacturing.ManufacturingConfig{
				ManufacturingId: &ufsmanufacturing.ConfigID{Value: "test-legacy-hwid-data"},
			}

			resp, err := GetChromeOSDeviceData(ctx, "machine-legacy-hwid", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Resemble(dutMachinelseLegacyHwid))
			assert.Loosely(t, resp.GetMachine(), should.Resemble(machineLegacyHwid))
			assert.Loosely(t, resp.GetDutState(), should.Resemble(dutStateLegacyHwid))
			assert.Loosely(t, resp.GetManufacturingConfig(), should.Resemble(mfgCfg))
			assert.Loosely(t, resp.GetHwidData(), should.Resemble(hwidCachedLegacyData))
			// So(resp.GetSchedulableLabels(), ShouldContainKey, "attr-design")
			// So(resp.GetSchedulableLabels()["attr-design"].GetLabelValues(), ShouldResemble, []string{"test"})
			assert.Loosely(t, resp.GetSchedulableLabels(), should.NotContainKey("hw-test-component"))
			assert.Loosely(t, resp.GetRespectAutomatedSchedulableLabels(), should.BeTrue)
			assert.Loosely(t, resp.GetDutV1().GetCommon().GetLabels().GetStability(), should.BeTrue)
		})
	})
}

func TestValidateDeviceconfig(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("ValidateDeviceconfig", t, func(t *ftt.Test) {
		t.Run("TestValidateDeviceconfig - With sku", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
						Sku:         "test",
					},
				},
			}
			err := validateDeviceConfig(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("TestValidateDeviceconfig - Without sku", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			err := validateDeviceConfig(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("TestValidateDeviceconfig - non exisitent", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-11",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test11",
						Model:       "test11",
						Sku:         "test11",
					},
				},
			}
			err := validateDeviceConfig(ctx, machine1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("No device config"))
		})
	})
}

func TestRenameDUT(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = withAuthorizedAtlUser(ctx)
	ftt.Run("renameDUT", t, func(t *ftt.Test) {
		t.Run("renameDUT - Rename a dut in scheduling unit", func(t *ftt.Test) {
			err := createValidDUTWithLabstation(ctx, t, "dut-1", "machine-1d", "labstation-1", "machine-1l")
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
				Name:        "su-1",
				MachineLSEs: []string{"dut-1"},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = RenameMachineLSE(ctx, "dut-1", "dut-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.FailedPrecondition.String()))
		})
		t.Run("renameDUT - Happy path", func(t *ftt.Test) {
			err := createValidDUTWithLabstation(ctx, t, "dut-2", "machine-2d", "labstation-2", "machine-2l")
			assert.Loosely(t, err, should.BeNil)
			_, err = RenameMachineLSE(ctx, "dut-2", "dut-3")
			assert.Loosely(t, err, should.BeNil)
			// Two snapshots, one at registration one at rename
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			// One snapshot at registration
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("dut-2"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("dut-3"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("dut-2"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("dut-3"))
			// State record for old dut should be deleted
			_, err = state.GetStateRecord(ctx, "hosts/dut-2")
			assert.Loosely(t, err, should.NotBeNil)
			// State record for new dut should be same as old dut
			s, err := state.GetStateRecord(ctx, "hosts/dut-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})
	})
}

func TestCheckDutIdAndHostnameAreAssociated(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = withAuthorizedAtlUser(ctx)
	ftt.Run("Check DUT Id and Hostname", t, func(t *ftt.Test) {
		t.Run("renameDUT - Rename a dut in scheduling unit", func(t *ftt.Test) {
			err := createValidDUTWithLabstation(ctx, t, "dut-1", "machine-20", "labstation-1", "machine-10")
			assert.Loosely(t, err, should.BeNil)
			err = checkDutIdAndHostnameAreAssociated(ctx, "machine-20", "dut-1")
			assert.Loosely(t, err, should.BeNil)
			err = checkDutIdAndHostnameAreAssociated(ctx, "machine-20", "dut-2")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestUpdateRecoveryData(t *testing.T) {
	t.Parallel()
	ftt.Run("Setup data", t, func(t *ftt.Test) {
		// new context every test allows us to reuse test data in a clean
		// context every time.
		baseCtx := external.WithTestingContext(testingContext())
		ctx := withAuthorizedAtlUser(baseCtx)

		const dutName = "dut-1"
		const dutMachine = "dut-machine-1"
		const labstationName = "labstation-1"
		const labstationMachine = "labstation-machine-1"
		req := &ufsAPI.UpdateDeviceRecoveryDataRequest{
			DeviceId:     dutMachine,
			Hostname:     dutName,
			ResourceType: ufsAPI.UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE,
			DeviceRecoveryData: &ufsAPI.UpdateDeviceRecoveryDataRequest_Chromeos{
				Chromeos: &ufsAPI.ChromeOsRecoveryData{
					DutState: &chromeosLab.DutState{
						Id: &chromeosLab.ChromeOSDeviceID{
							Value: dutMachine,
						},
						Hostname: dutName,
					},
					DutData: &ufsAPI.ChromeOsRecoveryData_DutData{
						SerialNumber: "serialNumber",
						HwID:         "hwID",
						DeviceSku:    "deviceSku",
						DlmSkuId:     "12345",
					},
					LabData: &ufsAPI.ChromeOsRecoveryData_LabData{},
				},
			},
			ResourceState: ufspb.State_STATE_READY,
		}
		err := createValidDUTWithLabstation(ctx, t, dutName, dutMachine, labstationName, labstationMachine)
		assert.Loosely(t, err, should.BeNil)
		machine, err := registration.GetMachine(ctx, dutMachine)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, machine.GetSerialNumber(), should.BeEmpty)
		assert.Loosely(t, machine.GetChromeosMachine().GetHwid(), should.BeEmpty)
		assert.Loosely(t, machine.GetChromeosMachine().GetSku(), should.BeEmpty)
		asset, err := registration.CreateAsset(ctx, &ufspb.Asset{
			Name: dutMachine,
			Info: &ufspb.AssetInfo{
				AssetTag: dutMachine + "-asset-1",
			},
			Type:     ufspb.AssetType_DUT,
			Location: &ufspb.Location{},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.BeEmpty)
		assert.Loosely(t, asset.GetInfo().GetHwid(), should.BeEmpty)
		assert.Loosely(t, asset.GetInfo().GetSku(), should.BeEmpty)
		err = updateRecoveryResourceState(ctx, dutName, ufspb.State_STATE_NEEDS_REPAIR)
		assert.Loosely(t, err, should.BeNil)
		lse, err := GetMachineLSE(ctx, dutName)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, lse.GetResourceState(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR))

		t.Run("Update recovery data should update information", func(t *ftt.Test) {
			err = UpdateRecoveryData(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			machine, err = registration.GetMachine(ctx, dutMachine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine.GetSerialNumber(), should.Equal("serialNumber"))
			assert.Loosely(t, machine.GetChromeosMachine().GetHwid(), should.Equal("hwID"))
			assert.Loosely(t, machine.GetChromeosMachine().GetSku(), should.Equal("deviceSku"))
			assert.Loosely(t, machine.GetChromeosMachine().GetDlmSkuId(), should.Equal("12345"))
			lse, err = GetMachineLSE(ctx, dutName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetResourceState(), should.Equal(ufspb.State_STATE_READY))
			asset, err = registration.GetAsset(ctx, dutMachine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.Equal("serialNumber"))
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.Equal("hwID"))
			assert.Loosely(t, asset.GetInfo().GetSku(), should.Equal("deviceSku"))
		})
		t.Run("Update recovery data rejected without perms", func(t *ftt.Test) {
			noPermsCtx := withAuthorizedNoPermsUser(baseCtx)
			err = UpdateRecoveryData(noPermsCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})

}
