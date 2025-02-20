// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"strings"

	empty "github.com/golang/protobuf/ptypes/empty"

	"go.chromium.org/luci/grpc/grpcutil"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// RackRegistration creates rack, switches, kvms, rpms in database.
func (fs *FleetServerImpl) RackRegistration(ctx context.Context, req *ufsAPI.RackRegistrationRequest) (rsp *ufspb.Rack, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	for _, kvm := range req.GetRack().GetChromeBrowserRack().GetKvmObjects() {
		if kvm != nil {
			kvm.Name = util.FormatDHCPHostname(util.RemovePrefix(kvm.Name))
		}
	}
	for _, rpm := range req.GetRack().GetChromeBrowserRack().GetRpmObjects() {
		if rpm != nil {
			rpm.Name = util.FormatDHCPHostname(util.RemovePrefix(rpm.Name))
		}
	}
	rack, err := controller.RackRegistration(ctx, req.Rack)
	if err != nil {
		return nil, err
	}
	return rack, err
}

// MachineRegistration creates machine/nics/drac entry in database.
func (fs *FleetServerImpl) MachineRegistration(ctx context.Context, req *ufsAPI.MachineRegistrationRequest) (rsp *ufspb.Machine, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if drac := req.GetMachine().GetChromeBrowserMachine().GetDracObject(); drac != nil {
		drac.Name = util.FormatDHCPHostname(util.RemovePrefix(drac.Name))
	}
	machine, err := controller.MachineRegistration(ctx, req.Machine)
	if err != nil {
		return nil, err
	}
	return machine, err
}

// UpdateMachine updates the machine information in database.
func (fs *FleetServerImpl) UpdateMachine(ctx context.Context, req *ufsAPI.UpdateMachineRequest) (rsp *ufspb.Machine, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Machine.Name = util.RemovePrefix(req.Machine.Name)
	machine, err := controller.UpdateMachine(ctx, req.Machine, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	machine.Name = util.AddPrefix(util.MachineCollection, machine.Name)
	return machine, err
}

// GetMachine gets the machine information from database.
func (fs *FleetServerImpl) GetMachine(ctx context.Context, req *ufsAPI.GetMachineRequest) (rsp *ufspb.Machine, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	machine, err := controller.GetMachine(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	machine.Name = util.AddPrefix(util.MachineCollection, machine.Name)
	return machine, err
}

// BatchGetMachines gets a batch of machines in batch from database.
func (fs *FleetServerImpl) BatchGetMachines(ctx context.Context, req *ufsAPI.BatchGetMachinesRequest) (rsp *ufsAPI.BatchGetMachinesResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	machines, err := controller.BatchGetMachines(ctx, util.FormatInputNames(req.GetNames()))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, v := range machines {
		v.Name = util.AddPrefix(util.MachineCollection, v.Name)
	}
	return &ufsAPI.BatchGetMachinesResponse{
		Machines: machines,
	}, nil
}

// ListMachines list the machines information from database.
func (fs *FleetServerImpl) ListMachines(ctx context.Context, req *ufsAPI.ListMachinesRequest) (rsp *ufsAPI.ListMachinesResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListMachines(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly, req.Full)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, machine := range result {
		machine.Name = util.AddPrefix(util.MachineCollection, machine.Name)
	}
	return &ufsAPI.ListMachinesResponse{
		Machines:      result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteMachine deletes the machine from database.
func (fs *FleetServerImpl) DeleteMachine(ctx context.Context, req *ufsAPI.DeleteMachineRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	err = controller.DeleteMachine(ctx, name)
	return &empty.Empty{}, err
}

// RenameMachine renames the machine in database.
func (fs *FleetServerImpl) RenameMachine(ctx context.Context, req *ufsAPI.RenameMachineRequest) (rsp *ufspb.Machine, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return
	}
	rsp, err = controller.RenameMachine(ctx, util.RemovePrefix(req.Name), util.RemovePrefix(req.NewName))
	if err != nil {
		return
	}
	// https://aip.dev/122 - as per AIP guideline
	rsp.Name = util.AddPrefix(util.MachineCollection, rsp.Name)
	return
}

// UpdateRack updates the rack information in database.
func (fs *FleetServerImpl) UpdateRack(ctx context.Context, req *ufsAPI.UpdateRackRequest) (rsp *ufspb.Rack, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Rack.Name = util.RemovePrefix(req.Rack.Name)
	rack, err := controller.UpdateRack(ctx, req.Rack, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	rack.Name = util.AddPrefix(util.RackCollection, rack.Name)
	return rack, err
}

// GetRack gets the rack information from database.
func (fs *FleetServerImpl) GetRack(ctx context.Context, req *ufsAPI.GetRackRequest) (rsp *ufspb.Rack, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	var rack *ufspb.Rack
	rack, err = controller.GetRack(ctx, name)
	if err != nil {
		if strings.ToLower(name) != name {
			rack, err = controller.GetRack(ctx, strings.ToLower(name))
		}
		if err != nil {
			return nil, err
		}
	}
	// https://aip.dev/122 - as per AIP guideline
	rack.Name = util.AddPrefix(util.RackCollection, rack.Name)
	return rack, err
}

// BatchGetRacks gets racks in batch from database.
func (fs *FleetServerImpl) BatchGetRacks(ctx context.Context, req *ufsAPI.BatchGetRacksRequest) (rsp *ufsAPI.BatchGetRacksResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	racks, err := controller.BatchGetRacks(ctx, util.FormatInputNames(req.GetNames()))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, v := range racks {
		v.Name = util.AddPrefix(util.RackCollection, v.Name)
	}
	return &ufsAPI.BatchGetRacksResponse{
		Racks: racks,
	}, nil
}

// ListRacks list the racks information from database.
func (fs *FleetServerImpl) ListRacks(ctx context.Context, req *ufsAPI.ListRacksRequest) (rsp *ufsAPI.ListRacksResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListRacks(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly, req.Full)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, rack := range result {
		rack.Name = util.AddPrefix(util.RackCollection, rack.Name)
	}
	return &ufsAPI.ListRacksResponse{
		Racks:         result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteRack deletes the rack from database.
func (fs *FleetServerImpl) DeleteRack(ctx context.Context, req *ufsAPI.DeleteRackRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	err = controller.DeleteRack(ctx, name)
	return &empty.Empty{}, err
}

// RenameRack rename a rack name to a new one.
func (fs *FleetServerImpl) RenameRack(ctx context.Context, req *ufsAPI.RenameRackRequest) (rsp *ufspb.Rack, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return
	}
	rsp, err = controller.RenameRack(ctx, util.RemovePrefix(req.Name), util.RemovePrefix(req.NewName))
	if err != nil {
		return
	}
	// https://aip.dev/122 - as per AIP guideline
	rsp.Name = util.AddPrefix(util.RackCollection, rsp.Name)
	return
}

// CreateNic creates nic entry in database.
func (fs *FleetServerImpl) CreateNic(ctx context.Context, req *ufsAPI.CreateNicRequest) (rsp *ufspb.Nic, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Nic.Name = req.NicId
	nic, err := controller.CreateNic(ctx, req.Nic)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	nic.Name = util.AddPrefix(util.NicCollection, nic.Name)
	return nic, err
}

// UpdateNic updates the nic information in database.
func (fs *FleetServerImpl) UpdateNic(ctx context.Context, req *ufsAPI.UpdateNicRequest) (rsp *ufspb.Nic, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Nic.Name = util.RemovePrefix(req.Nic.Name)
	nic, err := controller.UpdateNic(ctx, req.Nic, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	nic.Name = util.AddPrefix(util.NicCollection, nic.Name)
	return nic, err
}

// GetNic gets the nic information from database.
func (fs *FleetServerImpl) GetNic(ctx context.Context, req *ufsAPI.GetNicRequest) (rsp *ufspb.Nic, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	nic, err := controller.GetNic(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	nic.Name = util.AddPrefix(util.NicCollection, nic.Name)
	return nic, err
}

// BatchGetNics gets nics in batch from database.
func (fs *FleetServerImpl) BatchGetNics(ctx context.Context, req *ufsAPI.BatchGetNicsRequest) (rsp *ufsAPI.BatchGetNicsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	nics, err := controller.BatchGetNics(ctx, util.FormatInputNames(req.GetNames()))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, v := range nics {
		v.Name = util.AddPrefix(util.NicCollection, v.Name)
	}
	return &ufsAPI.BatchGetNicsResponse{
		Nics: nics,
	}, nil
}

// ListNics list the nics information from database.
func (fs *FleetServerImpl) ListNics(ctx context.Context, req *ufsAPI.ListNicsRequest) (rsp *ufsAPI.ListNicsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListNics(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, nic := range result {
		nic.Name = util.AddPrefix(util.NicCollection, nic.Name)
	}
	return &ufsAPI.ListNicsResponse{
		Nics:          result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteNic deletes the nic from database.
func (fs *FleetServerImpl) DeleteNic(ctx context.Context, req *ufsAPI.DeleteNicRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	err = controller.DeleteNic(ctx, name)
	return &empty.Empty{}, err
}

// RenameNic renames the nic in database.
func (fs *FleetServerImpl) RenameNic(ctx context.Context, req *ufsAPI.RenameNicRequest) (rsp *ufspb.Nic, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return
	}
	rsp, err = controller.RenameNic(ctx, util.RemovePrefix(req.Name), util.RemovePrefix(req.NewName))
	if err != nil {
		return
	}
	// https://aip.dev/122 - as per AIP guideline
	rsp.Name = util.AddPrefix(util.NicCollection, rsp.Name)
	return
}

// CreateKVM creates kvm entry in database.
func (fs *FleetServerImpl) CreateKVM(ctx context.Context, req *ufsAPI.CreateKVMRequest) (rsp *ufspb.KVM, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.KVM.Name = util.FormatDHCPHostname(req.KVMId)
	kvm, err := controller.CreateKVM(ctx, req.KVM)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
	return kvm, err
}

// UpdateKVM updates the kvm information in database.
func (fs *FleetServerImpl) UpdateKVM(ctx context.Context, req *ufsAPI.UpdateKVMRequest) (rsp *ufspb.KVM, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.KVM.Name = util.FormatDHCPHostname(util.RemovePrefix(req.KVM.Name))

	if req.GetNetworkOption() != nil &&
		(req.GetNetworkOption().GetDelete() || req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "") {
		var kvm *ufspb.KVM
		var err error
		if req.UpdateMask != nil && len(req.UpdateMask.Paths) > 0 {
			kvm, err = controller.UpdateKVM(ctx, req.KVM, req.UpdateMask)
			if err != nil {
				return nil, err
			}
		} else {
			kvm, err = controller.GetKVM(ctx, req.KVM.Name)
			if err != nil {
				return nil, err
			}
		}

		// If network_option.delete is enabled, ignore network_option.vlan and return directly
		if req.GetNetworkOption().GetDelete() {
			if err = controller.DeleteKVMHost(ctx, req.KVM.Name); err != nil {
				return nil, err
			}
		}

		if req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "" {
			if err = controller.UpdateKVMHost(ctx, kvm, req.GetNetworkOption()); err != nil {
				return nil, err
			}
		}

		// https://aip.dev/122 - as per AIP guideline
		kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
		return kvm, nil
	}

	kvm, err := controller.UpdateKVM(ctx, req.KVM, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
	return kvm, err
}

// GetKVM gets the kvm information from database.
func (fs *FleetServerImpl) GetKVM(ctx context.Context, req *ufsAPI.GetKVMRequest) (rsp *ufspb.KVM, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.FormatDHCPHostname(util.RemovePrefix(req.Name))
	kvm, err := controller.GetKVM(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
	return kvm, err
}

// BatchGetKVMs gets the kvm information in batch from database.
func (fs *FleetServerImpl) BatchGetKVMs(ctx context.Context, req *ufsAPI.BatchGetKVMsRequest) (rsp *ufsAPI.BatchGetKVMsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	kvms, err := controller.BatchGetKVMs(ctx, util.FormatDHCPHostnames(util.FormatInputNames(req.GetNames())))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, kvm := range kvms {
		kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
	}
	return &ufsAPI.BatchGetKVMsResponse{
		KVMs: kvms,
	}, nil
}

// ListKVMs list the kvms information from database.
func (fs *FleetServerImpl) ListKVMs(ctx context.Context, req *ufsAPI.ListKVMsRequest) (rsp *ufsAPI.ListKVMsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListKVMs(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, kvm := range result {
		kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
	}
	return &ufsAPI.ListKVMsResponse{
		KVMs:          result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteKVM deletes the kvm from database.
func (fs *FleetServerImpl) DeleteKVM(ctx context.Context, req *ufsAPI.DeleteKVMRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.FormatDHCPHostname(util.RemovePrefix(req.Name))
	err = controller.DeleteKVM(ctx, name)
	return &empty.Empty{}, err
}

// CreateRPM creates rpm entry in database.
func (fs *FleetServerImpl) CreateRPM(ctx context.Context, req *ufsAPI.CreateRPMRequest) (rsp *ufspb.RPM, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.RPM.Name = util.FormatDHCPHostname(req.RPMId)
	rpm, err := controller.CreateRPM(ctx, req.RPM)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	rpm.Name = util.AddPrefix(util.RPMCollection, rpm.Name)
	return rpm, err
}

// UpdateRPM updates the rpm information in database.
func (fs *FleetServerImpl) UpdateRPM(ctx context.Context, req *ufsAPI.UpdateRPMRequest) (rsp *ufspb.RPM, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.RPM.Name = util.FormatDHCPHostname(util.RemovePrefix(req.RPM.Name))

	if req.GetNetworkOption() != nil &&
		(req.GetNetworkOption().GetDelete() || req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "") {
		var rpm *ufspb.RPM
		var err error
		if req.UpdateMask != nil && len(req.UpdateMask.Paths) > 0 {
			rpm, err = controller.UpdateRPM(ctx, req.RPM, req.UpdateMask)
			if err != nil {
				return nil, err
			}
		} else {
			rpm, err = controller.GetRPM(ctx, req.RPM.Name)
			if err != nil {
				return nil, err
			}
		}

		// If network_option.delete is enabled, ignore network_option.vlan and return directly
		if req.GetNetworkOption().GetDelete() {
			if err = controller.DeleteRPMHost(ctx, req.RPM.Name); err != nil {
				return nil, err
			}
		}

		if req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "" {
			if err = controller.UpdateRPMHost(ctx, rpm, req.GetNetworkOption()); err != nil {
				return nil, err
			}
		}

		// https://aip.dev/122 - as per AIP guideline
		rpm.Name = util.AddPrefix(util.RPMCollection, rpm.Name)
		return rpm, nil
	}

	rpm, err := controller.UpdateRPM(ctx, req.RPM, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	rpm.Name = util.AddPrefix(util.RPMCollection, rpm.Name)
	return rpm, err
}

// GetRPM gets the rpm information from database.
func (fs *FleetServerImpl) GetRPM(ctx context.Context, req *ufsAPI.GetRPMRequest) (rsp *ufspb.RPM, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.FormatDHCPHostname(util.RemovePrefix(req.Name))
	rpm, err := controller.GetRPM(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	rpm.Name = util.AddPrefix(util.RPMCollection, rpm.Name)
	return rpm, err
}

// BatchGetRPMs gets rpms in batch from database.
func (fs *FleetServerImpl) BatchGetRPMs(ctx context.Context, req *ufsAPI.BatchGetRPMsRequest) (rsp *ufsAPI.BatchGetRPMsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	rpms, err := controller.BatchGetRPMs(ctx, util.FormatDHCPHostnames(util.FormatInputNames(req.GetNames())))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, v := range rpms {
		v.Name = util.AddPrefix(util.RPMCollection, v.Name)
	}
	return &ufsAPI.BatchGetRPMsResponse{
		Rpms: rpms,
	}, nil
}

// ListRPMs list the rpms information from database.
func (fs *FleetServerImpl) ListRPMs(ctx context.Context, req *ufsAPI.ListRPMsRequest) (rsp *ufsAPI.ListRPMsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListRPMs(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, rpm := range result {
		rpm.Name = util.AddPrefix(util.RPMCollection, rpm.Name)
	}
	return &ufsAPI.ListRPMsResponse{
		RPMs:          result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteRPM deletes the rpm from database.
func (fs *FleetServerImpl) DeleteRPM(ctx context.Context, req *ufsAPI.DeleteRPMRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.FormatDHCPHostname(util.RemovePrefix(req.Name))
	err = controller.DeleteRPM(ctx, name)
	return &empty.Empty{}, err
}

// CreateDrac creates drac entry in database.
func (fs *FleetServerImpl) CreateDrac(ctx context.Context, req *ufsAPI.CreateDracRequest) (rsp *ufspb.Drac, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Drac.Name = util.FormatDHCPHostname(req.DracId)
	drac, err := controller.CreateDrac(ctx, req.Drac)
	if err != nil {
		return nil, err
	}
	if req.GetNetworkOption() != nil && (req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "") {
		if err = controller.UpdateDracHost(ctx, drac, req.GetNetworkOption()); err != nil {
			return nil, err
		}
	}
	// https://aip.dev/122 - as per AIP guideline
	drac.Name = util.AddPrefix(util.DracCollection, drac.Name)
	return drac, err
}

// UpdateDrac updates the drac information in database.
func (fs *FleetServerImpl) UpdateDrac(ctx context.Context, req *ufsAPI.UpdateDracRequest) (rsp *ufspb.Drac, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Drac.Name = util.FormatDHCPHostname(util.RemovePrefix(req.Drac.Name))

	if req.GetNetworkOption() != nil &&
		(req.GetNetworkOption().GetDelete() || req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "") {
		var drac *ufspb.Drac
		var err error
		if req.UpdateMask != nil && len(req.UpdateMask.Paths) > 0 {
			drac, err = controller.UpdateDrac(ctx, req.Drac, req.UpdateMask)
			if err != nil {
				return nil, err
			}
		} else {
			drac, err = controller.GetDrac(ctx, req.Drac.Name)
			if err != nil {
				return nil, err
			}
		}

		// If network_option.delete is enabled, ignore network_option.vlan and return directly
		if req.GetNetworkOption().GetDelete() {
			if err = controller.DeleteDracHost(ctx, drac.Name); err != nil {
				return nil, err
			}
		}

		if req.GetNetworkOption().GetVlan() != "" || req.GetNetworkOption().GetIp() != "" {
			if err = controller.UpdateDracHost(ctx, drac, req.GetNetworkOption()); err != nil {
				return nil, err
			}
		}

		// https://aip.dev/122 - as per AIP guideline
		drac.Name = util.AddPrefix(util.DracCollection, drac.Name)
		return drac, nil
	}

	drac, err := controller.UpdateDrac(ctx, req.Drac, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	drac.Name = util.AddPrefix(util.DracCollection, drac.Name)
	return drac, err
}

// GetDrac gets the drac information from database.
func (fs *FleetServerImpl) GetDrac(ctx context.Context, req *ufsAPI.GetDracRequest) (rsp *ufspb.Drac, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.FormatDHCPHostname(util.RemovePrefix(req.Name))
	drac, err := controller.GetDrac(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	drac.Name = util.AddPrefix(util.DracCollection, drac.Name)
	return drac, err
}

// BatchGetDracs gets a batch of dracs in batch from database.
func (fs *FleetServerImpl) BatchGetDracs(ctx context.Context, req *ufsAPI.BatchGetDracsRequest) (rsp *ufsAPI.BatchGetDracsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	dracs, err := controller.BatchGetDracs(ctx, util.FormatDHCPHostnames(util.FormatInputNames(req.GetNames())))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, v := range dracs {
		v.Name = util.AddPrefix(util.DracCollection, v.Name)
	}
	return &ufsAPI.BatchGetDracsResponse{
		Dracs: dracs,
	}, nil
}

// ListDracs list the dracs information from database.
func (fs *FleetServerImpl) ListDracs(ctx context.Context, req *ufsAPI.ListDracsRequest) (rsp *ufsAPI.ListDracsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListDracs(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, drac := range result {
		drac.Name = util.AddPrefix(util.DracCollection, drac.Name)
	}
	return &ufsAPI.ListDracsResponse{
		Dracs:         result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteDrac deletes the drac from database.
func (fs *FleetServerImpl) DeleteDrac(ctx context.Context, req *ufsAPI.DeleteDracRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.FormatDHCPHostname(util.RemovePrefix(req.Name))
	err = controller.DeleteDrac(ctx, name)
	return &empty.Empty{}, err
}

// CreateSwitch creates switch entry in database.
func (fs *FleetServerImpl) CreateSwitch(ctx context.Context, req *ufsAPI.CreateSwitchRequest) (rsp *ufspb.Switch, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Switch.Name = req.SwitchId
	s, err := controller.CreateSwitch(ctx, req.Switch)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	s.Name = util.AddPrefix(util.SwitchCollection, s.Name)
	return s, err
}

// UpdateSwitch updates the switch information in database.
func (fs *FleetServerImpl) UpdateSwitch(ctx context.Context, req *ufsAPI.UpdateSwitchRequest) (rsp *ufspb.Switch, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Switch.Name = util.RemovePrefix(req.Switch.Name)
	s, err := controller.UpdateSwitch(ctx, req.Switch, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	s.Name = util.AddPrefix(util.SwitchCollection, s.Name)
	return s, err
}

// GetSwitch gets the switch information from database.
func (fs *FleetServerImpl) GetSwitch(ctx context.Context, req *ufsAPI.GetSwitchRequest) (rsp *ufspb.Switch, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	s, err := controller.GetSwitch(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	s.Name = util.AddPrefix(util.SwitchCollection, s.Name)
	return s, err
}

// BatchGetSwitches gets switches in batch from database.
func (fs *FleetServerImpl) BatchGetSwitches(ctx context.Context, req *ufsAPI.BatchGetSwitchesRequest) (rsp *ufsAPI.BatchGetSwitchesResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	switches, err := controller.BatchGetSwitches(ctx, util.FormatInputNames(req.GetNames()))
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, v := range switches {
		v.Name = util.AddPrefix(util.SwitchCollection, v.Name)
	}
	return &ufsAPI.BatchGetSwitchesResponse{
		Switches: switches,
	}, nil
}

// ListSwitches list the switches information from database.
func (fs *FleetServerImpl) ListSwitches(ctx context.Context, req *ufsAPI.ListSwitchesRequest) (rsp *ufsAPI.ListSwitchesResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListSwitches(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, s := range result {
		s.Name = util.AddPrefix(util.SwitchCollection, s.Name)
	}
	return &ufsAPI.ListSwitchesResponse{
		Switches:      result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteSwitch deletes the switch from database.
func (fs *FleetServerImpl) DeleteSwitch(ctx context.Context, req *ufsAPI.DeleteSwitchRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	err = controller.DeleteSwitch(ctx, name)
	return &empty.Empty{}, err
}

// RenameSwitch renames the switch in database.
func (fs *FleetServerImpl) RenameSwitch(ctx context.Context, req *ufsAPI.RenameSwitchRequest) (rsp *ufspb.Switch, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return
	}
	rsp, err = controller.RenameSwitch(ctx, util.RemovePrefix(req.Name), util.RemovePrefix(req.NewName))
	if err != nil {
		return
	}
	// https://aip.dev/122 - as per AIP guideline
	rsp.Name = util.AddPrefix(util.SwitchCollection, rsp.Name)
	return
}

// CreateAsset creates an asset entry in database.
func (fs *FleetServerImpl) CreateAsset(ctx context.Context, req *ufsAPI.CreateAssetRequest) (response *ufspb.Asset, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Asset.Name = util.RemovePrefix(req.Asset.Name)
	res, err := controller.AssetRegistration(ctx, req.GetAsset())
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	prefix, err := util.GetResourcePrefix(res)
	res.Name = util.AddPrefix(prefix, res.Name)
	return res, err
}

// UpdateAsset updates the asset information in database.
func (fs *FleetServerImpl) UpdateAsset(ctx context.Context, req *ufsAPI.UpdateAssetRequest) (rsp *ufspb.Asset, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Asset.Name = util.RemovePrefix(req.Asset.Name)
	asset, err := controller.UpdateAsset(ctx, req.Asset, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	prefix, err := util.GetResourcePrefix(asset)
	asset.Name = util.AddPrefix(prefix, asset.Name)
	return asset, err
}

// GetAsset gets the asset information from database.
func (fs *FleetServerImpl) GetAsset(ctx context.Context, req *ufsAPI.GetAssetRequest) (rsp *ufspb.Asset, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	asset, err := controller.GetAsset(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	prefix, err := util.GetResourcePrefix(asset)
	asset.Name = util.AddPrefix(prefix, asset.Name)
	return asset, err
}

// ListAssets list the assets information from database.
func (fs *FleetServerImpl) ListAssets(ctx context.Context, req *ufsAPI.ListAssetsRequest) (rsp *ufsAPI.ListAssetsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListAssets(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, asset := range result {
		asset.Name = util.AddPrefix(util.AssetCollection, asset.Name)
	}
	return &ufsAPI.ListAssetsResponse{
		Assets:        result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteAsset deletes the asset from database.
func (fs *FleetServerImpl) DeleteAsset(ctx context.Context, req *ufsAPI.DeleteAssetRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	err = controller.DeleteAsset(ctx, name)
	return &empty.Empty{}, err
}

// RenameAsset renames the asset from database.
func (fs *FleetServerImpl) RenameAsset(ctx context.Context, req *ufsAPI.RenameAssetRequest) (rsp *ufspb.Asset, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return
	}
	name := util.RemovePrefix(req.Name)
	newName := util.RemovePrefix(req.NewName)
	rsp, err = controller.RenameAsset(ctx, name, newName)
	if err != nil {
		return
	}
	rsp.Name = util.AddPrefix(util.AssetCollection, rsp.Name)
	return
}
