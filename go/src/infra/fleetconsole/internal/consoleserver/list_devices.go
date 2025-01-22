// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"encoding/base64"
	"fmt"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/consoleserver/filtering"
	"infra/fleetconsole/internal/consoleserver/sorting"
	"infra/fleetconsole/internal/devicemanagerclient"
	"infra/fleetconsole/internal/internalproto"
)

const maxPageSize int = 50

// ListDevices lists devices provided via DeviceManager.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (*fleetconsolerpc.ListDevicesResponse, error) {
	deviceManagerClient, err := frontend.deviceManagerClient(ctx, frontend.cloudProject)
	if err != nil {
		return nil, errors.Annotate(err, "list devices").Err()
	}

	afterDeviceID, err := pageTokenToDeviceID(ctx, req.PageToken)
	if err != nil {
		return nil, err
	}

	d, err := deviceManagerClient.Leaser.ListDevices(ctx, &api.ListDevicesRequest{})

	if err != nil {
		return nil, err
	}

	devicesFiltered, err := filtering.FilterDevices(devicemanagerclient.MapDevices(d.Devices), req.Filter)

	if err != nil {
		return nil, err
	}

	devices, err := sorting.SortDevices(devicesFiltered, req.OrderBy)

	if err != nil {
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	devicesPage, err := getPage(devices, afterDeviceID, pageSize)

	if err != nil {
		return nil, err
	}

	nextPageToken := ""
	if len(devicesPage) > 0 && devicesPage[len(devicesPage)-1].Id != devices[len(devices)-1].Id { // not reached the end of the collection yet
		deviceID := devicesPage[len(devicesPage)-1].Id
		nextPageToken, err = deviceIDToPageToken(ctx, deviceID)
		if err != nil {
			return nil, err
		}
	}

	return &fleetconsolerpc.ListDevicesResponse{
		Devices:       devicesPage,
		NextPageToken: nextPageToken,
	}, nil
}

func pageTokenToDeviceID(ctx context.Context, pageToken string) (string, error) {
	encodedProto, err := base64.RawURLEncoding.DecodeString(pageToken)
	if err != nil {
		return "", errors.Annotate(err, "invalid page token").Err()
	}

	var tokenProto internalproto.ListDevicesPaginationToken
	if err := proto.Unmarshal(encodedProto, &tokenProto); err != nil {
		return "", errors.Annotate(err, "invalid page token").Err()
	}

	return tokenProto.GetToken(), nil
}

func deviceIDToPageToken(ctx context.Context, pageToken string) (string, error) {
	nextPageToken, err := proto.Marshal(&internalproto.ListDevicesPaginationToken{
		Token: pageToken,
	})

	if err != nil {
		return "", errors.Annotate(err, "failed to encrypt page token").Err()
	}

	return base64.RawURLEncoding.EncodeToString(nextPageToken), nil
}

func getPage(devices []*fleetconsolerpc.Device, afterDeviceID string, pageSize int) ([]*fleetconsolerpc.Device, error) {
	if afterDeviceID == "" {
		return devices[0:min(pageSize, len(devices))], nil
	}

	for i, v := range devices {
		if v.Id == afterDeviceID {
			return devices[i+1 : min(i+1+pageSize, len(devices))], nil
		}
	}
	return nil, fmt.Errorf("couldn't find device id: %s", afterDeviceID)
}
