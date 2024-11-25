// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

// Data provides access to versions data.
type Data interface {
	GetOsVersion() string
	GetOsImagePath() string
	GetFirmwareRoVersion() string
	GetFirmwareRoImagePath() string
}

// ByDut finds version for DUT.
func ByDut(ctx context.Context, dut *tlw.Dut) (Data, error) {
	if dut == nil {
		return nil, errors.Reason("version by dut: dut is not provided").Err()
	}
	// TODO: update version type based on type of detail of the DUT.
	versionType := UnspecifiedType
	pools := dut.ExtraAttributes[tlw.ExtraAttributePools]
	v, err := version(ctx, dut.Name, versionType, dut.GetBoard(), dut.GetModel(), pools)
	return v, errors.Annotate(err, "version by dut").Err()
}

// ByResource finds version by resource in correlation with version-type by finding board, model and pools infos.
func ByResource(ctx context.Context, versionType Type, dut *tlw.Dut, resource string) (Data, error) {
	if dut == nil {
		return nil, errors.Reason("version by resource: dut is not provided").Err()
	}
	if resource == "" {
		return nil, errors.Reason("version by resource: resource is not specified").Err()
	}
	versionType = defaultTypeIfEmpty(versionType, resource)
	var board, model string
	switch versionType {
	case CrOSType, CameraBoxTabletType, AndroidOSType:
		board = dut.GetBoard()
		model = dut.GetModel()
	case WifiRouterType:
		for _, wr := range dut.GetChromeos().GetWifiRouters() {
			if wr.GetName() == resource {
				model = wr.GetModel()
				// Board and model are the same.
				board = model
				break
			}
		}
		if model == "" {
			return nil, errors.Reason("version by resource: resource not found for WiFi router %s", resource).Err()
		}
	default:
		return nil, errors.Reason("version by resource: unsupported version-type: %s", versionType).Err()
	}
	v, err := version(ctx, resource, versionType, board, model, dut.GetPools())
	return v, errors.Annotate(err, "version by resource").Err()
}

// ByDetails finds version by board, model and pools info.
func ByDetails(ctx context.Context, versionType Type, deviceName, board, model string, pools []string) (Data, error) {
	if deviceName == "" {
		if board == "" || model == "" {
			return nil, errors.Reason("version by details: please provide board and model if device-name is not provided").Err()
		}
	}
	v, err := version(ctx, deviceName, versionType, board, model, pools)
	return v, errors.Annotate(err, "version by details").Err()
}

func version(ctx context.Context, deviceName string, versionType Type, board, model string, pools []string) (rData Data, _ error) {
	c := GetClient(ctx)
	if c == nil {
		return nil, errors.Reason("version: client not found").Err()
	}
	deviceType := toDeviceType(defaultTypeIfEmpty(versionType, deviceName))
	cacheKey := cacheKey(deviceType, deviceName, board, model, pools)
	// Check if the version is in the cache before trying to read from outside.
	// The cache is enabled to prevent instability between two calls that use version information,
	// for example, when the local version file is updated in the middle of a task.
	if cData := getVersionFromCache(ctx, cacheKey); cData != nil {
		log.Debugf(ctx, "Version found in cache by key: %q", cacheKey)
		return cData, nil
	}
	defer func() {
		if rData != nil {
			// Save the version if one is found for this request.
			setVersionToCache(ctx, cacheKey, rData)
		}
	}()
	if board != "" && model != "" {
		if v, err := readLocalVersion(ctx, board, model); err != nil {
			log.Debugf(ctx, "Fail to read local version: %s", err)
		} else {
			return v, nil
		}
	}
	req := &fleet.GetRecoveryVersionRequest{
		DeviceType: deviceType,
		DeviceName: deviceName,
		Board:      board,
		Model:      model,
		Pools:      pools,
	}
	log.Debugf(ctx, "Version uses device type: %q, device name: %q, board: %q, model: %q, pools: %v", req.GetDeviceType(), req.GetDeviceName(), req.GetBoard(), req.GetModel(), req.GetPools())
	res, err := c.GetRecoveryVersion(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.Reason("version: record not found").Err()
		}
		return nil, errors.Annotate(err, "version").Err()
	}
	v := res.GetVersion()
	if v == nil || v.GetOsVersion() == "" {
		return nil, errors.Reason("version: version is empty").Err()
	}
	return v, nil
}
