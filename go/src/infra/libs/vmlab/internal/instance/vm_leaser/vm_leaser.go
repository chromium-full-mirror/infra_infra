// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmleaser

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	vmlabpb "go.chromium.org/infra/libs/vmlab/api"
	"go.chromium.org/infra/vm_leaser/client"
)

// vmLeaserInstanceApi implements vmlabpb.InstanceApi
//
// The struct itself doesn't need to be public.
type vmLeaserInstanceApi struct{}

// New constructs a new vmlabpb.InstanceApi with VM Leaser Service backend.
func New() (vmlabpb.InstanceApi, error) {
	return &vmLeaserInstanceApi{}, nil
}

// vmLeaserServiceClient interfaces the VM Leaser service client.
type vmLeaserServiceClient interface {
	LeaseVM(ctx context.Context, r *api.LeaseVMRequest, opts ...grpc.CallOption) (*api.LeaseVMResponse, error)
	ReleaseVM(ctx context.Context, r *api.ReleaseVMRequest, opts ...grpc.CallOption) (*api.ReleaseVMResponse, error)
	ListLeases(ctx context.Context, r *api.ListLeasesRequest, opts ...grpc.CallOption) (*api.ListLeasesResponse, error)
}

// envConfig returns a VM Leaser client config.
//
// The appropriate config is based on the environment the CLI leasing client
// wishes to connect to.
func envConfig(backendEnv vmlabpb.Config_VmLeaserBackend_Environment) *client.Config {
	switch backendEnv {
	case vmlabpb.Config_VmLeaserBackend_ENV_LOCAL:
		return client.LocalConfig()
	case vmlabpb.Config_VmLeaserBackend_ENV_STAGING:
		return client.StagingConfig()
	default:
		return client.ProdConfig()
	}
}

// Create takes a CreateVmInstanceRequest and returns a VmInstance.
//
// Create parses the CreateVmInstanceRequest into a LeaseVMRequest. A client
// connection is established with the VM Leaser service and the request is sent.
// The connected service (local/staging/production) is based on the Env config.
func (g *vmLeaserInstanceApi) Create(ctx context.Context, req *vmlabpb.CreateVmInstanceRequest) (*vmlabpb.VmInstance, error) {
	err := req.ValidateVmLeaserBackend()
	if err != nil {
		return nil, err
	}

	vmLeaser, err := client.NewClient(ctx, envConfig(req.GetConfig().GetVmLeaserBackend().GetEnv()))
	if err != nil {
		return nil, fmt.Errorf("failed to create new client: %w", err)
	}
	defer vmLeaser.Close()

	return g.leaseVM(ctx, vmLeaser.VMLeaserClient, req)
}

// Delete takes a VmInstance and releases the associated lease using VM Leaser.
func (g *vmLeaserInstanceApi) Delete(ctx context.Context, ins *vmlabpb.VmInstance) error {
	vmLeaserBackend := ins.GetConfig().GetVmLeaserBackend()
	if vmLeaserBackend == nil {
		return fmt.Errorf("invalid argument: bad backend: want vm leaser, got %v", ins.GetConfig())
	}
	if ins.GetName() == "" {
		return errors.New("instance name must be set")
	}
	if vmLeaserBackend.GetVmRequirements().GetGceProject() == "" {
		return errors.New("project must be set")
	}

	vmLeaser, err := client.NewClient(ctx, envConfig(vmLeaserBackend.GetEnv()))
	if err != nil {
		return fmt.Errorf("failed to create new client: %w", err)
	}
	defer vmLeaser.Close()

	return g.releaseVM(ctx, vmLeaser.VMLeaserClient, ins)
}

// List takes a ListVmInstancesRequest and returns a list of VmInstance.
func (g *vmLeaserInstanceApi) List(ctx context.Context, req *vmlabpb.ListVmInstancesRequest) ([]*vmlabpb.VmInstance, error) {
	vmLeaserBackend := req.GetConfig().GetVmLeaserBackend()
	if vmLeaserBackend == nil {
		return nil, fmt.Errorf("invalid argument: bad backend: want vm leaser, got %v", req.GetConfig())
	}
	if vmLeaserBackend.GetVmRequirements().GetGceProject() == "" {
		return nil, errors.New("project must be set")
	}
	if req.GetTagFilters() != nil {
		logging.Debugf(ctx, "List: tag filters are not implemented; they will be ignored.")
	}

	vmLeaser, err := client.NewClient(ctx, envConfig(req.GetConfig().GetVmLeaserBackend().GetEnv()))
	if err != nil {
		return nil, fmt.Errorf("failed to create new client: %w", err)
	}
	defer vmLeaser.Close()

	return g.listLeases(ctx, vmLeaser.VMLeaserClient, req)
}

// leaseVM calls LeaseVM using the VM Leaser service client.
func (g *vmLeaserInstanceApi) leaseVM(ctx context.Context, client vmLeaserServiceClient, req *vmlabpb.CreateVmInstanceRequest) (*vmlabpb.VmInstance, error) {
	vmLeaserBackend := req.GetConfig().GetVmLeaserBackend()
	rsp, err := client.LeaseVM(ctx, &api.LeaseVMRequest{
		LeaseDuration: vmLeaserBackend.GetLeaseDuration(),
		HostReqs:      vmLeaserBackend.GetVmRequirements(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to lease VM: %w", err)
	}

	return &vmlabpb.VmInstance{
		Name: rsp.GetLeaseId(),
		Ssh: &vmlabpb.AddressPort{
			Address: rsp.GetVm().GetAddress().GetHost(),
			Port:    rsp.GetVm().GetAddress().GetPort(),
		},
		Config:    req.GetConfig(),
		GceRegion: rsp.GetVm().GetGceRegion(),
	}, nil
}

// releaseVM calls ReleaseVM using the VM Leaser service client.
func (g *vmLeaserInstanceApi) releaseVM(ctx context.Context, client vmLeaserServiceClient, ins *vmlabpb.VmInstance) error {
	vmLeaserBackend := ins.GetConfig().GetVmLeaserBackend()
	_, err := client.ReleaseVM(ctx, &api.ReleaseVMRequest{
		LeaseId:    ins.GetName(),
		GceProject: vmLeaserBackend.GetVmRequirements().GetGceProject(),
		GceRegion:  ins.GetGceRegion(),
	})
	if err != nil {
		return fmt.Errorf("failed to release VM: %w", err)
	}
	return nil
}

// listLeases calls ListLeases using the VM Leaser service client.
func (g *vmLeaserInstanceApi) listLeases(ctx context.Context, client vmLeaserServiceClient, req *vmlabpb.ListVmInstancesRequest) ([]*vmlabpb.VmInstance, error) {
	vmLeaserBackend := req.GetConfig().GetVmLeaserBackend()
	var parent string
	parent = fmt.Sprintf("/projects/%s", vmLeaserBackend.GetVmRequirements().GetGceProject())
	if vmLeaserBackend.GetVmRequirements().GetGceRegion() != "" {
		parent = fmt.Sprintf("/projects/%s/zones/%s", vmLeaserBackend.GetVmRequirements().GetGceProject(), vmLeaserBackend.GetVmRequirements().GetGceRegion())
	}
	leases, err := client.ListLeases(ctx, &api.ListLeasesRequest{
		Parent: parent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	ins := []*vmlabpb.VmInstance{}
	for _, vm := range leases.GetVms() {
		in := &vmlabpb.VmInstance{
			Name: vm.GetId(),
			Ssh: &vmlabpb.AddressPort{
				Address: vm.GetAddress().GetHost(),
				Port:    vm.GetAddress().GetPort(),
			},
			Config: &vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						VmRequirements: &api.VMRequirements{
							GceProject: vmLeaserBackend.GetVmRequirements().GetGceProject(),
							GceRegion:  vm.GetGceRegion(),
						},
					},
				},
			},
			GceRegion: vm.GetGceRegion(),
		}
		ins = append(ins, in)
	}

	logging.Debugf(ctx, "listLeases: %d leases found", len(ins))
	return ins, nil
}
