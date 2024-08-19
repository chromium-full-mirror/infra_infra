// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"fmt"
	"strings"

	proto "github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/chromiumos/infra/proto/go/manufacturing"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/appengine/cros/lab_inventory/app/config"
	"infra/appengine/cros/lab_inventory/app/external"
	chopsasset "infra/libs/fleet/protos"
	chopsfleet "infra/libs/fleet/protos/go"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufschromeoslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
	ufsutil "infra/unifiedfleet/app/util"
)

// DeviceData holds the invV2 Device and updatetime(of MachineLSE)
type DeviceData struct {
	Device     *lab.ChromeOSDevice
	UpdateTime *timestamppb.Timestamp
}

// DutStateData holds the invV2 DutState and updatetime
type DutStateData struct {
	DutState   *lab.DutState
	UpdateTime *timestamppb.Timestamp
}

// UpdateUFSDutState updates dutmeta, labmeta and dutstate in UFS
func UpdateUFSDutState(ctx context.Context, req *api.UpdateDutsStatusRequest) ([]*api.DeviceOpResult, []*api.DeviceOpResult, error) {
	ufsClient, err := GetUFSClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	ctx = SetupOSNameSpaceContext(ctx)
	dutMetas := req.GetDutMetas()
	labMetas := req.GetLabMetas()
	dutStates := req.GetStates()
	failed := make([]*api.DeviceOpResult, 0, len(dutMetas))
	passed := make([]*api.DeviceOpResult, 0, len(dutMetas))
	for i := range dutMetas {
		req := &ufsapi.ListMachineLSEsRequest{
			PageSize: 1,
			Filter:   fmt.Sprintf("machine=%s", dutMetas[i].GetChromeosDeviceId()),
		}
		res, err := ufsClient.ListMachineLSEs(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "ListMachineLSEs failed for %s", dutMetas[i].GetChromeosDeviceId())
			return nil, nil, err
		}
		if len(res.GetMachineLSEs()) == 0 {
			logging.Errorf(ctx, "No MachineLSE found for %s", dutMetas[i].GetChromeosDeviceId())
			failed = append(failed, &api.DeviceOpResult{
				Id:       dutMetas[i].GetChromeosDeviceId(),
				ErrorMsg: fmt.Sprintf("No MachineLSE found for for %s", dutMetas[i].GetChromeosDeviceId()),
			})
			return nil, failed, nil
		}
		lse := res.GetMachineLSEs()[0]
		lse.Name = ufsutil.RemovePrefix(lse.Name)

		_, err = ufsClient.UpdateDutState(ctx, &ufsapi.UpdateDutStateRequest{
			DutState: CopyInvV2DutStateToUFSDutState(dutStates[i], lse.Name),
			DutMeta:  CopyInvV2DutMetaToUFSDutMeta(dutMetas[i], lse.Name),
			LabMeta:  CopyInvV2LabMetaToUFSLabMeta(labMetas[i], lse.Name),
		})
		if err != nil {
			failed = append(failed, &api.DeviceOpResult{
				Id:       dutMetas[i].GetChromeosDeviceId(),
				Hostname: lse.Name,
				ErrorMsg: err.Error(),
			})
			continue
		}
		passed = append(passed, &api.DeviceOpResult{
			Id:       dutMetas[i].GetChromeosDeviceId(),
			Hostname: lse.Name,
		})
	}
	return passed, failed, nil
}

// GetUFSDevicesByIds Gets MachineLSEs from UFS by Asset id/Machine id.
func GetUFSDevicesByIds(ctx context.Context, ufsClient external.UFSClient, ids []string) ([]*lab.ChromeOSDevice, []*api.DeviceOpResult) {
	ctx = SetupOSNameSpaceContext(ctx)
	failedDevices := make([]*api.DeviceOpResult, 0, len(ids))
	var devices []*lab.ChromeOSDevice
	for _, id := range ids {
		machine, err := ufsClient.GetMachine(ctx, &ufsapi.GetMachineRequest{
			Name: ufsutil.AddPrefix(ufsutil.MachineCollection, id),
		})
		if err != nil {
			logging.Errorf(ctx, "Machine not found for machine ID %s", id)
			failedDevices = append(failedDevices, &api.DeviceOpResult{
				Id:       id,
				ErrorMsg: err.Error(),
			})
			continue
		}
		machine.Name = ufsutil.RemovePrefix(machine.Name)
		req := &ufsapi.ListMachineLSEsRequest{
			PageSize: 1,
			Filter:   fmt.Sprintf("machine=%s", id),
		}
		res, err := ufsClient.ListMachineLSEs(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "ListMachineLSEs failed for %s", id)
			failedDevices = append(failedDevices, &api.DeviceOpResult{
				Id:       id,
				ErrorMsg: err.Error(),
			})
			continue
		}
		if len(res.GetMachineLSEs()) == 0 {
			logging.Errorf(ctx, "MachineLSE not found for machine ID %s", id)
			failedDevices = append(failedDevices, &api.DeviceOpResult{
				Id:       id,
				ErrorMsg: fmt.Sprintf("No MachineLSE found for for %s", id),
			})
			continue
		}
		lse := res.GetMachineLSEs()[0]
		lse.Name = ufsutil.RemovePrefix(lse.Name)
		devices = append(devices, ConstructInvV2Device(machine, lse))
	}
	return devices, failedDevices
}

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

// GetUFSDevicesByModels Gets MachineLSEs from UFS by Asset/Machine model.
func GetUFSDevicesByModels(ctx context.Context, ufsClient external.UFSClient, models []string) ([]*lab.ChromeOSDevice, []*api.DeviceOpResult) {
	ctx = SetupOSNameSpaceContext(ctx)
	var ids []string
	for _, model := range models {
		var pageToken string
		for {
			req := &ufsapi.ListMachinesRequest{
				PageSize:  1000,
				PageToken: pageToken,
				Filter:    fmt.Sprintf("model=%s", model),
			}
			res, err := ufsClient.ListMachines(ctx, req)
			if err != nil {
				logging.Errorf(ctx, "Failed to get MachineLSE for model %s", model)
				break
			}
			if len(res.GetMachines()) == 0 {
				logging.Errorf(ctx, "MachineLSE not found for model %s", model)
				break
			}
			for _, machine := range res.GetMachines() {
				ids = append(ids, ufsutil.RemovePrefix(machine.GetName()))
			}
			pageToken = res.GetNextPageToken()
			if pageToken == "" {
				break
			}
		}
	}
	return GetUFSDevicesByIds(ctx, ufsClient, ids)
}

// GetUFSDutStateForDevices Gets DutStates from UFS by Asset id/Machine id.
func GetUFSDutStateForDevices(ctx context.Context, ufsClient external.UFSClient, devices []*lab.ChromeOSDevice) ([]*api.ExtendedDeviceData, []*api.DeviceOpResult) {
	ctx = SetupOSNameSpaceContext(ctx)
	extendedData := make([]*api.ExtendedDeviceData, 0, len(devices))
	for _, d := range devices {
		dutState, err := ufsClient.GetDutState(ctx, &ufsapi.GetDutStateRequest{
			ChromeosDeviceId: d.GetId().GetValue(),
		})
		if err != nil {
			// Add empty dutstate as the device maybe deployed just now.
			dutState = &ufschromeoslab.DutState{}
		}
		data := &api.ExtendedDeviceData{
			LabConfig: d,
			DutState:  CopyUFSDutStateToInvV2DutState(dutState),
		}
		extendedData = append(extendedData, data)
	}
	return extendedData, nil
}

// GetAllUFSDevicesData Gets all the MachineLSEs and Machines from UFS and returns invV2 Devices and updatedtime.
func GetAllUFSDevicesData(ctx context.Context, ufsClient external.UFSClient) ([]*DeviceData, error) {
	ctx = SetupOSNameSpaceContext(ctx)
	var devicesData []*DeviceData
	idToMachine := make(map[string]*ufspb.Machine, 0)
	for curPageToken := ""; ; {
		req := &ufsapi.ListMachinesRequest{
			PageSize:  1000,
			PageToken: curPageToken,
		}
		res, err := ufsClient.ListMachines(ctx, req)
		if err != nil {
			return nil, errors.Annotate(err, "Failed to get Machines from UFS").Err()
		}
		for _, machine := range res.GetMachines() {
			machine.Name = ufsutil.RemovePrefix(machine.Name)
			idToMachine[machine.GetName()] = machine
		}
		if res.GetNextPageToken() == "" {
			break
		}
		curPageToken = res.GetNextPageToken()
	}
	for curPageToken := ""; ; {
		req := &ufsapi.ListMachineLSEsRequest{
			PageSize:  1000,
			PageToken: curPageToken,
		}
		res, err := ufsClient.ListMachineLSEs(ctx, req)
		if err != nil {
			return nil, errors.Annotate(err, "Failed to get MachineLSEs from UFS").Err()
		}
		for _, lse := range res.GetMachineLSEs() {
			lse.Name = ufsutil.RemovePrefix(lse.Name)
			if len(lse.GetMachines()) == 0 {
				logging.Errorf(ctx, "no Machine in LSE %s", lse.GetName())
				continue
			}
			if machine, found := idToMachine[lse.GetMachines()[0]]; found {
				deviceData := &DeviceData{
					Device:     ConstructInvV2Device(machine, lse),
					UpdateTime: lse.GetUpdateTime(),
				}
				devicesData = append(devicesData, deviceData)
				continue
			}
			logging.Errorf(ctx, "no Machine found %s", lse.GetMachines()[0])
		}
		if res.GetNextPageToken() == "" {
			break
		}
		curPageToken = res.GetNextPageToken()
	}
	return devicesData, nil
}

// GetAllUFSDutStatesData Gets all the DutStateLSEs and DutStates from UFS and returns invV2 DutStates and updatedtime.
func GetAllUFSDutStatesData(ctx context.Context, ufsClient external.UFSClient) ([]*DutStateData, error) {
	ctx = SetupOSNameSpaceContext(ctx)
	var dutStatesData []*DutStateData
	for curPageToken := ""; ; {
		req := &ufsapi.ListDutStatesRequest{
			PageSize:  1000,
			PageToken: curPageToken,
		}
		res, err := ufsClient.ListDutStates(ctx, req)
		if err != nil {
			return nil, errors.Annotate(err, "Failed to get DutStates from UFS").Err()
		}
		for _, dutState := range res.GetDutStates() {
			dutStateData := &DutStateData{
				DutState:   CopyUFSDutStateToInvV2DutState(dutState),
				UpdateTime: dutState.GetUpdateTime(),
			}
			dutStatesData = append(dutStatesData, dutStateData)
		}
		if res.GetNextPageToken() == "" {
			break
		}
		curPageToken = res.GetNextPageToken()
	}
	return dutStatesData, nil
}

// DeleteMachineLSEs removes multiple machine lses from UFS and returns DeleteCrosDevicesResponse. Used to pipe DeleteCrosDevices
// API to UFS.
func DeleteMachineLSEs(iv2ctx context.Context, hosts []*api.DeviceID) *api.DeleteCrosDevicesResponse {
	resp := &api.DeleteCrosDevicesResponse{
		RemovedDevices: []*api.DeviceOpResult{},
		FailedDevices:  []*api.DeviceOpResult{},
	}
	ctx := SetupOSNameSpaceContext(iv2ctx)
	// Create a UFS client
	ufsClient, err := GetUFSClient(ctx)
	if err != nil {
		for _, host := range hosts {
			resp.FailedDevices = append(resp.FailedDevices, &api.DeviceOpResult{
				Hostname: host.GetHostname(),
				Id:       host.GetChromeosDeviceId(),
				ErrorMsg: fmt.Sprintf("DeleteMachineLSEs - [UFS] Failed to delete host. %s", err.Error()),
			})
		}
		return resp
	}
	for _, host := range hosts {
		if host.GetHostname() != "" {
			// Hostname is given. Delete the machine LSE by name.
			_, err := ufsClient.DeleteMachineLSE(ctx, &ufsapi.DeleteMachineLSERequest{
				Name: ufsutil.AddPrefix(ufsutil.MachineLSECollection, host.GetHostname()),
			})
			if err != nil {
				resp.FailedDevices = append(resp.FailedDevices, &api.DeviceOpResult{
					Hostname: host.GetHostname(),
					ErrorMsg: fmt.Sprintf("DeleteMachineLSEs - [UFS] Failed to delete %s: %s", host.GetHostname(), err.Error()),
				})
			} else {
				resp.RemovedDevices = append(resp.RemovedDevices, &api.DeviceOpResult{
					Hostname: host.GetHostname(),
				})
			}
			continue
		}
		if host.GetChromeosDeviceId() != "" {
			// Determine the host name of the device to delete. UFS doesn't delete asset entries for host retirement.
			lmlseResp, err := ufsClient.ListMachineLSEs(ctx, &ufsapi.ListMachineLSEsRequest{
				PageSize: 1,
				Filter:   fmt.Sprintf("machine=%s", host.GetChromeosDeviceId()),
				KeysOnly: true,
			})
			if err != nil {
				resp.FailedDevices = append(resp.FailedDevices, &api.DeviceOpResult{
					Id:       host.GetChromeosDeviceId(),
					ErrorMsg: fmt.Sprintf("DeleteMachineLSEs - [UFS] Failed to determine host through machine ID. %s", err.Error()),
				})
				continue
			}
			// We should only get one machine lse. Log error if we dont.
			if len(lmlseResp.MachineLSEs) != 1 {
				resp.FailedDevices = append(resp.FailedDevices, &api.DeviceOpResult{
					Id:       host.GetChromeosDeviceId(),
					ErrorMsg: fmt.Sprintf("DeleteMachineLSEs - [UFS] Unable to determine host through machine ID. One too many hosts %v", lmlseResp),
				})
				continue
			} else {
				// Delete the host from UFS.
				name := ufsutil.RemovePrefix(lmlseResp.MachineLSEs[0].GetName())
				_, err := ufsClient.DeleteMachineLSE(ctx, &ufsapi.DeleteMachineLSERequest{
					Name: lmlseResp.MachineLSEs[0].GetName(),
				})
				if err != nil {
					resp.FailedDevices = append(resp.FailedDevices, &api.DeviceOpResult{
						Hostname: name,
						Id:       host.GetChromeosDeviceId(),
						ErrorMsg: fmt.Sprintf("DeleteMachineLSEs - [UFS] Failed to delete host %s. %s", name, err.Error()),
					})
					continue
				}
				resp.RemovedDevices = append(resp.RemovedDevices, &api.DeviceOpResult{
					Hostname: name,
					Id:       host.GetChromeosDeviceId(),
				})
			}
		}
	}
	return resp
}

// GetAssets retrieves asset data from UFS. To be used to pipe GetAssets API to UFS.
func GetAssets(iv2ctx context.Context, identifiers []string) *api.AssetResponse {
	resp := &api.AssetResponse{
		Passed: []*api.AssetResult{},
		Failed: []*api.AssetResult{},
	}
	ctx := SetupOSNameSpaceContext(iv2ctx)
	// Create a UFS client
	ufsClient, err := GetUFSClient(ctx)
	if err != nil {
		for _, id := range identifiers {
			resp.Failed = append(resp.Failed, &api.AssetResult{
				Asset: &chopsasset.ChopsAsset{
					Id: id,
				},
				ErrorMsg: fmt.Sprintf("GetAssets - [UFS] Failed to get asset. %s", err.Error()),
			})
		}
		return resp
	}
	for _, id := range identifiers {
		asset, err := ufsClient.GetAsset(ctx, &ufsapi.GetAssetRequest{
			Name: ufsutil.AddPrefix(ufsutil.AssetCollection, id),
		})
		if err != nil {
			resp.Failed = append(resp.Failed, &api.AssetResult{
				Asset: &chopsasset.ChopsAsset{
					Id: id,
				},
				ErrorMsg: fmt.Sprintf("GetAssets - [UFS] Failed to get asset %s. %s", id, err.Error()),
			})
			continue
		}
		// Copy the ufs asset to ChopsAsset
		resp.Passed = append(resp.Passed, &api.AssetResult{
			Asset: &chopsasset.ChopsAsset{
				Id: ufsutil.RemovePrefix(asset.GetName()),
				Location: &chopsfleet.Location{
					Aisle:    asset.GetLocation().GetAisle(),
					Row:      asset.GetLocation().GetRow(),
					Rack:     asset.GetLocation().GetRack(),
					Position: asset.GetLocation().GetPosition(),
					Shelf:    asset.GetLocation().GetShelf(),
					Lab:      asset.GetLocation().GetZone().String(),
				},
			},
		})
	}
	return resp
}

// UpdateAssets updates the asset location to UFS. To be used to pipe UpdateAssets API to UFS.
func UpdateAssets(iv2ctx context.Context, assets []*chopsasset.ChopsAsset) *api.AssetResponse {
	resp := &api.AssetResponse{
		Passed: []*api.AssetResult{},
		Failed: []*api.AssetResult{},
	}
	ctx := SetupOSNameSpaceContext(iv2ctx)
	// Create a UFS client
	ufsClient, err := GetUFSClient(ctx)
	if err != nil {
		for _, asset := range assets {
			resp.Failed = append(resp.Failed, &api.AssetResult{
				Asset:    asset,
				ErrorMsg: fmt.Sprintf("UpdateAssets - [UFS] Failed to update asset. %s", err.Error()),
			})
		}
		return resp
	}
	for _, asset := range assets {
		ufsAsset := &ufspb.Asset{
			Name: ufsutil.AddPrefix(ufsutil.AssetCollection, asset.GetId()),
			Location: &ufspb.Location{
				Aisle:    asset.GetLocation().GetAisle(),
				Row:      asset.GetLocation().GetRow(),
				Position: asset.GetLocation().GetPosition(),
				Shelf:    asset.GetLocation().GetShelf(),
			},
		}
		ufsAsset.Location.Zone = ufsutil.LabToZone(asset.GetLocation().GetLab())
		// Construct rack name as `chromeos[$zone]`-row`$row`-rack`$rack`
		loc := asset.GetLocation()
		var r strings.Builder
		if loc.GetLab() == "" {
			resp.Failed = append(resp.Failed, &api.AssetResult{
				Asset:    asset,
				ErrorMsg: fmt.Sprintf("UpdateAssets - [UFS] Failed to update asset %s. Missing Lab name in Location", asset.GetId()),
			})
			continue
		}
		r.WriteString(loc.GetLab())
		if row := loc.GetRow(); row != "" {
			r.WriteString("-row")
			r.WriteString(row)
		}
		if rack := loc.GetRack(); rack != "" {
			r.WriteString("-rack")
			r.WriteString(rack)
			ufsAsset.Location.RackNumber = rack
		} else {
			// Avoid setting Rack to zone name, e.g. chromeos2
			r.WriteString("-norack")
		}
		ufsAsset.Location.Rack = r.String()

		// UpdateAsset location to UFS.
		_, err := ufsClient.UpdateAsset(ctx, &ufsapi.UpdateAssetRequest{
			Asset: ufsAsset,
			UpdateMask: &field_mask.FieldMask{
				Paths: []string{
					"location.aisle",
					"location.row",
					"location.rack",
					"location.shelf",
					"location.position",
					"location.zone",
					"location.rack_number",
				},
			},
		})
		if err != nil {
			resp.Failed = append(resp.Failed, &api.AssetResult{
				Asset:    asset,
				ErrorMsg: fmt.Sprintf("UpdateAssets - [UFS] Failed to update asset %s. %s", asset.GetId(), err.Error()),
			})
			continue
		}
		resp.Passed = append(resp.Passed, &api.AssetResult{
			Asset: asset,
		})
	}
	return resp
}

// UpdateMachineLSEsBatch impersonates a batch update function. For use in piping BatchUpdateDevices API to UFS.
func UpdateMachineLSEsBatch(iv2ctx context.Context, req *api.BatchUpdateDevicesRequest) (err error) {
	ctx := SetupOSNameSpaceContext(iv2ctx)
	// Create a UFS client
	ufsClient, err := GetUFSClient(ctx)
	if err != nil {
		return errors.Annotate(err, "UpdateMachineLSEsBatch - [UFS] failed to get ufs instance").Err()
	}
	for _, p := range req.GetDeviceProperties() {
		oldlse, err := ufsClient.GetMachineLSE(ctx, &ufsapi.GetMachineLSERequest{
			Name: ufsutil.AddPrefix(ufsutil.MachineLSECollection, p.GetHostname()),
		})
		if err != nil {
			return errors.Annotate(err, "UpdateMachineLSEsBatch - [UFS] failed to get device %s", p.GetHostname()).Err()
		}
		oldlseCopy := proto.Clone(oldlse).(*ufspb.MachineLSE)
		req := &ufsapi.UpdateMachineLSERequest{
			MachineLSE: oldlse,
			UpdateMask: &field_mask.FieldMask{
				Paths: []string{},
			},
		}
		if oldlse.GetChromeosMachineLse().GetDeviceLse().GetDut() != nil {
			if p.GetPool() != "" {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{p.GetPool()}
				req.UpdateMask.Paths = append(req.UpdateMask.Paths, "dut.pools")
			}
			if oldlse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm() == nil {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().Rpm = &ufschromeoslab.OSRPM{}
			}
			if p.GetRpm().GetPowerunitName() != "" {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().PowerunitName = p.GetRpm().GetPowerunitName()
				req.UpdateMask.Paths = append(req.UpdateMask.Paths, "dut.rpm.host")
			}
			if p.GetRpm().GetPowerunitOutlet() != "" {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().PowerunitOutlet = p.GetRpm().GetPowerunitOutlet()
				req.UpdateMask.Paths = append(req.UpdateMask.Paths, "dut.rpm.outlet")
			}
		}
		if oldlse.GetChromeosMachineLse().GetDeviceLse().GetLabstation() != nil {
			if p.GetPool() != "" {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{p.GetPool()}
				req.UpdateMask.Paths = append(req.UpdateMask.Paths, "labstation.pools")
			}
			if oldlse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm() == nil {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Rpm = &ufschromeoslab.OSRPM{}
			}
			if p.GetRpm().GetPowerunitName() != "" {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().PowerunitName = p.GetRpm().GetPowerunitName()
				req.UpdateMask.Paths = append(req.UpdateMask.Paths, "labstation.rpm.host")
			}
			if p.GetRpm().GetPowerunitOutlet() != "" {
				oldlse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().PowerunitOutlet = p.GetRpm().GetPowerunitOutlet()
				req.UpdateMask.Paths = append(req.UpdateMask.Paths, "labstation.rpm.outlet")
			}
		}
		_, err = ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				_, rerr := ufsClient.UpdateMachineLSE(ctx, &ufsapi.UpdateMachineLSERequest{
					MachineLSE: oldlseCopy,
				})
				if rerr != nil {
					logging.Errorf(ctx, "UpdateMachineLSEsBatch - failed to revert %s. %s", oldlseCopy.GetName(), rerr.Error())
				}
			}
		}()
	}
	return err
}

// CopyUFSDutStateToInvV2DutState converts UFS DutState to InvV2 DutState proto format.
func CopyUFSDutStateToInvV2DutState(oldS *ufschromeoslab.DutState) *lab.DutState {
	if oldS == nil {
		return nil
	}
	s := proto.MarshalTextString(oldS)
	var newS lab.DutState
	proto.UnmarshalText(s, &newS)
	return &newS
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

// CopyInvV2DutStateToUFSDutState converts InvV2 DutState to UFS DutState
func CopyInvV2DutStateToUFSDutState(oldS *lab.DutState, hostname string) *ufschromeoslab.DutState {
	if oldS == nil {
		return nil
	}
	s := proto.MarshalTextString(oldS)
	var newS ufschromeoslab.DutState
	proto.UnmarshalText(s, &newS)
	newS.Hostname = hostname
	return &newS
}

// CopyInvV2DutMetaToUFSDutMeta converts InvV2 DutMeta to UFS DutMeta
func CopyInvV2DutMetaToUFSDutMeta(oldDm *api.DutMeta, hostname string) *ufspb.DutMeta {
	if oldDm == nil {
		return nil
	}
	s := proto.MarshalTextString(oldDm)
	var newDm ufspb.DutMeta
	proto.UnmarshalText(s, &newDm)
	newDm.Hostname = hostname
	return &newDm
}

// CopyInvV2LabMetaToUFSLabMeta converts InvV2 LabMeta to UFS LabMeta
func CopyInvV2LabMetaToUFSLabMeta(oldLm *api.LabMeta, hostname string) *ufspb.LabMeta {
	if oldLm == nil {
		return nil
	}
	s := proto.MarshalTextString(oldLm)
	var newLm ufspb.LabMeta
	proto.UnmarshalText(s, &newLm)
	newLm.Hostname = hostname
	return &newLm
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
