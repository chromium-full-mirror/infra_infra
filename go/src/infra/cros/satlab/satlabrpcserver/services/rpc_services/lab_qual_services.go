// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"

	"go.chromium.org/infra/cros/satlab/common/run"
)

// RunBvt run a bvt test with special parameters
func (s *SatlabRpcServiceServer) RunLabQual(
	ctx context.Context,
	in *pb.RunLabQualRequest,
) (*pb.RunLabQualResponse, error) {
	if err := s.validateServices(); err != nil {
		return nil, err
	}

	r := &run.Run{
		Suite:         "labqual_stable",
		TestArgs:      "firmware.firmwarePath=" + in.GetFirmwarePath(),
		Model:         in.GetModel(),
		Board:         in.GetBoard(),
		Milestone:     in.GetMilestone(),
		Build:         in.GetBuild(),
		Pool:          in.GetPool(),
		AddedDims:     parseDims(in.GetDims()),
		Local:         true,
		CFT:           true,
		TRV2:          in.GetSettings().GetTrv2(),
		UploadToCpcon: in.GetSettings().GetUploadToCpcon(),
		TagIncludes:   []string{"group:labqual_stable"},
	}
	buildLink, err := r.TriggerRun(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.RunLabQualResponse{BuildLink: buildLink}, nil
}
