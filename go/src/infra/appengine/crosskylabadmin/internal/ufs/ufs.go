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

	"infra/appengine/crosskylabadmin/internal/app/config"
	"infra/appengine/crosskylabadmin/site"
	shivasUtils "infra/cmd/shivas/utils"
	"infra/libs/skylab/common/heuristics"
	"infra/libs/skylab/inventory"
	models "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
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

// setupContext set up the outgoing context for API calls.
func setupContext(ctx context.Context, namespace string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, namespace)
	return metadata.NewOutgoingContext(ctx, md)
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
func getPoolsForGenericDevice(ctx context.Context, client Client, botID string, namespace string) ([]string, error) {
	if namespace == "" {
		return nil, errors.Reason(`get pools for generic device %q: namespace cannot be ""`, namespace).Err()
	}
	ctx = shivasUtils.SetupContext(ctx, namespace)
	res, err := client.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
		Hostname: heuristics.NormalizeBotNameToDeviceName(botID),
	})
	if err != nil {
		return nil, errors.Annotate(err, "get pools for generic device %q", botID).Err()
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
	return nil, errors.Reason("get pools for generic device %q: unsupported device type %q", botID, res.GetResourceType().String()).Err()
}

// GetPools gets the pools associated with a particular bot or dut.
// UFSClient may be nil.
func GetPools(ctx context.Context, client Client, botID string) ([]string, error) {
	if client == nil {
		return nil, errors.Reason("get pools: client cannot be nil").Err()
	}
	// Namespace Anyone who call it need to set namespase, if not then we will use default os.
	namespace := ufsUtil.OSNamespace
	if existingMetadata, ok := metadata.FromOutgoingContext(ctx); ok {
		// we found a namespace already set in the context, so should just use that
		if ns, ok := existingMetadata[ufsUtil.Namespace]; ok && len(ns) != 0 {
			namespace = ns[0]
		}
	}
	logging.Infof(ctx, "Using namespace %q for %q", namespace, botID)
	pools, err := getPoolsForGenericDevice(ctx, client, botID, namespace)
	if err != nil {
		logging.Infof(ctx, "Encountered error for bot %q and namespace %q: %s", botID, namespace, err)
		return nil, err
	}
	logging.Infof(ctx, "Successfully got pools for generic device %q in namespace %q", botID, namespace)
	return pools, err
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
	osCtx := setupContext(ctx, ufsUtil.OSNamespace)
	res, err := client.GetDeviceData(osCtx, &ufsAPI.GetDeviceDataRequest{
		Hostname: hostname,
	})
	if err != nil {
		return nil, err
	}
	return res.GetChromeOsDeviceData().GetDutV1(), nil
}
