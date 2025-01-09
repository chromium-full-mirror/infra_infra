// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"
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
	DBClient     *database.Client
	PubSubClient external.PubSubClient
	UFSClient    ufsAPI.FleetClient
	BQClient     *bigquery.Client
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
func NewDBClient(ctx context.Context, dbconf *database.DatabaseConfig) (*database.Client, error) {
	db, err := database.ConnectDB(ctx, dbconf)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "new db client: %s", err)
	}
	return &database.Client{Conn: db, Config: dbconf}, nil
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

// SetUpBQClient sets up a reusable BigQuery client for the server
func SetUpBQClient(ctx context.Context, server *Server, cloudProject string) error {
	var cp string
	if cloudProject == "" {
		cp = "fleet-device-manager-dev"
	} else {
		cp = cloudProject
	}

	client, err := external.NewBQClient(ctx, cp)
	if err != nil {
		logging.Errorf(ctx, "UpdateDevice: cannot set up PubSub client: %s", err)
		return err
	}
	server.ServiceClients.BQClient = client
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
		return nil, status.Errorf(codes.InvalidArgument, "LeaseDevice: schedulable labels are empty")
	}

	var (
		idType   model.DeviceIDType
		deviceID string
	)

	for _, v := range []model.DeviceIDType{
		model.IDTypeDutID,
		model.IDTypeHostname,
	} {
		deviceID, err = controller.ExtractSingleValuedDimension(ctx, deviceLabels, string(v))
		if err == nil {
			idType = v
			break
		}
		logging.Debugf(ctx, err.Error())
	}

	if deviceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "LeaseDevice: dut_id and device_id labels have no values")
	}
	return controller.LeaseDevice(ctx, s.ServiceClients.DBClient.Conn, r, deviceID, idType)
}

// BulkLeaseDevices takes a BulkLeaseDevicesRequest and leases a corresponding device.
//
// BulkLeaseDevices currently only supported leasing with DUT ID.
func (s *Server) BulkLeaseDevices(ctx context.Context, r *api.BulkLeaseDevicesRequest) (*api.BulkLeaseDevicesResponse, error) {
	logging.Debugf(ctx, "BulkLeaseDevices: received BulkLeaseDevicesRequest %v", r)

	if len(r.GetLeaseDeviceRequests()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "BulkLeaseDevices: no lease device requests provided")
	}

	logging.Debugf(ctx, "BulkLeaseDevices: controller processing BulkLeaseDevicesRequests")
	return controller.BulkLeaseDevices(ctx, s.ServiceClients.DBClient.Conn, r)
}

// ReleaseDevice releases the leased device.
func (s *Server) ReleaseDevice(ctx context.Context, r *api.ReleaseDeviceRequest) (*api.ReleaseDeviceResponse, error) {
	logging.Debugf(ctx, "ReleaseDevice: received request %v", r)
	// TODO b/379718400 verify the input lease ID is a valid UUID.
	if r.GetLeaseId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ReleaseDevice: no lease id provided")
	}
	return controller.ReleaseDevice(ctx, s.ServiceClients.DBClient.Conn, r)
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
