// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"

	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	"go.chromium.org/infra/appengine/crosskylabadmin/site"
	shivasUtils "go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/libs/skylab/inventory"
	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// NewHTTPClient creates a new client specifically configured to talk to UFS correctly when run from
// CrOSSkyladAdmin dev or prod. It does not support other environments.
func NewHTTPClient(ctx context.Context) (*http.Client, error) {
	transport, err := auth.GetRPCTransport(ctx, auth.AsSelf)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get RPC transport").Err()
	}
	return &http.Client{
		Transport: transport,
	}, nil
}

// ContextWithNamespace sets namespace to the context.
// If namespace is empty, it will use the default namespace.
func ContextWithNamespace(ctx context.Context, namespace string) context.Context {
	if namespace == "" {
		namespace = ufsUtil.OSNamespace
	}
	logging.Infof(ctx, "Set namespace %q to context", namespace)
	return shivasUtils.SetupContext(ctx, namespace)
}

// Client exposes a deliberately chosen subset of the UFS functionality.
type Client interface {
	GetMachineLSE(context.Context, *ufsAPI.GetMachineLSERequest, ...grpc.CallOption) (*models.MachineLSE, error)
	GetDeviceData(context.Context, *ufsAPI.GetDeviceDataRequest, ...grpc.CallOption) (*ufsAPI.GetDeviceDataResponse, error)
	GetDUTsForLabstation(context.Context, *ufsAPI.GetDUTsForLabstationRequest, ...grpc.CallOption) (*ufsAPI.GetDUTsForLabstationResponse, error)
}

// ClientImpl is the concrete implementation of this client.
type clientImpl struct {
	client ufsAPI.FleetClient
}

// GetMachineLSE gets information about a DUT.
func (c *clientImpl) GetMachineLSE(ctx context.Context, req *ufsAPI.GetMachineLSERequest) (*models.MachineLSE, error) {
	return c.client.GetMachineLSE(ctx, req)
}

// NewClient creates a new UFS client when given a hostname and a http client.
// The hostname should generally be read from the config.
func NewClient(ctx context.Context, hc *http.Client, hostname string) (Client, error) {
	if hc == nil {
		return nil, errors.Reason("new ufs client: hc cannot be nil").Err()
	}
	if hostname == "" {
		return nil, errors.Reason("new ufs client: hostname cannot be empty").Err()
	}
	return ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    hostname,
		Options: site.DefaultPRPCOptions,
	}), nil
}

// GetPoolsClient expsoes the subset of the UFS client API needed by GetPools.
type GetPoolsClient interface {
	GetMachineLSE(ctx context.Context, in *ufsAPI.GetMachineLSERequest, opts ...grpc.CallOption) (*models.MachineLSE, error)
}

// getPoolsForGenericDevice gets the pools for the generic device.
func getPoolsForGenericDevice(ctx context.Context, client Client, hostname string) ([]string, error) {
	res, err := client.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
		Hostname: hostname,
	})
	if err != nil {
		return nil, errors.Annotate(err, "get pools for generic device %q", hostname).Err()
	}
	switch res.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT:
		dRes, _ := res.GetResource().(*ufsAPI.GetDeviceDataResponse_SchedulingUnit)
		return dRes.SchedulingUnit.GetPools(), nil
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		dRes, _ := res.GetResource().(*ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData)
		d := dRes.ChromeOsDeviceData
		if d.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut() != nil {
			// We have a non-labstation DUT.
			return d.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools(), nil
		}
		// We have a labstation DUT.
		return d.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetPools(), nil
	}
	return nil, errors.Reason("get pools for generic device %q: unsupported device type %q", hostname, res.GetResourceType().String()).Err()
}

// GetPools gets the pools associated with a particular bot or dut.
// UFSClient may be nil.
func GetPools(ctx context.Context, client Client, hostname string) ([]string, error) {
	if client == nil {
		return nil, errors.Reason("get pools: client cannot be nil").Err()
	}
	pools, err := getPoolsForGenericDevice(ctx, client, hostname)
	if err != nil {
		logging.Infof(ctx, "Encountered error for bot %q: %s", hostname, err)
		return nil, err
	}
	logging.Infof(ctx, "Successfully got pools for generic device %q", hostname)
	return pools, err
}

// DeviceInfo holds base device info.
type DeviceInfo struct {
	Name  string
	Board string
	Model string
	Pools []string
}

// GetDeviceInfo reads base device info from inventory.
func GetDeviceInfo(ctx context.Context, client Client, hostname string) (*DeviceInfo, error) {
	if client == nil {
		return nil, errors.Reason("get device info: client is nil").Err()
	}
	ddrsp, err := client.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
		Hostname: hostname,
	})
	if err != nil {
		return nil, errors.Annotate(err, "get device info: fail to get device data for %q", hostname).Err()
	}
	switch ddrsp.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT:
		if su := ddrsp.GetSchedulingUnit(); su != nil {
			return &DeviceInfo{
				Name:  su.GetName(),
				Pools: su.GetPools(),
			}, nil
		} else {
			return nil, errors.Reason("get device info: scheduling unit %q is empty", hostname).Err()
		}
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		if d := ddrsp.GetChromeOsDeviceData(); d != nil {
			if dut := d.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut(); dut != nil {
				return &DeviceInfo{
					Name:  d.GetLabConfig().GetName(),
					Board: d.GetMachine().GetChromeosMachine().GetBuildTarget(),
					Model: d.GetMachine().GetChromeosMachine().GetModel(),
					Pools: dut.GetPools(),
				}, nil
			} else if labstation := d.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetLabstation(); labstation != nil {
				return &DeviceInfo{
					Name:  d.GetLabConfig().GetName(),
					Board: d.GetMachine().GetChromeosMachine().GetBuildTarget(),
					Model: d.GetMachine().GetChromeosMachine().GetModel(),
					Pools: labstation.GetPools(),
				}, nil
			} else if devBoard := d.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDevboard(); devBoard != nil {
				di := &DeviceInfo{
					Name:  d.GetLabConfig().GetName(),
					Pools: devBoard.GetPools(),
				}
				if d.GetMachine().GetDevboard().GetAndreiboard() != nil {
					di.Board = "andreiboard"
				} else if d.GetMachine().GetDevboard().GetIcetower() != nil {
					di.Board = "icetower"
				} else if d.GetMachine().GetDevboard().GetDragonclaw() != nil {
					di.Board = "dragonclaw"
				}
				di.Model = di.Board
				return di, nil
			}
			return nil, errors.Reason("get device info: type of chromeos %q is not supported", hostname).Err()
		} else {
			return nil, errors.Reason("get device info: chromeos %q is empty", hostname).Err()
		}
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		if ad := ddrsp.GetAttachedDeviceData(); ad != nil {
			return &DeviceInfo{
				Name:  ad.GetMachine().GetName(),
				Board: ad.GetMachine().GetAttachedDevice().GetBuildTarget(),
				Model: ad.GetMachine().GetAttachedDevice().GetModel(),
				// Pools are not supported
			}, nil
		} else {
			return nil, errors.Reason("get device info: attached device %q is empty", hostname).Err()
		}
	default:
		return nil, errors.Reason("get device info: unsupported device type %q", hostname).Err()
	}
}

func GetDutV1(ctx context.Context, hostname string) (*inventory.DeviceUnderTest, error) {
	cfg := config.Get(ctx)
	hc, err := NewHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(ctx, hc, cfg.GetUFS().GetHost())
	if err != nil {
		return nil, err
	}
	// Namespace Anyone who call it need to set namespase, if not then we will use default os.
	namespace := ufsUtil.OSNamespace
	if existingMetadata, ok := metadata.FromOutgoingContext(ctx); ok {
		// we found a namespace already set in the context, so should just use that
		if ns, ok := existingMetadata[ufsUtil.Namespace]; ok && len(ns) != 0 {
			namespace = ns[0]
		}
	}
	ufsCtx := ContextWithNamespace(ctx, namespace)
	logging.Infof(ctx, "Using namespace %q for %q", namespace, hostname)
	res, err := client.GetDeviceData(ufsCtx, &ufsAPI.GetDeviceDataRequest{
		Hostname: hostname,
	})
	if err != nil {
		return nil, err
	}
	return res.GetChromeOsDeviceData().GetDutV1(), nil
}
