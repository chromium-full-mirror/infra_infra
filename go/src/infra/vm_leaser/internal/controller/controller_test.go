// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// mockComputeInstancesClient mocks compute.NewInstancesRESTClient for testing.
type mockComputeInstancesClient struct {
	deleteFunc         func() (*compute.Operation, error)
	getFunc            func() (*computepb.Instance, error)
	insertFunc         func(r *computepb.InsertInstanceRequest) (*compute.Operation, error)
	listFunc           func() *compute.InstanceIterator
	aggregatedListFunc func() *compute.InstancesScopedListPairIterator
}

// Delete mocks the Delete instance method of the compute client.
func (m *mockComputeInstancesClient) Delete(context.Context, *computepb.DeleteInstanceRequest, ...gax.CallOption) (*compute.Operation, error) {
	return m.deleteFunc()
}

// Get mocks the Get instance method of the compute client.
func (m *mockComputeInstancesClient) Get(context.Context, *computepb.GetInstanceRequest, ...gax.CallOption) (*computepb.Instance, error) {
	return m.getFunc()
}

// Insert mocks the Insert instance method of the compute client.
func (m *mockComputeInstancesClient) Insert(ctx context.Context, r *computepb.InsertInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error) {
	return m.insertFunc(r)
}

// List mocks the List instance method of the compute client.
func (m *mockComputeInstancesClient) List(context.Context, *computepb.ListInstancesRequest, ...gax.CallOption) *compute.InstanceIterator {
	return m.listFunc()
}

// AggregateList mocks the AggregateList instance method of the compute client.
func (m *mockComputeInstancesClient) AggregatedList(context.Context, *computepb.AggregatedListInstancesRequest, ...gax.CallOption) *compute.InstancesScopedListPairIterator {
	return m.aggregatedListFunc()
}

func TestCheckIdempotencyKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test CheckIdempotencyKey", t, func(t *ftt.Test) {
		t.Run("CheckIdempotencyKey - no instances found", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				aggregatedListFunc: func() *compute.InstancesScopedListPairIterator {
					return nil
				},
			}
			in := CheckIdempotencyKey(ctx, client, "test-project", "test-key")
			assert.Loosely(t, in, should.BeNil)
		})
	})
}

func TestCreateInstance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test CreateInstance", t, func(t *ftt.Test) {
		t.Run("CreateInstance - error: unable to create", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				insertFunc: func(*computepb.InsertInstanceRequest) (*compute.Operation, error) {
					return nil, errors.New("failed insert")
				},
			}
			leaseReq := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceNetwork:     "test-network",
					GceDiskSize:    100,
				},
			}
			err := CreateInstance(ctx, client, "dev", "test-id", leaseReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unable to create instance"))
		})
		t.Run("CreateInstance - error: no operation returned", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				insertFunc: func(*computepb.InsertInstanceRequest) (*compute.Operation, error) {
					return nil, nil
				},
			}
			leaseReq := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceNetwork:     "test-network",
					GceDiskSize:    100,
				},
			}
			err := CreateInstance(ctx, client, "dev", "test-id", leaseReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("no operation returned for waiting"))
		})
		t.Run("CreateInstance - with labels - error: no operation returned", func(t *ftt.Test) {
			labels := map[string]string{"k": "v"}
			client := &mockComputeInstancesClient{
				insertFunc: func(r *computepb.InsertInstanceRequest) (*compute.Operation, error) {
					if !reflect.DeepEqual(r.InstanceResource.GetLabels(), labels) {
						return nil, fmt.Errorf("Expected labels to be %v, but is %v", labels, r.InstanceResource.GetLabels())
					}
					return nil, nil
				},
			}
			leaseReq := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceNetwork:     "test-network",
					GceDiskSize:    100,
				},
				Labels: labels,
			}
			err := CreateInstance(ctx, client, "dev", "test-id", leaseReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("no operation returned for waiting"))
		})
		t.Run("CreateInstance - error: failed to get network interface", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				insertFunc: func(*computepb.InsertInstanceRequest) (*compute.Operation, error) {
					return nil, nil
				},
			}

			leaseReq := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
			}
			err := CreateInstance(ctx, client, "dev", "test-id", leaseReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("failed to get network interface"))
		})
	})
}

func TestDeleteInstance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test DeleteInstance", t, func(t *ftt.Test) {
		t.Run("DeleteInstance - error: unable to delete", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				deleteFunc: func() (*compute.Operation, error) {
					return nil, errors.New("failed delete")
				},
			}
			releaseReq := &api.ReleaseVMRequest{
				LeaseId:    "test-id",
				GceProject: "test-project",
				GceRegion:  "test-region",
			}
			err := DeleteInstance(ctx, client, releaseReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unable to delete instance"))
		})
		t.Run("DeleteInstance - success", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				deleteFunc: func() (*compute.Operation, error) {
					return &compute.Operation{}, nil
				},
			}
			releaseReq := &api.ReleaseVMRequest{
				LeaseId:    "test-id",
				GceProject: "test-project",
				GceRegion:  "test-region",
			}
			err := DeleteInstance(ctx, client, releaseReq)
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestGetInstance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test GetInstance", t, func(t *ftt.Test) {
		t.Run("GetInstance - happy path", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				getFunc: func() (*computepb.Instance, error) {
					return &computepb.Instance{
						Name: proto.String("test-id"),
						NetworkInterfaces: []*computepb.NetworkInterface{
							{
								AccessConfigs: []*computepb.AccessConfig{
									{
										NatIP: proto.String("1.2.3.4"),
									},
								},
							},
						},
					}, nil
				},
			}
			hostReqs := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			ins, err := GetInstance(ctx, client, "test-id", hostReqs, false)
			assert.Loosely(t, ins, should.NotBeNil)
			assert.Loosely(t, ins, should.Match(&computepb.Instance{
				Name: proto.String("test-id"),
				NetworkInterfaces: []*computepb.NetworkInterface{
					{
						AccessConfigs: []*computepb.AccessConfig{
							{
								NatIP: proto.String("1.2.3.4"),
							},
						},
					},
				},
			}))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("GetInstance - error: failed get", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				getFunc: func() (*computepb.Instance, error) {
					return nil, errors.New("failed get")
				},
			}
			hostReqs := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			ins, err := GetInstance(ctx, client, "test-id", hostReqs, false)
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("failed get"))
		})
		t.Run("GetInstance - error: no network interface", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				getFunc: func() (*computepb.Instance, error) {
					return &computepb.Instance{}, nil
				},
			}
			hostReqs := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			ins, err := GetInstance(ctx, client, "test-id", hostReqs, false)
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("instance does not have a network interface"))
		})
		t.Run("GetInstance - error: no access config", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				getFunc: func() (*computepb.Instance, error) {
					return &computepb.Instance{
						NetworkInterfaces: []*computepb.NetworkInterface{
							{},
						},
					}, nil
				},
			}
			hostReqs := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			ins, err := GetInstance(ctx, client, "test-id", hostReqs, false)
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("instance does not have an access config"))
		})
		t.Run("GetInstance - error: no nat ip", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				getFunc: func() (*computepb.Instance, error) {
					return &computepb.Instance{
						NetworkInterfaces: []*computepb.NetworkInterface{
							{
								AccessConfigs: []*computepb.AccessConfig{
									{},
								},
							},
						},
					}, nil
				},
			}
			hostReqs := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			ins, err := GetInstance(ctx, client, "test-id", hostReqs, false)
			assert.Loosely(t, ins, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("instance does not have a nat ip"))
		})
	})
}

func TestListInstances(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test ListInstances", t, func(t *ftt.Test) {
		t.Run("listAllInstances - nil iterator returned", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				aggregatedListFunc: func() *compute.InstancesScopedListPairIterator {
					return nil
				},
			}
			listReq := &api.ListLeasesRequest{
				Parent:    "projects/test-project",
				PageSize:  5,
				PageToken: "test-token",
			}
			_, err := listAllInstances(ctx, client, "test-project", listReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("listAllInstances: cannot get instances"))
		})
		t.Run("listZoneInstances - nil iterator returned", func(t *ftt.Test) {
			client := &mockComputeInstancesClient{
				listFunc: func() *compute.InstanceIterator {
					return nil
				},
			}
			listReq := &api.ListLeasesRequest{
				Parent:    "projects/test-project/zones/test-zone",
				PageSize:  5,
				PageToken: "test-token",
			}
			_, err := listZoneInstances(ctx, client, "test-project", "test-zone", listReq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("listZoneInstances: cannot get instances"))
		})
	})
}

func TestComputeExpirationTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test computeExpirationTime", t, func(t *ftt.Test) {
		t.Run("Compute expiration time - no lease duration passed", func(t *ftt.Test) {
			defaultExpTime := time.Now().Unix() + (600 * 60)
			res, err := computeExpirationTime(ctx, nil, "dev")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.BeBetweenOrEqual(defaultExpTime, defaultExpTime+1))
		})
		t.Run("Compute expiration time - lease duration passed", func(t *ftt.Test) {
			leaseDuration, err := time.ParseDuration("20m")
			assert.Loosely(t, err, should.BeNil)

			expTime := time.Now().Add(leaseDuration).Unix()
			logging.Errorf(ctx, "%s", durationpb.New(leaseDuration))
			res, err := computeExpirationTime(ctx, durationpb.New(leaseDuration), "dev")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.BeBetweenOrEqual(expTime, expTime+1))
		})
	})
}

func TestGetInstanceNetworkInterfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test getInstanceNetworkInterfaces", t, func(t *ftt.Test) {
		t.Run("getInstanceNetworkInterfaces - happy path", func(t *ftt.Test) {
			hostReqs := &api.VMRequirements{
				GceNetwork: "test-network",
				GceSubnet:  "test-subnet",
			}
			n, err := getInstanceNetworkInterfaces(ctx, hostReqs)
			assert.Loosely(t, n, should.Match([]*computepb.NetworkInterface{
				{
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name: proto.String("External NAT"),
						},
					},
					Network:    proto.String("test-network"),
					Subnetwork: proto.String("test-subnet"),
				},
			}))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("getInstanceNetworkInterfaces - error: no network", func(t *ftt.Test) {
			hostReqs := &api.VMRequirements{}
			n, err := getInstanceNetworkInterfaces(ctx, hostReqs)
			assert.Loosely(t, n, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("gce network cannot be empty"))
		})
		t.Run("getInstanceNetworkInterfaces - no subnet", func(t *ftt.Test) {
			hostReqs := &api.VMRequirements{
				GceNetwork: "test-network",
			}
			n, err := getInstanceNetworkInterfaces(ctx, hostReqs)
			assert.Loosely(t, n, should.Match([]*computepb.NetworkInterface{
				{
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name: proto.String("External NAT"),
						},
					},
					Network: proto.String("test-network"),
				},
			}))
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestPoll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Test poll", t, func(t *ftt.Test) {
		t.Run("poll - no context deadline", func(t *ftt.Test) {
			f := func(ctx context.Context) (bool, error) {
				return false, nil
			}
			interval := time.Duration(1)
			err := poll(ctx, f, interval)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("poll - quit on error", func(t *ftt.Test) {
			expected := 2
			count := 1
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			f := func(ctx context.Context) (bool, error) {
				count++
				if count == 2 {
					return false, errors.New("error on 2")
				}
				return false, nil
			}
			err := poll(ctx, f, 100*time.Millisecond)
			actual := count
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, actual, should.Equal(expected))
		})
		t.Run("poll - quit on success", func(t *ftt.Test) {
			expected := 3
			count := 1
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			f := func(ctx context.Context) (bool, error) {
				count++
				if count == 3 {
					return true, nil
				}
				return false, nil
			}
			err := poll(ctx, f, 100*time.Millisecond)
			actual := count

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.Equal(expected))
		})
	})
}
