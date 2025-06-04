// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/libs/vmlab"
	vmapi "go.chromium.org/infra/libs/vmlab/api"
	"go.chromium.org/infra/vm_leaser/internal/constants"
	"go.chromium.org/infra/vm_leaser/internal/controller"
	"go.chromium.org/infra/vm_leaser/internal/validation"
	"go.chromium.org/infra/vm_leaser/internal/zone_selector"
)

// Prove that Server implements pb.VMLeaserServiceServer by instantiating a Server
var _ api.VMLeaserServiceServer = (*Server)(nil)

// Server is a struct implements the pb.VMLeaserServiceServer
type Server struct {
	api.UnimplementedVMLeaserServiceServer
	Env string

	// retry defaults
	initialRetryBackoff time.Duration
	maxRetries          int
}

// NewServer returns a new Server
func NewServer(env string) *Server {
	return &Server{
		Env:                 env,
		initialRetryBackoff: 1 * time.Second,
		maxRetries:          3,
	}
}

// LeaseVM leases a VM defined by LeaseVMRequest
func (s *Server) LeaseVM(ctx context.Context, r *api.LeaseVMRequest) (*api.LeaseVMResponse, error) {
	logging.Infof(ctx, "LeaseVM start")

	// Set defaults for LeaseVMRequest if needed.
	r, err := setDefaultLeaseVMRequest(ctx, r, s.Env)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to set lease request: %s", err)
	}

	if err := validation.ValidateLeaseVMRequest(r); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to validate lease request: %s", err)
	}

	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create NewInstancesRESTClient: %s", err)
	}
	defer instancesClient.Close()

	in := controller.CheckIdempotencyKey(ctx, instancesClient, r.GetHostReqs().GetGceProject(), r.GetIdempotencyKey())
	if in != nil {
		zone, err := zone_selector.ExtractGoogleApiZone(in.GetZone())
		if err != nil {
			return nil, err
		}
		return &api.LeaseVMResponse{
			LeaseId: in.GetName(),
			Vm: &api.VM{
				Id:        in.GetName(),
				GceRegion: zone,
				Address: &api.VMAddress{
					Host: in.GetNetworkInterfaces()[0].GetNetworkIP(),
					Port: 22,
				},
			},
		}, nil
	}

	// Appending "vm-" to satisfy GCE regex
	leaseID := fmt.Sprintf("vm-%s", uuid.New().String())
	quotaExceededZones := map[string]bool{}
	retry := 0
	for {
		err = controller.CreateInstance(ctx, instancesClient, s.Env, leaseID, r)
		if err == nil {
			break
		}
		logging.Errorf(ctx, "retry #%d - error when creating instance: %s", retry, err)

		r = handleLeaseVMError(ctx, r, err, quotaExceededZones)

		if retry >= s.maxRetries {
			return nil, status.Errorf(codes.Internal, "failed to create instance after %d retries: %s", s.maxRetries, err)
		}

		if ctx.Err() != nil {
			return nil, status.Errorf(codes.DeadlineExceeded, "context error when creating instance: %s", ctx.Err())
		}

		time.Sleep(s.initialRetryBackoff * (1 << retry))
		retry++
		logging.Debugf(ctx, "LeaseVM: retrying %d time createInstance", retry)
	}

	in, err = controller.GetInstance(ctx, instancesClient, leaseID, r.GetHostReqs(), true)
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.Errorf(codes.DeadlineExceeded, "when getting instance: %s", ctx.Err())
		}
		return nil, status.Errorf(codes.Internal, "failed to get instance: %s", err)
	}

	zone, err := zone_selector.ExtractGoogleApiZone(in.GetZone())
	if err != nil {
		return nil, err
	}

	return &api.LeaseVMResponse{
		LeaseId: in.GetName(),
		Vm: &api.VM{
			Id: in.GetName(),
			Address: &api.VMAddress{
				// Internal IP. Only one NetworkInterface should be available.
				Host: in.GetNetworkInterfaces()[0].GetNetworkIP(),
				// Temporarily hardcode as port 22
				Port: 22,
			},
			GceRegion: zone,
		},
	}, nil
}

// ExtendLease extends a VM lease
func (s *Server) ExtendLease(ctx context.Context, r *api.ExtendLeaseRequest) (*api.ExtendLeaseResponse, error) {
	logging.Infof(ctx, "ExtendLease start")
	return nil, status.Errorf(codes.Unimplemented, "ExtendLease is not implemented")
}

// ReleaseVM releases a VM lease
func (s *Server) ReleaseVM(ctx context.Context, r *api.ReleaseVMRequest) (*api.ReleaseVMResponse, error) {
	logging.Infof(ctx, "ReleaseVM start")

	// Set default values for ReleaseVMRequest if needed.
	r = setDefaultReleaseVMRequest(ctx, r, s.Env)

	if err := validation.ValidateReleaseVMRequest(r); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to validate release request: %s", err)
	}

	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create NewInstancesRESTClient: %s", err)
	}
	defer instancesClient.Close()

	retry := 0
	for {
		err := controller.DeleteInstance(ctx, instancesClient, r)
		if err == nil {
			break
		}

		logging.Errorf(ctx, "retry #%d - error when deleting instance: %s", retry, err)
		if retry >= s.maxRetries {
			return nil, status.Errorf(codes.Internal, "failed to delete instance after %d retries: %s", s.maxRetries, err)
		}

		if ctx.Err() != nil {
			return nil, status.Errorf(codes.DeadlineExceeded, "context error when deleting instance: %s", ctx.Err())
		}

		time.Sleep(s.initialRetryBackoff * (1 << retry))
		retry++
	}

	return &api.ReleaseVMResponse{
		LeaseId: r.GetLeaseId(),
	}, nil
}

// ImportImage imports a VM custom image using image sync.
func (s *Server) ImportImage(ctx context.Context, r *api.ImportImageRequest) (*api.ImportImageResponse, error) {
	imageApi, err := vmlab.NewImageApi(vmapi.ProviderId_CLOUDSDK)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot get image api provider: %v", err)
	}

	gceImage, err := imageApi.GetImage(r.ImagePath, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to import image: %v", err)
	}
	return &api.ImportImageResponse{
		ImageName: gceImage.GetName(),
	}, nil
}

func (s *Server) ListLeases(ctx context.Context, r *api.ListLeasesRequest) (*api.ListLeasesResponse, error) {
	logging.Infof(ctx, "ListLeases start")

	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create NewInstancesRESTClient: %s", err)
	}
	defer instancesClient.Close()

	instances, err := controller.ListInstances(ctx, instancesClient, r)
	if err != nil {
		return nil, err
	}

	var vms []*api.VM
	for _, in := range instances {
		logging.Infof(ctx, in.GetName())
		if in.GetNetworkInterfaces() == nil || in.GetNetworkInterfaces()[0] == nil {
			logging.Warningf(ctx, "instance %s does not have a network interface", in.GetName())
		}

		var expirationTime *timestamppb.Timestamp
		for _, i := range in.GetMetadata().GetItems() {
			if i.GetKey() == "expiration_time" {
				if unixTime, err := strconv.ParseInt(i.GetValue(), 10, 64); err == nil {
					expirationTime = timestamppb.New(time.Unix(unixTime, 0))
				}
				break
			}
		}

		vms = append(vms, &api.VM{
			Id:        in.GetName(),
			GceRegion: in.GetZone(),
			Address: &api.VMAddress{
				// Internal IP. Only one NetworkInterface should be available.
				Host: in.GetNetworkInterfaces()[0].GetNetworkIP(),
				// Temporarily hardcode as port 22.
				Port: 22,
			},
			ExpirationTime: expirationTime,
		})
	}

	return &api.ListLeasesResponse{
		Vms: vms,
	}, nil
}

// handleLeaseVMError updates the LeaseVMRequest to resolve the error.
func handleLeaseVMError(ctx context.Context, r *api.LeaseVMRequest, err error, quotaExceededZones map[string]bool) *api.LeaseVMRequest {
	if err == nil {
		return r
	}
	if strings.Contains(err.Error(), "QUOTA_EXCEEDED") {
		logging.Debugf(ctx, "handleLeaseVMError: quota exceeded for zone %s; reselecting zone", r.HostReqs.GceRegion)
		quotaExceededZones[r.HostReqs.GceRegion] = true
		r.HostReqs.GceRegion = ""
		for {
			retryZone := zone_selector.SelectZone(ctx, r, time.Now().UnixNano())
			logging.Debugf(ctx, "handleLeaseVMError: selected new zone %s", retryZone)
			logging.Debugf(ctx, "handleLeaseVMError: quota exceeded in %v", quotaExceededZones)
			if !quotaExceededZones[retryZone] {
				r.HostReqs.GceRegion = retryZone
				return r
			}
		}
	}
	return r
}

// setDefaultLeaseVMRequest sets default values for VMRequirements.
func setDefaultLeaseVMRequest(ctx context.Context, r *api.LeaseVMRequest, env string) (*api.LeaseVMRequest, error) {
	defaultParams := constants.GetDefaultParams(env)
	hostReqs := r.GetHostReqs()
	if hostReqs.GetGceDiskSize() == 0 {
		hostReqs.GceDiskSize = defaultParams.DefaultDiskSize
	}
	if hostReqs.GetGceMachineType() == "" {
		hostReqs.GceMachineType = defaultParams.DefaultMachineType
	}
	if hostReqs.GetGceNetwork() == "" {
		hostReqs.GceNetwork = defaultParams.DefaultNetwork
	}
	if hostReqs.GetGceProject() == "" {
		hostReqs.GceProject = defaultParams.DefaultProject
	}
	if hostReqs.GetGceRegion() == "" {
		hostReqs.GceRegion = zone_selector.SelectZone(ctx, r, time.Now().UnixNano())
	}
	if hostReqs.GetSubnetModeNetworkEnabled() {
		var err error
		hostReqs.GceSubnet, err = zone_selector.GetZoneSubnet(ctx, hostReqs.GetGceRegion())
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

// setDefaultReleaseVMRequest sets default values for ReleaseVMRequest.
func setDefaultReleaseVMRequest(ctx context.Context, r *api.ReleaseVMRequest, env string) *api.ReleaseVMRequest {
	defaultParams := constants.GetDefaultParams(env)
	if r.GetGceProject() == "" {
		r.GceProject = defaultParams.DefaultProject
	}
	return r
}
