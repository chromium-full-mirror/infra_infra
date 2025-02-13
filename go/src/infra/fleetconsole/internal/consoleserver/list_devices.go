// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"strconv"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/consoleserver/filtering"
	"infra/fleetconsole/internal/consoleserver/sorting"
	"infra/fleetconsole/internal/database/devicesdb"
	"infra/fleetconsole/internal/devicemanagerclient"
	"infra/fleetconsole/internal/internalproto"
	"infra/fleetconsole/internal/utils"
)

const maxPageSize int = 50

// ListDevices lists devices provided via DeviceManager.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (_ *fleetconsolerpc.ListDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()
	logging.Infof(ctx, "beginning of list devices call")
	deviceManagerClient, err := frontend.deviceManagerClient(ctx, frontend.cloudProject)
	if err != nil {
		logging.Errorf(ctx, "failed to connect to device manager: %s", err)
		return nil, errors.Annotate(err, "list devices").Err()
	}

	afterDeviceID, err := pageTokenToDeviceID(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	d, err := deviceManagerClient.Leaser.ListDevices(ctx, &api.ListDevicesRequest{})

	if err != nil {
		logging.Errorf(ctx, "failed to list devices: %s", err)
		return nil, err
	}

	devicesFiltered, err := filtering.FilterDevices(devicemanagerclient.MapDevices(d.Devices), req.Filter)

	if err != nil {
		logging.Errorf(ctx, "failed to filter devices: %s", err)
		return nil, err
	}

	devices, err := sorting.SortDevices(devicesFiltered, req.OrderBy)

	if err != nil {
		logging.Errorf(ctx, "failed to sort devices: %s", err)
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	devicesPage, err := getPage(devices, afterDeviceID, pageSize)

	if err != nil {
		logging.Errorf(ctx, "failed to get page: %s", err)
		return nil, err
	}

	nextPageToken := ""
	if len(devicesPage) > 0 && devicesPage[len(devicesPage)-1].Id != devices[len(devices)-1].Id { // not reached the end of the collection yet
		deviceID := devicesPage[len(devicesPage)-1].Id
		nextPageToken, err = deviceIDToPageToken(ctx, deviceID, req)
		if err != nil {
			logging.Errorf(ctx, "failed to next get page: %s", err)
			return nil, err
		}
	}

	logging.Infof(ctx, "successful end of list devices call")
	return &fleetconsolerpc.ListDevicesResponse{
		Devices:       devicesPage,
		NextPageToken: nextPageToken,
	}, nil
}

// ListDevicesFromDB lists devices from DB. When everything with the db is ready it should replace the ListDevices.
func (frontend *FleetConsoleFrontend) ListDevicesFromDB(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (_ *fleetconsolerpc.ListDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	pageToken, err := pageTokenToDeviceID(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	// Since we're using index-based pagination, we can assume the pageToken is equivalent to the offset
	// TODO: Move this logic into pageTokenToDeviceID (later pageTokenToOffset) as it is part of the pageToken parsing
	var offset int
	if pageToken == "" {
		offset = 0
	} else {
		offset, err = strconv.Atoi(pageToken)
		if err != nil {
			return nil, utils.InvalidTokenError(fmt.Errorf("the page number should be an integer"))
		}
	}

	results, hasMoreData, err := devicesdb.List(ctx, frontend.dbConnection, req.Filter, req.OrderBy, offset, pageSize)
	if err != nil {
		return nil, err
	}

	var nextPageToken string
	if hasMoreData {
		nextPageToken, err = deviceIDToPageToken(ctx, strconv.Itoa(offset+pageSize), req)
		if err != nil {
			logging.Errorf(ctx, "failed to encode next page token: %s", err)
			return nil, err
		}
	}

	return &fleetconsolerpc.ListDevicesResponse{
		Devices:       results,
		NextPageToken: nextPageToken,
	}, nil
}

// TODO: after switching to limit-offset adjust the names of the following functions accordingly

func pageTokenToDeviceID(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (string, error) {
	encodedProto, err := base64.RawURLEncoding.DecodeString(req.GetPageToken())
	if err != nil {
		return "", utils.InvalidTokenError(err)
	}

	var tokenProto internalproto.ListDevicesPaginationToken
	if err := proto.Unmarshal(encodedProto, &tokenProto); err != nil {
		return "", utils.InvalidTokenError(err)
	}

	// Only compare request hashes when a `deviceID` is provided.
	deviceID := tokenProto.GetDeviceId()
	if deviceID != "" && tokenProto.GetParamsHash() != hashListDevicesRequest(req) {
		return "", utils.InvalidTokenError(errors.New("request message fields do not match page deviceID"))
	}

	return deviceID, nil
}

func deviceIDToPageToken(ctx context.Context, deviceID string, req *fleetconsolerpc.ListDevicesRequest) (string, error) {
	nextPageToken, err := proto.Marshal(&internalproto.ListDevicesPaginationToken{
		DeviceId:   deviceID,
		ParamsHash: hashListDevicesRequest(req),
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

func hashListDevicesRequest(req *fleetconsolerpc.ListDevicesRequest) string {
	hash := fnv.New64a()
	hash.Write([]byte("filter"))
	hash.Write([]byte(req.GetFilter()))
	hash.Write([]byte("order_by"))
	hash.Write([]byte(req.GetOrderBy()))
	// page_size and page_token are omitted
	// as they are allowed to change between requests
	return strconv.FormatUint(hash.Sum64(), 36)
}
