// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"

	"infra/cros/satlab/common/run"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/misc"
)

// RunStorageQual run a storage qualification suite
func (s *SatlabRpcServiceServer) RunStorageQual(
	ctx context.Context,
	in *pb.RunStorageQualRequest,
) (*pb.RunStorageQualResponse, error) {
	if err := s.validateServices(); err != nil {
		return nil, err
	}

	bugId := strings.TrimSpace(in.GetBugId())

	// if bug id is empty, return an error.
	if bugId == "" {
		return nil, errors.New("bug id is empty")
	}

	// test run ID is generated at time of the test request and used to group all trv2 executions within a request
	testRunId := time.Now().UTC().UnixMilli()

	p := "release"
	if misc.IsCustomBuild(in.GetBuild()) {
		p = "local"
	}

	testArgs := fmt.Sprintf("buildartifactsurl=gs://%s/%s-%s/R%s-%s/ bug_id=%s qual_run_id=%d", site.GetGCSPartnerBucket(), in.GetBoard(), p, in.GetMilestone(), in.GetBuild(), bugId, testRunId)
	r := &run.Run{
		Suite:     in.GetSuite(),
		TestArgs:  testArgs,
		Model:     in.GetModel(),
		Board:     in.GetBoard(),
		Milestone: in.GetMilestone(),
		Build:     in.GetBuild(),
		Pool:      in.GetPool(),
		AddedDims: parseDims(in.GetDims()),
		Tags: map[string]string{
			site.BugIDTag:     bugId,
			site.QualRunIDTag: fmt.Sprintf("%d", testRunId),
		},
		TRV2:          true,
		CFT:           true,
		Local:         true,
		TimeoutMins:   site.MaxIshCTPTimeoutMins,
		UploadToCpcon: true,
	}
	buildLink, err := r.TriggerRun(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.RunStorageQualResponse{BuildLink: buildLink}, nil
}
