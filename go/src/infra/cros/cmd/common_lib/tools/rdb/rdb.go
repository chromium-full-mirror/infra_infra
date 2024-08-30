// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rdb implements all rdb commands needed in the TSE environment.
package rdb

import (
	"context"
	"fmt"

	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/metadata"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/lucictx"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"
)

// InheritRDBInvocation links the child builders with the current CTP run in
// RDB. This allows tests results to be displayed at the CTP level in MILO.
func InheritRDBInvocation(ctx context.Context, buildID int64, bbClient buildbucketpb.BuildsClient, recorderClient resultpb.RecorderClient) {
	rdbCtx := lucictx.GetResultDB(ctx)

	// Do not attach the resultdb token if it's empty.
	if rdbCtx == nil || rdbCtx.CurrentInvocation.UpdateToken == "" {
		return
	}

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(resultpb.UpdateTokenMetadataKey, rdbCtx.CurrentInvocation.UpdateToken))
	parentInv := rdbCtx.CurrentInvocation.Name

	var inv string
	// The ScheduleBuild response does not populate the invocation, so we need
	// to request it from buildbucket.
	req := &buildbucketpb.GetBuildRequest{
		Id: buildID,
		Fields: &field_mask.FieldMask{Paths: []string{
			"id",
			"infra.resultdb.invocation",
		}},
	}
	b, err := bbClient.GetBuild(ctx, req)
	if err != nil && b.GetInfra().GetResultdb().GetInvocation() != "" {
		inv = b.GetInfra().GetResultdb().GetInvocation()
	} else {
		// As a fallback, the ResultDB invocation for the child build can be
		// inferred using the Buildbucket id.
		inv = fmt.Sprintf("invocations/build-%d", buildID)
	}

	rreq := resultpb.UpdateIncludedInvocationsRequest{IncludingInvocation: parentInv, AddInvocations: []string{inv}}
	recorderClient.UpdateIncludedInvocations(ctx, &rreq)
}
