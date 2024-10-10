// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	api "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/model/state"
	"infra/unifiedfleet/app/util"
)

func TestUpdateState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Update state", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "hosts/chromeos1-row2-rack3-host4",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			s, err := state.GetStateRecord(osCtx, "hosts/chromeos1-row2-rack3-host4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetResourceName(), should.Equal("hosts/chromeos1-row2-rack3-host4"))
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_RESERVED))
		})
		t.Run("invalid resource prefix", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "resources/chromeos1-row2-rack3-host4",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("empty resource name", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("invalid characters in resource name", func(t *ftt.Test) {
			req := &api.UpdateStateRequest{
				State: &ufspb.StateRecord{
					ResourceName: "hosts/host1@_@",
					State:        ufspb.State_STATE_RESERVED,
				},
			}
			_, err := tf.Fleet.UpdateState(osCtx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
	})
}

func TestGetState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Get state", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			s := &ufspb.StateRecord{
				ResourceName: "hosts/chromeos1-row2-rack3-host4",
				State:        ufspb.State_STATE_RESERVED,
			}
			_, err := state.UpdateStateRecord(ctx, s)
			assert.Loosely(t, err, should.BeNil)
			req := &api.GetStateRequest{
				ResourceName: "hosts/chromeos1-row2-rack3-host4",
			}
			res, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Resemble(s))
		})
		t.Run("valid resource name, but not found", func(t *ftt.Test) {
			res, err := tf.Fleet.GetState(ctx, &api.GetStateRequest{
				ResourceName: "hosts/chromeos-fakehost",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("invalid resource prefix", func(t *ftt.Test) {
			req := &api.GetStateRequest{
				ResourceName: "resources/chromeos1-row2-rack3-host4",
			}
			_, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("empty resource name", func(t *ftt.Test) {
			req := &api.GetStateRequest{
				ResourceName: "",
			}
			_, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
		t.Run("invalid characters in resource name", func(t *ftt.Test) {
			req := &api.GetStateRequest{
				ResourceName: "hosts/host1@_@",
			}
			_, err := tf.Fleet.GetState(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.ResourceFormat))
		})
	})
}

func TestUpdateDutState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	dutStateGood := &chromeosLab.DutState{
		Id:       &chromeosLab.ChromeOSDeviceID{Value: "UUID:01"},
		Hostname: "hostname-01",
	}
	ftt.Run("Update dut state", t, func(t *ftt.Test) {
		t.Run("empty dut ID", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.EmptyID))
		})

		t.Run("dut ID with all spaces", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id: &chromeosLab.ChromeOSDeviceID{Value: "   "},
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(api.EmptyID))
		})

		t.Run("empty hostname", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id:       &chromeosLab.ChromeOSDeviceID{Value: "UUID:01"},
					Hostname: "   ",
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Hostname cannot be empty"))
		})

		t.Run("non-matched dut ID in lab meta", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: dutStateGood,
				LabMeta: &ufspb.LabMeta{
					ChromeosDeviceId: "UUID:wrong",
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Mismatched dut ID"))
		})

		t.Run("non-matched dut hostname in lab meta", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: dutStateGood,
				LabMeta: &ufspb.LabMeta{
					ChromeosDeviceId: "UUID:01",
					Hostname:         "hostname-wrong",
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Mismatched dut hostname"))
		})

		t.Run("non-matched dut ID in dut meta", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: dutStateGood,
				DutMeta: &ufspb.DutMeta{
					ChromeosDeviceId: "UUID:wrong",
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Mismatched dut ID"))
		})

		t.Run("non-matched dut hostname in dut meta", func(t *ftt.Test) {
			_, err := tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: dutStateGood,
				DutMeta: &ufspb.DutMeta{
					ChromeosDeviceId: "UUID:01",
					Hostname:         "hostname-wrong",
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Mismatched dut hostname"))
		})

		t.Run("happy path with no data", func(t *ftt.Test) {
			err := mockOSMachineAssetAndHost(ctx, "rpc-dutstate-id1", "rpc-dutstate-host1", "dut")
			assert.Loosely(t, err, should.BeNil)

			// Use osCtx as we will restrict ctx to include namespace in prod.
			_, err = tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id:       &chromeosLab.ChromeOSDeviceID{Value: "rpc-dutstate-id1"},
					Hostname: "rpc-dutstate-host1",
				},
			})
			assert.Loosely(t, err, should.BeNil)

			m, err := registration.GetMachine(osCtx, "rpc-dutstate-id1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, m.GetChromeosMachine().GetSku(), should.BeEmpty)
			a, err := registration.GetAsset(osCtx, "rpc-dutstate-id1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.GetInfo().GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, a.GetInfo().GetSku(), should.BeEmpty)
			lse, err := inventory.GetMachineLSE(osCtx, "rpc-dutstate-host1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoType(), should.BeEmpty)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoTopology(), should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
		})

		t.Run("happy path with dut meta", func(t *ftt.Test) {
			err := mockOSMachineAssetAndHost(ctx, "rpc-dutstate-id2", "rpc-dutstate-host2", "dut")
			assert.Loosely(t, err, should.BeNil)
			// Use osCtx as we will restrict ctx to include namespace in prod.
			_, err = tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id:       &chromeosLab.ChromeOSDeviceID{Value: "rpc-dutstate-id2"},
					Hostname: "rpc-dutstate-host2",
				},
				DutMeta: &ufspb.DutMeta{
					ChromeosDeviceId: "rpc-dutstate-id2",
					Hostname:         "rpc-dutstate-host2",
					SerialNumber:     "real-serial",
					HwID:             "real-hwid",
					DeviceSku:        "real-sku",
				},
			})
			assert.Loosely(t, err, should.BeNil)

			m, err := registration.GetMachine(osCtx, "rpc-dutstate-id2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.GetSerialNumber(), should.Equal("real-serial"))
			assert.Loosely(t, m.GetChromeosMachine().GetSku(), should.Equal("real-sku"))
			assert.Loosely(t, m.GetChromeosMachine().GetHwid(), should.Equal("real-hwid"))
			a, err := registration.GetAsset(osCtx, "rpc-dutstate-id2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.GetInfo().GetSerialNumber(), should.Equal("real-serial"))
			assert.Loosely(t, a.GetInfo().GetSku(), should.Equal("real-sku"))
			assert.Loosely(t, a.GetInfo().GetHwid(), should.Equal("real-hwid"))
			lse, err := inventory.GetMachineLSE(osCtx, "rpc-dutstate-host2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoType(), should.BeEmpty)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoTopology(), should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
		})

		t.Run("happy path with lab meta", func(t *ftt.Test) {
			err := mockOSMachineAssetAndHost(ctx, "rpc-dutstate-id3", "rpc-dutstate-host3", "dut")
			assert.Loosely(t, err, should.BeNil)
			topology := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{
					Type:         "servo_v4",
					Serial:       "SomeSerial",
					SysfsProduct: "1-4.6.5",
				},
			}
			// Use osCtx as we will restrict ctx to include namespace in prod.
			_, err = tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id:       &chromeosLab.ChromeOSDeviceID{Value: "rpc-dutstate-id3"},
					Hostname: "rpc-dutstate-host3",
				},
				LabMeta: &ufspb.LabMeta{
					ChromeosDeviceId: "rpc-dutstate-id3",
					Hostname:         "rpc-dutstate-host3",
					ServoType:        "servo_v4_with_ccd_cr50",
					ServoTopology:    topology,
					SmartUsbhub:      true,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			m, err := registration.GetMachine(osCtx, "rpc-dutstate-id3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, m.GetChromeosMachine().GetSku(), should.BeEmpty)
			a, err := registration.GetAsset(osCtx, "rpc-dutstate-id3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.GetInfo().GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, a.GetInfo().GetSku(), should.BeEmpty)
			lse, err := inventory.GetMachineLSE(osCtx, "rpc-dutstate-host3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoType(), should.Equal("servo_v4_with_ccd_cr50"))
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoTopology(), should.Resemble(topology))
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeTrue)
		})

		t.Run("only dut meta update for labstation", func(t *ftt.Test) {
			err := mockOSMachineAssetAndHost(ctx, "rpc-dutstate-id4", "rpc-dutstate-host4", "labstation")
			assert.Loosely(t, err, should.BeNil)
			// Use osCtx as we will restrict ctx to include namespace in prod.
			_, err = tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id:       &chromeosLab.ChromeOSDeviceID{Value: "rpc-dutstate-id4"},
					Hostname: "rpc-dutstate-host4",
				},
				DutMeta: &ufspb.DutMeta{
					ChromeosDeviceId: "rpc-dutstate-id4",
					Hostname:         "rpc-dutstate-host4",
					SerialNumber:     "real-serial",
				},
				LabMeta: &ufspb.LabMeta{
					ChromeosDeviceId: "rpc-dutstate-id4",
					Hostname:         "rpc-dutstate-host4",
					ServoType:        "servo_v4_with_ccd_cr50",
					SmartUsbhub:      true,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			m, err := registration.GetMachine(osCtx, "rpc-dutstate-id4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.GetSerialNumber(), should.Equal("real-serial"))
			a, err := registration.GetAsset(osCtx, "rpc-dutstate-id4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.GetInfo().GetSerialNumber(), should.Equal("real-serial"))
			lse, err := inventory.GetMachineLSE(osCtx, "rpc-dutstate-host4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoType(), should.BeEmpty)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoTopology(), should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
		})

		t.Run("no update for chrome device", func(t *ftt.Test) {
			err := mockOSMachineAssetAndHost(ctx, "rpc-dutstate-id5", "rpc-dutstate-host5", "browser")
			assert.Loosely(t, err, should.BeNil)
			// Use osCtx as we will restrict ctx to include namespace in prod.
			_, err = tf.Fleet.UpdateDutState(osCtx, &api.UpdateDutStateRequest{
				DutState: &chromeosLab.DutState{
					Id:       &chromeosLab.ChromeOSDeviceID{Value: "rpc-dutstate-id5"},
					Hostname: "rpc-dutstate-host5",
				},
				DutMeta: &ufspb.DutMeta{
					ChromeosDeviceId: "rpc-dutstate-id5",
					Hostname:         "rpc-dutstate-host5",
					SerialNumber:     "real-serial",
				},
				LabMeta: &ufspb.LabMeta{
					ChromeosDeviceId: "rpc-dutstate-id5",
					Hostname:         "rpc-dutstate-host5",
					ServoType:        "servo_v4_with_ccd_cr50",
					SmartUsbhub:      true,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			m, err := registration.GetMachine(osCtx, "rpc-dutstate-id5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.GetSerialNumber(), should.BeEmpty)
			_, err = registration.GetAsset(osCtx, "rpc-dutstate-id5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("not found"))
			lse, err := inventory.GetMachineLSE(osCtx, "rpc-dutstate-host5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoType(), should.BeEmpty)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoTopology(), should.BeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
		})
	})
}

func mockOSMachineAssetAndHost(ctx context.Context, id, hostname, deviceType string) error {
	osCtx, err := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	if err != nil {
		return err
	}
	var machineLSE1 *ufspb.MachineLSE
	var machine *ufspb.Machine
	var asset *ufspb.Asset
	switch deviceType {
	case "dut":
		machineLSE1 = &ufspb.MachineLSE{
			Name:     hostname,
			Hostname: hostname,
			Machines: []string{id},
			Lse: &ufspb.MachineLSE_ChromeosMachineLse{
				ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
					ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
						DeviceLse: &ufspb.ChromeOSDeviceLSE{
							Device: &ufspb.ChromeOSDeviceLSE_Dut{
								Dut: &chromeosLab.DeviceUnderTest{
									Hostname: hostname,
									Peripherals: &chromeosLab.Peripherals{
										Servo: &chromeosLab.Servo{},
									},
								},
							},
						},
					},
				},
			},
		}
		machine = &ufspb.Machine{
			Name: id,
			Device: &ufspb.Machine_ChromeosMachine{
				ChromeosMachine: &ufspb.ChromeOSMachine{},
			},
		}
		asset = &ufspb.Asset{
			Name: id,
			Info: &ufspb.AssetInfo{
				AssetTag: id,
			},
			Type:     ufspb.AssetType_DUT,
			Location: &ufspb.Location{},
		}
	case "labstation":
		machineLSE1 = &ufspb.MachineLSE{
			Name:     hostname,
			Hostname: hostname,
			Machines: []string{id},
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
		machine = &ufspb.Machine{
			Name: id,
			Device: &ufspb.Machine_ChromeosMachine{
				ChromeosMachine: &ufspb.ChromeOSMachine{},
			},
		}
		asset = &ufspb.Asset{
			Name: id,
			Info: &ufspb.AssetInfo{
				AssetTag: id,
			},
			Type:     ufspb.AssetType_LABSTATION,
			Location: &ufspb.Location{},
		}
	case "browser":
		machineLSE1 = &ufspb.MachineLSE{
			Name:     hostname,
			Hostname: hostname,
			Machines: []string{id},
			Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
				ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
			},
		}
		machine = &ufspb.Machine{
			Name: id,
			Device: &ufspb.Machine_ChromeBrowserMachine{
				ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
			},
		}
	}

	if _, err := registration.CreateMachine(osCtx, machine); err != nil {
		return err
	}
	if asset != nil {
		if _, err := registration.CreateAsset(osCtx, asset); err != nil {
			return err
		}
	}
	if _, err := inventory.CreateMachineLSE(osCtx, machineLSE1); err != nil {
		return err
	}
	return nil
}
