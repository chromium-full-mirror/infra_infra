// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"

	"infra/device_manager/internal/controller"
	"infra/device_manager/internal/database"
	"infra/device_manager/internal/external"
	"infra/device_manager/internal/model"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// Prove that Server implements pb.DeviceLeaseServiceServer by instantiating a Server.
var _ api.DeviceLeaseServiceServer = (*Server)(nil)

// Server is a struct implements the pb.DeviceLeaseServiceServer.
type Server struct {
	api.UnimplementedDeviceLeaseServiceServer

	ServiceClients ServiceClients

	// retry defaults
	initialRetryBackoff time.Duration
	maxRetries          int
}

// ServiceClients contains all relevant service clients for Device Manager Service.
type ServiceClients struct {
	DBClient     database.Client
	PubSubClient *pubsub.Client
	UFSClient    ufsAPI.FleetClient
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

// InstallServices takes a DeviceLeaseServiceServer and exposes it to a LUCI
// prpc.Server.
func InstallServices(s *Server, srv *server.Server) {
	srv.ConfigurePRPC(func(p *prpc.Server) {
		p.AccessControl = prpc.AllowOriginAll
	})

	api.RegisterDeviceLeaseServiceServer(srv, s)
}

// SetUpDBClient sets up a reusable database client for the server
func SetUpDBClient(ctx context.Context, server *Server, dbconf database.DatabaseConfig) error {
	db, err := database.ConnectDB(ctx, dbconf)
	if err != nil {
		return status.Errorf(codes.Internal, "SetUpDBClient: could not set up DB client: %s", err)
	}

	server.ServiceClients.DBClient.Conn = db
	server.ServiceClients.DBClient.Config = dbconf

	return nil
}

// SetUpPubSubClient sets up a reusable PubSub client for the server
func SetUpPubSubClient(ctx context.Context, server *Server, cloudProject string) error {
	var cp string
	if cloudProject == "" {
		cp = "fleet-device-manager-dev"
	} else {
		cp = cloudProject
	}

	client, err := external.NewPubSubClient(ctx, cp)
	if err != nil {
		logging.Errorf(ctx, "UpdateDevice: cannot set up PubSub client: %s", err)
		return err
	}
	server.ServiceClients.PubSubClient = client
	return nil
}

// LeaseDevice takes a LeaseDeviceRequest and leases a corresponding device.
func (s *Server) LeaseDevice(ctx context.Context, r *api.LeaseDeviceRequest) (*api.LeaseDeviceResponse, error) {
	logging.Debugf(ctx, "LeaseDevice: received LeaseDeviceRequest %v", r)

	// Check idempotency of lease. Return if there is an existing unexpired lease.
	rsp, err := controller.CheckLeaseIdempotency(ctx, s.ServiceClients.DBClient.Conn, r.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	if rsp.GetDeviceLease() != nil {
		return rsp, nil
	}

	// Parse hardware requirements. Initial iteration will take an ID and search
	// for the device to lease.
	deviceLabels := r.GetHardwareDeviceReqs().GetSchedulableLabels()
	if len(deviceLabels) == 0 {
		return nil, status.Errorf(codes.NotFound, "LeaseDevice: schedulable labels are empty")
	}

	var (
		idType model.DeviceIDType
		val    string
	)

	for _, v := range []model.DeviceIDType{
		model.IDTypeDutID,
		model.IDTypeHostname,
	} {
		val, err = controller.ExtractSingleValuedDimension(ctx, deviceLabels, string(v))
		if err == nil {
			idType = v
			break
		}
		logging.Debugf(ctx, err.Error())
	}

	if val == "" {
		return nil, status.Errorf(codes.NotFound, "LeaseDevice: dut_id and device_id labels have no values")
	}

	device, err := model.GetDeviceByID(ctx, s.ServiceClients.DBClient.Conn, idType, val)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "LeaseDevice: failed to find Device %s: %s", val, err)
	}
	logging.Debugf(ctx, "LeaseDevice: found Device %s: %v", val, device)

	if !controller.IsDeviceAvailable(ctx, device.DeviceState) {
		return nil, status.Errorf(codes.Unavailable, "LeaseDevice: device %s is unavailable for lease", val)
	}
	return controller.LeaseDevice(ctx, s.ServiceClients.DBClient.Conn, s.ServiceClients.PubSubClient, r, &device)
}

// ReleaseDevice releases the leased device.
func (s *Server) ReleaseDevice(ctx context.Context, r *api.ReleaseDeviceRequest) (*api.ReleaseDeviceResponse, error) {
	return controller.ReleaseDevice(ctx, s.ServiceClients.DBClient.Conn, s.ServiceClients.PubSubClient, r)
}

// ExtendLease attempts to extend the lease on a device by ExtendLeaseRequest.
func (s *Server) ExtendLease(ctx context.Context, r *api.ExtendLeaseRequest) (*api.ExtendLeaseResponse, error) {
	logging.Debugf(ctx, "ExtendLease: received ExtendLeaseRequest %v", r)

	// Check idempotency of ExtendLeaseRequest. Return request if it is a
	// duplicate.
	rsp, err := controller.CheckExtensionIdempotency(ctx, s.ServiceClients.DBClient.Conn, r.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	if rsp.GetLeaseId() != "" {
		return rsp, nil
	}

	return controller.ExtendLease(ctx, s.ServiceClients.DBClient.Conn, r)
}

// GetDevice takes a GetDeviceRequest and returns a corresponding device.
func (s *Server) GetDevice(ctx context.Context, r *api.GetDeviceRequest) (*api.Device, error) {
	logging.Debugf(ctx, "GetDevice: received GetDeviceRequest %v", r)
	if r.Name == "" {
		return nil, status.Errorf(codes.Internal, "GetDevice: request has no device name")
	}

	// Default to using hostname as the query ID type.
	device, err := controller.GetDevice(ctx, s.ServiceClients.DBClient.Conn, model.IDTypeHostname, r.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "GetDevice: failed to get Device %s: %s", r.Name, err)
	}
	logging.Debugf(ctx, "GetDevice: received Device %v", device)
	return device, nil
}

// ListDevices takes a ListDevicesRequest and returns a list of corresponding devices.
func (s *Server) ListDevices(ctx context.Context, r *api.ListDevicesRequest) (*api.ListDevicesResponse, error) {
	// TODO (b/337086313): Implement endpoint-level validations
	if r.GetParent() != "" {
		return nil, status.Errorf(codes.Unimplemented, "ListDevices: filtering by parent (pool) is not yet supported")
	}

	return controller.ListDevices(ctx, s.ServiceClients.DBClient.Conn, r)
}
