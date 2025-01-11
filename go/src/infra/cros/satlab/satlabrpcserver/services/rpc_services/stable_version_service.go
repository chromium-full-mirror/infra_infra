// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"errors"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/common/logging"

	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/misc"
)

func (s *SatlabRpcServiceServer) StableVersion(ctx context.Context, in *pb.StableVersionRequest) (*pb.StableVersionResponse, error) {
	logging.Infof(ctx, "get stable version request")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	if in.GetBoard() == "" || in.GetModel() == "" {
		return nil, errors.New("invalid pramaters, missing board or model")
	}

	recoveryVersion, err := misc.StableVersionFromFile(in.GetBoard(), in.GetModel())
	if err != nil {
		recoveryVersion, err = s.buildService.FindMostStableBuildByBoardAndModel(ctx, in.GetBoard(), in.GetModel())
		if err != nil {
			return nil, err
		}

		if err = misc.WriteLocalStableVersion(recoveryVersion, site.RecoveryVersionDirectory); err != nil {
			logging.Errorf(ctx, "can't write the recovery file")
		}
	}

	return &pb.StableVersionResponse{
		OsImage:   recoveryVersion.GetOsImage(),
		FwImage:   recoveryVersion.GetFwImage(),
		FwVersion: recoveryVersion.GetFwVersion(),
	}, nil
}
