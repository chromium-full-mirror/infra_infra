// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"errors"
	"fmt"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"

	"infra/cros/satlab/common/run"
	"infra/cros/satlab/common/site"
)

const dlmSkuIDDim = "label-dlm_sku_id"
const qualificationIDTag = "qualification_id"

// RunQualification runs a generic PVS qualification on the provided hardware
// and software.
func (s *SatlabRpcServiceServer) RunQualification(
	ctx context.Context,
	req *pb.RunQualificationRequest,
) (*pb.RunQualificationResponse, error) {

	// Validate all clients are authenticated properly.
	if err := s.validateServices(); err != nil {
		return nil, err
	}

	// Validate input request parameters.
	if req.GetQualificationId() == "" {
		return nil, errors.New("qualification id is empty")
	}
	if req.GetBoard() == "" {
		return nil, errors.New("board is empty")
	}
	if req.GetModel() == "" {
		return nil, errors.New("model is empty")
	}
	if req.GetMilestone() == "" {
		return nil, errors.New("cros milestone is empty")
	}
	if req.GetVersion() == "" {
		return nil, errors.New("cros version is empty")
	}
	if req.GetCentralizedSuite() == "" {
		return nil, errors.New("centralized suite is empty")
	}
	if req.GetPool() == "" {
		return nil, errors.New("pool is empty")
	}

	// Construct and trigger CTP request.
	dims := parseDims(req.GetDims())
	if req.GetDlmSkuId() != "" {
		dims[dlmSkuIDDim] = req.GetDlmSkuId()
	}
	buildArtifactsURL := fmt.Sprintf(
		"gs://%s/%s-release/R%s-%s/",
		site.GetGCSPartnerBucket(),
		req.GetBoard(),
		req.GetMilestone(),
		req.GetVersion(),
	)
	r := &run.Run{
		Board:     req.GetBoard(),
		Model:     req.GetModel(),
		Milestone: req.GetMilestone(),
		Build:     req.GetVersion(),
		Suite:     fmt.Sprintf("centralizedsuite:%v", req.GetCentralizedSuite()),
		Pool:      req.GetPool(),
		AddedDims: dims,
		TestArgs:  fmt.Sprintf("buildartifactsurl=%v", buildArtifactsURL),
		Tags: map[string]string{
			qualificationIDTag: req.GetQualificationId(),
		},
		TRV2:             true,
		CFT:              true,
		RunCtpv2WithQs:   true,
		IsIncrementalRun: req.GetIsIncrementalRun(),
		Local:            true,
		TimeoutMins:      site.MaxIshCTPTimeoutMins,
	}
	buildLink, err := r.TriggerRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start ctp build: %w", err)
	}
	return &pb.RunQualificationResponse{BuildLink: buildLink}, nil
}
