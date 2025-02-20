// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"encoding/base64"
	"hash/fnv"
	"strconv"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
	"go.chromium.org/infra/fleetconsole/internal/internalproto"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

const maxPageSize int = 50

// ListDevices lists devices from the db.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (_ *fleetconsolerpc.ListDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	offset, err := pageTokenToOffset(req)
	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	results, hasMoreData, err := devicesdb.List(ctx, frontend.dbConnection, req.Filter, req.OrderBy, offset, pageSize)
	if err != nil {
		return nil, err
	}

	var nextPageToken string
	if hasMoreData {
		nextPageToken, err = offsetToPageToken(offset+pageSize, req)
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

func pageTokenToOffset(req *fleetconsolerpc.ListDevicesRequest) (int, error) {
	encodedProto, err := base64.RawURLEncoding.DecodeString(req.GetPageToken())
	if err != nil {
		return 0, utils.InvalidTokenError(err)
	}

	var tokenProto internalproto.ListDevicesPaginationToken
	if err := proto.Unmarshal(encodedProto, &tokenProto); err != nil {
		return 0, utils.InvalidTokenError(err)
	}

	// Only compare request hashes when a `offset` is provided.
	offset := tokenProto.GetOffset()
	if offset != 0 && tokenProto.GetParamsHash() != hashListDevicesRequest(req) {
		return 0, utils.InvalidTokenError(errors.New("request message fields do not match fields for the current page"))
	}

	return int(offset), nil
}

func offsetToPageToken(offset int, req *fleetconsolerpc.ListDevicesRequest) (string, error) {
	nextPageToken, err := proto.Marshal(&internalproto.ListDevicesPaginationToken{
		Offset:     int32(offset),
		ParamsHash: hashListDevicesRequest(req),
	})

	if err != nil {
		return "", errors.Annotate(err, "failed to encrypt page token").Err()
	}

	return base64.RawURLEncoding.EncodeToString(nextPageToken), nil
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
