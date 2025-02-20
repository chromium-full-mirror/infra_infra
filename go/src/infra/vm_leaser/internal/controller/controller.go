// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/vm_leaser/internal/constants"
	"go.chromium.org/infra/vm_leaser/internal/validation"
)

// computeInstancesClient interfaces the GCE instance client API.
type computeInstancesClient interface {
	Delete(ctx context.Context, r *computepb.DeleteInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error)
	Get(ctx context.Context, r *computepb.GetInstanceRequest, opts ...gax.CallOption) (*computepb.Instance, error)
	Insert(ctx context.Context, r *computepb.InsertInstanceRequest, opts ...gax.CallOption) (*compute.Operation, error)
	List(ctx context.Context, r *computepb.ListInstancesRequest, opts ...gax.CallOption) *compute.InstanceIterator
	AggregatedList(ctx context.Context, r *computepb.AggregatedListInstancesRequest, opts ...gax.CallOption) *compute.InstancesScopedListPairIterator
}

// CheckIdempotencyKey checks the request idempotency with the existing leases
//
// CheckIdempotencyKey takes a key and searches all instances within a project
// to check if any current lease matches the request being made. If a request is
// a duplicate request based on the key, then the instance already created will
// be returned to the caller and no additional request will be made to GCE.
func CheckIdempotencyKey(ctx context.Context, client computeInstancesClient, project, idemKey string) *computepb.Instance {
	t1 := time.Now()
	allInstances, err := listAllInstances(ctx, client, project, nil)
	if err != nil {
		logging.Warningf(ctx, "CheckIdempotencyKey: could not check idempotency; continuing lease operation")
	}
	for _, in := range allInstances {
		for _, m := range in.GetMetadata().GetItems() {
			if m.GetKey() == "idempotency_key" && m.GetValue() == idemKey {
				logging.Debugf(ctx, "CheckIdempotencyKey: found matching idempotency key; returning VM")
				logging.Debugf(ctx, "CheckIdempotencyKey: check completed in %v", time.Since(t1))
				return in
			}
		}
	}
	logging.Debugf(ctx, "CheckIdempotencyKey: check completed in %v", time.Since(t1))
	return nil
}

// CreateInstance sends an instance creation request to the Compute Engine API and waits for it to complete.
func CreateInstance(parentCtx context.Context, client computeInstancesClient, env, leaseID string, r *api.LeaseVMRequest) error {
	ctx, cancel := context.WithTimeout(parentCtx, 600*time.Second)
	defer cancel()

	hostReqs := r.GetHostReqs()
	zone := hostReqs.GetGceRegion()
	networkInterfaces, err := getInstanceNetworkInterfaces(ctx, hostReqs)
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}
	metadata, err := getMetadata(ctx, env, r)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	req := &computepb.InsertInstanceRequest{
		Project: hostReqs.GetGceProject(),
		Zone:    zone,
		InstanceResource: &computepb.Instance{
			Name: proto.String(leaseID),
			Disks: []*computepb.AttachedDisk{
				{
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						DiskSizeGb:  proto.Int64(hostReqs.GetGceDiskSize()),
						SourceImage: proto.String(hostReqs.GetGceImage()),
					},
					AutoDelete: proto.Bool(true),
					Boot:       proto.Bool(true),
					Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
				},
			},
			MachineType:       proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", zone, hostReqs.GetGceMachineType())),
			Metadata:          metadata,
			NetworkInterfaces: networkInterfaces,
			Labels:            r.GetLabels(),
		},
	}

	if hostReqs.GetGceMinCpuPlatform() != "" {
		req.InstanceResource.MinCpuPlatform = proto.String(hostReqs.GetGceMinCpuPlatform())
	}

	logging.Debugf(ctx, "CreateInstance: InsertInstanceRequest payload: %v", req)
	op, err := client.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("unable to create instance: %w", err)
	}
	if op == nil {
		return errors.New("no operation returned for waiting")
	}

	logging.Debugf(ctx, "CreateInstance: waiting for operation completion")
	if err = op.Wait(ctx); err != nil {
		return fmt.Errorf("unable to wait for the operation: %w", err)
	}

	logging.Infof(ctx, "CreateInstance: instance scheduled for creation: %s", leaseID)
	return nil
}

// DeleteInstance sends an instance deletion request to the Compute Engine API.
func DeleteInstance(ctx context.Context, c computeInstancesClient, r *api.ReleaseVMRequest) error {
	req := &computepb.DeleteInstanceRequest{
		Instance: r.GetLeaseId(),
		Project:  r.GetGceProject(),
		Zone:     r.GetGceRegion(),
	}
	logging.Debugf(ctx, "DeleteInstance: DeleteInstanceRequest payload: %v", req)

	// We omit checking the returned operation or calling Wait so that this call
	// becomes non-blocking. This saves callers time and lets the clean up cron
	// job take care of any stale instances instead. See b/287524018.
	_, err := c.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("unable to delete instance: %w", err)
	}

	logging.Infof(ctx, "DeleteInstance: instance delete request received by GCP")
	return nil
}

// GetInstance gets a GCE instance based on lease id and GCE configs.
//
// GetInstance returns a GCE instance with valid network interface and network
// IP. If no network is available, it does not return the instance.
func GetInstance(parentCtx context.Context, client computeInstancesClient, leaseID string, hostReqs *api.VMRequirements, shouldPoll bool) (*computepb.Instance, error) {
	// Implement a 30 second deadline for polling for the instance
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	getReq := &computepb.GetInstanceRequest{
		Instance: leaseID,
		Project:  hostReqs.GetGceProject(),
		Zone:     hostReqs.GetGceRegion(),
	}

	var in *computepb.Instance
	var err error
	if shouldPoll {
		logging.Debugf(ctx, "GetInstance: polling for instance")
		err = poll(ctx, func(ctx context.Context) (bool, error) {
			in, err = client.Get(ctx, getReq)
			if err != nil {
				return false, err
			}
			return true, nil
		}, 2*time.Second)
		if err != nil {
			return nil, err
		}
	} else {
		logging.Debugf(ctx, "GetInstance: getting instance without polling")
		in, err = client.Get(ctx, getReq)
		if err != nil {
			return nil, err
		}
	}

	if in.GetNetworkInterfaces() == nil || in.GetNetworkInterfaces()[0] == nil {
		return nil, errors.New("instance does not have a network interface")
	}
	if in.GetNetworkInterfaces()[0].GetAccessConfigs() == nil || in.GetNetworkInterfaces()[0].GetAccessConfigs()[0] == nil {
		return nil, errors.New("instance does not have an access config")
	}
	if in.GetNetworkInterfaces()[0].GetAccessConfigs()[0].GetNatIP() == "" {
		return nil, errors.New("instance does not have a nat ip")
	}
	return in, nil
}

// ListInstances lists VMs in a GCP project based on request filters.
func ListInstances(ctx context.Context, client computeInstancesClient, r *api.ListLeasesRequest) ([]*computepb.Instance, error) {
	logging.Debugf(ctx, "ListInstances: %v", r)
	parent := r.GetParent()
	if err := validation.ValidateLeaseParent(parent); err != nil {
		return nil, err
	}
	matches := validation.ValidLeaseParent.FindStringSubmatch(parent)
	project := matches[validation.ValidLeaseParent.SubexpIndex("project")]
	zone := matches[validation.ValidLeaseParent.SubexpIndex("zone")]

	if zone == "" {
		return listAllInstances(ctx, client, project, r)
	}
	return listZoneInstances(ctx, client, project, zone, r)
}

// listAllInstances lists all VMs in a GCP project.
func listAllInstances(ctx context.Context, client computeInstancesClient, project string, r *api.ListLeasesRequest) ([]*computepb.Instance, error) {
	maxResults := uint32(r.GetPageSize())
	req := &computepb.AggregatedListInstancesRequest{
		Project:    project,
		PageToken:  proto.String(r.GetPageToken()),
		MaxResults: &maxResults,
		Filter:     proto.String(filterString(r)),
	}

	var instances []*computepb.Instance
	it := client.AggregatedList(ctx, req)
	if it == nil {
		return nil, errors.New("listAllInstances: cannot get instances")
	}
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		zoneIns := pair.Value.Instances
		if len(zoneIns) > 0 {
			instances = append(instances, zoneIns...)
		}
	}
	logging.Infof(ctx, "listAllInstances: found %v instances in project %s", len(instances), project)
	return instances, nil
}

// listZoneInstances lists all VMs in a GCP project specified by zone.
func listZoneInstances(ctx context.Context, client computeInstancesClient, project, zone string, r *api.ListLeasesRequest) ([]*computepb.Instance, error) {
	maxResults := uint32(r.GetPageSize())
	req := &computepb.ListInstancesRequest{
		PageToken:  proto.String(r.GetPageToken()),
		Project:    project,
		MaxResults: &maxResults,
		Zone:       zone,
		Filter:     proto.String(filterString(r)),
	}

	var instances []*computepb.Instance
	it := client.List(ctx, req)
	if it == nil {
		return nil, errors.New("listZoneInstances: cannot get instances")
	}
	for {
		in, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		instances = append(instances, in)
	}
	logging.Infof(ctx, "listZoneInstances: found %v instances in project %s zone %s", len(instances), project, zone)
	return instances, nil
}

// filterString generates a full filter string based on the request.
func filterString(r *api.ListLeasesRequest) string {
	filterString := "name:vm-*"
	if r.GetFilter() != "" {
		filterString += " AND " + r.GetFilter()
	}
	return filterString
}

// computeExpirationTime calculates the expiration time of a VM
//
// computeExpirationTime return a future Unix time as an int64. The calculation
// is based on the specified lease duration.
func computeExpirationTime(ctx context.Context, leaseDuration *durationpb.Duration, env string) (int64, error) {
	defaultParams := constants.GetDefaultParams(env)
	expirationTime := time.Now().Unix()
	if leaseDuration == nil {
		return expirationTime + (defaultParams.DefaultLeaseDuration * 60), nil
	}
	return expirationTime + leaseDuration.GetSeconds(), nil
}

// getInstanceNetworkInterfaces gets the NetworkInterfaces based on VM reqs.
func getInstanceNetworkInterfaces(ctx context.Context, hostReqs *api.VMRequirements) ([]*computepb.NetworkInterface, error) {
	if hostReqs.GetGceNetwork() == "" {
		return nil, errors.New("gce network cannot be empty")
	}

	netInts := []*computepb.NetworkInterface{
		{
			AccessConfigs: []*computepb.AccessConfig{
				{
					Name: proto.String("External NAT"),
				},
			},
			Network: proto.String(hostReqs.GetGceNetwork()),
		},
	}
	if hostReqs.GetGceSubnet() != "" {
		netInts[0].Subnetwork = proto.String(hostReqs.GetGceSubnet())
	}

	return netInts, nil
}

// getMetadata gets the Metadata based on VM reqs.
func getMetadata(ctx context.Context, env string, r *api.LeaseVMRequest) (*computepb.Metadata, error) {
	expirationTime, err := computeExpirationTime(ctx, r.GetLeaseDuration(), env)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to compute expiration time: %s", err)
	}

	metadataItems := []*computepb.Items{
		{
			Key:   proto.String("expiration_time"),
			Value: proto.String(strconv.FormatInt(expirationTime, 10)),
		},
	}

	if r.GetIdempotencyKey() != "" {
		metadataItems = append(metadataItems, &computepb.Items{
			Key:   proto.String("idempotency_key"),
			Value: proto.String(r.GetIdempotencyKey()),
		})
	}

	return &computepb.Metadata{
		Items: metadataItems,
	}, nil
}

// poll is a generic polling function that polls by interval
//
// poll provides a generic implementation of calling f at interval, exits on
// error or ctx timeout. f return true to end poll early.
func poll(ctx context.Context, f func(context.Context) (bool, error), interval time.Duration) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("context must have a deadline to avoid infinite polling")
	}
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			success, err := f(ctx)
			if err != nil {
				logging.Debugf(ctx, "poll: error")
				return err
			}
			if success {
				logging.Debugf(ctx, "poll: success")
				return nil
			}
		case <-ctx.Done():
			logging.Debugf(ctx, "poll: context done")
			return ctx.Err()
		}
	}
}
