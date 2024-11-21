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
	deviceType := toDeviceType(validate(versionType, deviceName))
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
	log.Debugf(ctx, "Version uses device type: %q", req.GetDeviceType())
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
