// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package device contains helper functions for fleet devices.
package device

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/libs/fleet/device/attacheddevice"
	"go.chromium.org/infra/libs/fleet/device/dut"
	"go.chromium.org/infra/libs/fleet/device/schedulingunit"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsutil "go.chromium.org/infra/unifiedfleet/app/util"
)

// GetOSResourceDims gets the dimensions of a fleet resource to present to the
// scheduling layer.
func GetOSResourceDims(ctx context.Context, client ufsAPI.FleetClient, r swarming.ReportFunc, name string) (swarming.Dimensions, error) {
	deviceData, err := client.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
		Hostname: name,
	})
	if err != nil {
		return nil, err
	}

	if deviceData.GetResourceType() == ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT {
		return getSUDeviceDims(ctx, client, r, deviceData.GetSchedulingUnit())
	}
	deviceDimensions, err := getBaseResourceDims(ctx, client, r, deviceData)
	if err != nil {
		return nil, err
	}
	return deviceDimensions, nil
}

// getSUDeviceDims gets the dimensions of a SchedulingUnit.
//
// Because Scheduling Units are comprised of multiple devices, this function
// also combines and joins all the dimensions of the other devices.
func getSUDeviceDims(ctx context.Context, client ufsAPI.FleetClient, r swarming.ReportFunc, su *ufspb.SchedulingUnit) (swarming.Dimensions, error) {
	var dutsDims []swarming.Dimensions
	for _, hostname := range su.GetMachineLSEs() {
		deviceData, err := client.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
			Hostname: hostname,
		})
		if err != nil {
			return nil, err
		}
		deviceDimensions, err := getBaseResourceDims(ctx, client, r, deviceData)
		if err != nil {
			return nil, err
		}
		dutsDims = append(dutsDims, deviceDimensions)
	}
	return schedulingunit.GetSchedulingUnitDimensions(su, dutsDims), nil
}

// getBaseResourceDims gets the dimensions of a base device type (ChromeOS
// Device or Attached Device)
func getBaseResourceDims(ctx context.Context, client ufsAPI.FleetClient, r swarming.ReportFunc, deviceData *ufsAPI.GetDeviceDataResponse) (swarming.Dimensions, error) {
	switch deviceData.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		dutState := dutstate.Read(ctx, client, deviceData.GetChromeOsDeviceData().GetLabConfig().GetName())
		return dut.GetDUTBotDims(ctx, r, dutState, deviceData.GetChromeOsDeviceData()), nil
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		dutState := dutstate.Read(ctx, client, deviceData.GetAttachedDeviceData().GetLabConfig().GetName())
		return attacheddevice.GetAttachedDeviceBotDims(ctx, r, dutState, deviceData.GetAttachedDeviceData()), nil
	}
	return nil, fmt.Errorf("getBaseResourceDims: invalid device type (%s)", deviceData.GetResourceType())
}

// GetPoolsClient exposes the subset of the UFS client API for GetPools.
type GetPoolsClient interface {
	GetMachineLSE(ctx context.Context, in *ufsAPI.GetMachineLSERequest, opts ...grpc.CallOption) (*ufspb.MachineLSE, error)
	GetDeviceData(ctx context.Context, in *ufsAPI.GetDeviceDataRequest, opts ...grpc.CallOption) (*ufsAPI.GetDeviceDataResponse, error)
}

// GetPools gets the pools associated with a particular bot or dut.
func GetPools(ctx context.Context, client GetPoolsClient, hostname string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("GetPools: client cannot be nil")
	}
	// Namespace Anyone who call it need to set namespase, if not then we will use default os.
	namespace := ufsutil.OSNamespace
	if existingMetadata, ok := metadata.FromOutgoingContext(ctx); ok {
		// we found a namespace already set in the context, so should just use that
		if ns, ok := existingMetadata[ufsutil.Namespace]; ok && len(ns) != 0 {
			namespace = ns[0]
		}
	}
	pools, err := getPoolsForGenericDevice(ctx, client, hostname, namespace)
	if err != nil {
		return nil, errors.Annotate(err, "getting pool(s) for device").Err()
	}
	return pools, nil
}

// getPoolsForGenericDevice gets the pools for the generic device.
func getPoolsForGenericDevice(ctx context.Context, client GetPoolsClient, hostname string, namespace string) ([]string, error) {
	if namespace == "" {
		return nil, fmt.Errorf("getPoolsForGenericDevice: namespace cannot be empty")
	}
	md := metadata.Pairs(ufsutil.Namespace, namespace)
	ctx = metadata.NewOutgoingContext(ctx, md)
	res, err := client.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
		Hostname: hostname,
	})
	if err != nil {
		return nil, errors.Annotate(err, "getting device data for device %s", hostname).Err()
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
	return nil, fmt.Errorf("getPoolsForGenericDevice %q: unsupported device type %q", hostname, res.GetResourceType().String())
}
