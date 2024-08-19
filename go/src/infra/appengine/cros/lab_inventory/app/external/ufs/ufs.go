// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"fmt"
	"strings"

	proto "github.com/golang/protobuf/proto"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/chromiumos/infra/proto/go/manufacturing"
	"go.chromium.org/luci/common/logging"

	api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/appengine/cros/lab_inventory/app/config"
	"infra/appengine/cros/lab_inventory/app/external"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufschromeoslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
	ufsutil "infra/unifiedfleet/app/util"
)

// GetUFSDevicesByHostnames Gets MachineLSEs from UFS by MachineLSE name/hostname.
func GetUFSDevicesByHostnames(ctx context.Context, ufsClient external.UFSClient, names []string) ([]*lab.ChromeOSDevice, []*api.DeviceOpResult) {
	ctx = SetupOSNameSpaceContext(ctx)
	failedDevices := make([]*api.DeviceOpResult, 0, len(names))
	var devices []*lab.ChromeOSDevice
	for _, name := range names {
		lse, err := ufsClient.GetMachineLSE(ctx, &ufsapi.GetMachineLSERequest{
			Name: ufsutil.AddPrefix(ufsutil.MachineLSECollection, name),
		})
		if err != nil {
			logging.Errorf(ctx, "MachineLSE not found for hostname %s", name)
			failedDevices = append(failedDevices, &api.DeviceOpResult{
				Hostname: name,
				ErrorMsg: err.Error(),
			})
			continue
		}
		lse.Name = ufsutil.RemovePrefix(lse.Name)

		if len(lse.GetMachines()) == 0 {
			logging.Errorf(ctx, "Machine not found for hostname %s", name)
			failedDevices = append(failedDevices, &api.DeviceOpResult{
				Hostname: lse.GetName(),
				ErrorMsg: fmt.Sprintf("Machine not found for hostname %s", name),
			})
			continue
		}
		machine, err := ufsClient.GetMachine(ctx, &ufsapi.GetMachineRequest{
			Name: ufsutil.AddPrefix(ufsutil.MachineCollection, lse.GetMachines()[0]),
		})
		if err != nil {
			logging.Errorf(ctx, "Machine not found for machine ID %s", lse.GetMachines()[0])
			failedDevices = append(failedDevices, &api.DeviceOpResult{
				Id:       lse.GetMachines()[0],
				Hostname: lse.GetName(),
				ErrorMsg: err.Error(),
			})
			continue
		}
		machine.Name = ufsutil.RemovePrefix(machine.Name)
		devices = append(devices, ConstructInvV2Device(machine, lse))
	}
	return devices, failedDevices
}

// CopyUFSDutToInvV2Dut converts UFS DUT to InvV2 DUT proto format.
func CopyUFSDutToInvV2Dut(dut *ufschromeoslab.DeviceUnderTest) *lab.DeviceUnderTest {
	if dut == nil {
		return nil
	}
	s := proto.MarshalTextString(dut)
	var newDUT lab.DeviceUnderTest
	proto.UnmarshalText(s, &newDUT)
	return &newDUT
}

// CopyUFSLabstationToInvV2Labstation converts UFS Labstation to InvV2 Labstation proto format.
func CopyUFSLabstationToInvV2Labstation(labstation *ufschromeoslab.Labstation) *lab.Labstation {
	if labstation == nil {
		return nil
	}
	s := proto.MarshalTextString(labstation)
	var newL lab.Labstation
	proto.UnmarshalText(s, &newL)
	return &newL
}

func getDeviceConfigIDFromMachine(machine *ufspb.Machine) *device.ConfigId {
	buildTarget := strings.ToLower(machine.GetChromeosMachine().GetBuildTarget())
	model := strings.ToLower(machine.GetChromeosMachine().GetModel())
	devConfigID := &device.ConfigId{
		PlatformId: &device.PlatformId{
			Value: buildTarget,
		},
		ModelId: &device.ModelId{
			Value: model,
		},
	}
	sku := strings.ToLower(machine.GetChromeosMachine().GetSku())
	if sku != "" {
		devConfigID.VariantId = &device.VariantId{
			Value: sku,
		}
	}
	return devConfigID
}

// ConstructInvV2Device constructs a InvV2 Device from UFs MachineLSE and Machine.
func ConstructInvV2Device(machine *ufspb.Machine, lse *ufspb.MachineLSE) *lab.ChromeOSDevice {
	crosDevice := &lab.ChromeOSDevice{
		Id:              &lab.ChromeOSDeviceID{Value: machine.GetName()},
		SerialNumber:    machine.GetSerialNumber(),
		ManufacturingId: &manufacturing.ConfigID{Value: machine.GetChromeosMachine().GetHwid()},
		DeviceConfigId:  getDeviceConfigIDFromMachine(machine),
	}
	if lse.GetChromeosMachineLse().GetDeviceLse().GetDut() != nil {
		crosDevice.Device = &lab.ChromeOSDevice_Dut{
			Dut: CopyUFSDutToInvV2Dut(lse.GetChromeosMachineLse().GetDeviceLse().GetDut()),
		}
	} else {
		crosDevice.Device = &lab.ChromeOSDevice_Labstation{
			Labstation: CopyUFSLabstationToInvV2Labstation(lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation()),
		}
	}
	return crosDevice
}

// GetUFSClient gets the UFS clien.
func GetUFSClient(ctx context.Context) (external.UFSClient, error) {
	es, err := external.GetServerInterface(ctx)
	if err != nil {
		return nil, err
	}
	return es.NewUFSInterfaceFactory(ctx, config.Get(ctx).GetUfsService())
}

// SetupOSNameSpaceContext sets up context with namespace
func SetupOSNameSpaceContext(ctx context.Context) context.Context {
	md := metadata.Pairs(ufsutil.Namespace, ufsutil.OSNamespace)
	return metadata.NewOutgoingContext(ctx, md)
}
