// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmleaser

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	vmlabpb "infra/libs/vmlab/api"
)

// mockVMLeaserClient mocks vmLeaserServiceClient for testing.
type mockVMLeaserClient struct {
	leaseVM    func() (*api.LeaseVMResponse, error)
	releaseVM  func() (*api.ReleaseVMResponse, error)
	listLeases func() (*api.ListLeasesResponse, error)
}

// LeaseVM mocks the LeaseVM method of the VM Leaser Client.
func (m *mockVMLeaserClient) LeaseVM(context.Context, *api.LeaseVMRequest, ...grpc.CallOption) (*api.LeaseVMResponse, error) {
	return m.leaseVM()
}

// ReleaseVM mocks the ReleaseVM method of the VM Leaser Client.
func (m *mockVMLeaserClient) ReleaseVM(context.Context, *api.ReleaseVMRequest, ...grpc.CallOption) (*api.ReleaseVMResponse, error) {
	return m.releaseVM()
}

// ListLeases mocks the ListLeases method of the VM Leaser Client.
func (m *mockVMLeaserClient) ListLeases(context.Context, *api.ListLeasesRequest, ...grpc.CallOption) (*api.ListLeasesResponse, error) {
	return m.listLeases()
}

func TestCreate(t *testing.T) {
	t.Parallel()
	ftt.Run("Test Create", t, func(t *ftt.Test) {
		t.Run("Create - error: empty request", func(t *ftt.Test) {
			vmLeaser, err := New()
			assert.Loosely(t, err, should.BeNil)

			ins, err := vmLeaser.Create(context.Background(), &vmlabpb.CreateVmInstanceRequest{})
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("no config found"))
		})
	})
}

func TestLeaseVM(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("Test leaseVM", t, func(t *ftt.Test) {
		t.Run("leaseVM - success", func(t *ftt.Test) {
			client := &mockVMLeaserClient{
				leaseVM: func() (*api.LeaseVMResponse, error) {
					return &api.LeaseVMResponse{
						LeaseId: "vm-test-id",
						Vm: &api.VM{
							Id: "vm-test-id",
							Address: &api.VMAddress{
								Host: "1.2.3.4",
								Port: 99,
							},
							Type: api.VMType_VM_TYPE_DUT,
						},
						ExpirationTime: timestamppb.Now(),
					}, nil
				},
			}

			d, err := time.ParseDuration("60s")
			assert.Loosely(t, err, should.BeNil)

			vmLeaser := &vmLeaserInstanceApi{}
			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceImage:       "test-image",
							GceRegion:      "test-region",
							GceProject:     "test-project",
							GceMachineType: "test-machine-type",
							GceDiskSize:    100,
						},
						LeaseDuration: durationpb.New(d),
					},
				},
			}
			ins, err := vmLeaser.leaseVM(ctx, client, &vmlabpb.CreateVmInstanceRequest{
				Config: &cfg,
			})
			assert.Loosely(t, ins, should.Match(&vmlabpb.VmInstance{
				Name: "vm-test-id",
				Ssh: &vmlabpb.AddressPort{
					Address: "1.2.3.4",
					Port:    99,
				},
				Config: &cfg,
			}))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("leaseVM - error: failed to lease VM", func(t *ftt.Test) {
			client := &mockVMLeaserClient{
				leaseVM: func() (*api.LeaseVMResponse, error) {
					return nil, errors.New("leasing error")
				},
			}

			d, err := time.ParseDuration("60s")
			assert.Loosely(t, err, should.BeNil)

			vmLeaser := &vmLeaserInstanceApi{}
			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceImage:       "test-image",
							GceRegion:      "test-region",
							GceProject:     "test-project",
							GceMachineType: "test-machine-type",
							GceDiskSize:    100,
						},
						LeaseDuration: durationpb.New(d),
					},
				},
			}
			ins, err := vmLeaser.leaseVM(ctx, client, &vmlabpb.CreateVmInstanceRequest{
				Config: &cfg,
			})
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("failed to lease VM: leasing error"))
		})
	})
}

func TestDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("Test Delete", t, func(t *ftt.Test) {
		t.Run("Delete - error when deleting; no backend", func(t *ftt.Test) {
			vmLeaser, err := New()
			assert.Loosely(t, err, should.BeNil)

			err = vmLeaser.Delete(ctx, &vmlabpb.VmInstance{})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: bad backend: want vm leaser"))
		})
		t.Run("Delete - error when deleting; no instance name", func(t *ftt.Test) {
			vmLeaser, err := New()
			assert.Loosely(t, err, should.BeNil)

			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceRegion: "test-region",
						},
					},
				},
			}

			err = vmLeaser.Delete(ctx, &vmlabpb.VmInstance{
				Config: &cfg,
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("instance name must be set"))
		})
		t.Run("Delete - error when deleting; no gce project", func(t *ftt.Test) {
			vmLeaser, err := New()
			assert.Loosely(t, err, should.BeNil)

			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceRegion: "test-region",
						},
					},
				},
			}

			err = vmLeaser.Delete(ctx, &vmlabpb.VmInstance{
				Name:   "test-name",
				Config: &cfg,
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("project must be set"))
		})
	})
}

func TestList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("Test List", t, func(t *ftt.Test) {
		t.Run("List - error when listing; no backend", func(t *ftt.Test) {
			vmLeaser, err := New()
			assert.Loosely(t, err, should.BeNil)

			ins, err := vmLeaser.List(ctx, &vmlabpb.ListVmInstancesRequest{})
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: bad backend: want vm leaser"))
		})
		t.Run("List - error when listing; no gce project", func(t *ftt.Test) {
			vmLeaser, err := New()
			assert.Loosely(t, err, should.BeNil)

			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceRegion: "test-region",
						},
					},
				},
			}

			ins, err := vmLeaser.List(ctx, &vmlabpb.ListVmInstancesRequest{
				Config: &cfg,
			})
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("project must be set"))
		})
	})
}

func TestListLeases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("Test listLeases", t, func(t *ftt.Test) {
		t.Run("listLeases - success", func(t *ftt.Test) {
			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceRegion:  "test-region",
							GceProject: "test-project",
						},
					},
				},
			}

			client := &mockVMLeaserClient{
				listLeases: func() (*api.ListLeasesResponse, error) {
					return &api.ListLeasesResponse{
						Vms: []*api.VM{
							{
								Id: "vm-test-id",
								Address: &api.VMAddress{
									Host: "1.2.3.4",
									Port: 99,
								},
								Type:      api.VMType_VM_TYPE_DUT,
								GceRegion: "test-region",
							},
							{
								Id: "vm-test-id-2",
								Address: &api.VMAddress{
									Host: "2.3.4.5",
									Port: 99,
								},
								Type:      api.VMType_VM_TYPE_DUT,
								GceRegion: "test-region",
							},
						},
					}, nil
				},
			}

			vmLeaser := &vmLeaserInstanceApi{}
			ins, err := vmLeaser.listLeases(ctx, client, &vmlabpb.ListVmInstancesRequest{
				Config: &cfg,
			})
			assert.Loosely(t, ins, should.Match([]*vmlabpb.VmInstance{
				{
					Name: "vm-test-id",
					Ssh: &vmlabpb.AddressPort{
						Address: "1.2.3.4",
						Port:    99,
					},
					Config:    &cfg,
					GceRegion: "test-region",
				},
				{
					Name: "vm-test-id-2",
					Ssh: &vmlabpb.AddressPort{
						Address: "2.3.4.5",
						Port:    99,
					},
					Config:    &cfg,
					GceRegion: "test-region",
				},
			}))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("listLeases - no results", func(t *ftt.Test) {
			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceRegion:  "test-region",
							GceProject: "test-project",
						},
					},
				},
			}

			client := &mockVMLeaserClient{
				listLeases: func() (*api.ListLeasesResponse, error) {
					return &api.ListLeasesResponse{
						Vms: []*api.VM{},
					}, nil
				},
			}

			vmLeaser := &vmLeaserInstanceApi{}
			ins, err := vmLeaser.listLeases(ctx, client, &vmlabpb.ListVmInstancesRequest{
				Config: &cfg,
			})
			assert.Loosely(t, ins, should.Match([]*vmlabpb.VmInstance{}))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("listLeases - error when listing", func(t *ftt.Test) {
			cfg := vmlabpb.Config{
				Backend: &vmlabpb.Config_VmLeaserBackend_{
					VmLeaserBackend: &vmlabpb.Config_VmLeaserBackend{
						Env: vmlabpb.Config_VmLeaserBackend_ENV_LOCAL,
						VmRequirements: &api.VMRequirements{
							GceRegion:  "test-region",
							GceProject: "test-project",
						},
					},
				},
			}

			client := &mockVMLeaserClient{
				listLeases: func() (*api.ListLeasesResponse, error) {
					return nil, fmt.Errorf("cannot list VMs")
				},
			}

			vmLeaser := &vmLeaserInstanceApi{}
			ins, err := vmLeaser.listLeases(ctx, client, &vmlabpb.ListVmInstancesRequest{
				Config: &cfg,
			})
			assert.Loosely(t, err, should.ErrLike("failed to list VMs"))
			assert.Loosely(t, ins, should.BeNil)
		})
	})
}
