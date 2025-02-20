// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package moblab_test

import (
	"context"

	"google.golang.org/api/iterator"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	"go.chromium.org/infra/cros/satlab/common/google.golang.org/google/chromeos/moblab"
)

func ExampleNewBuildClient() {
	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use client.
	_ = c
}

func ExampleBuildClient_ListBuildTargets() {
	// import moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	// import "google.golang.org/api/iterator"

	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &moblabpb.ListBuildTargetsRequest{
		// TODO: Fill request struct fields.
	}
	it := c.ListBuildTargets(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			// TODO: Handle error.
		}
		// TODO: Use resp.
		_ = resp
	}
}

func ExampleBuildClient_ListModels() {
	// import moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	// import "google.golang.org/api/iterator"

	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &moblabpb.ListModelsRequest{
		// TODO: Fill request struct fields.
	}
	it := c.ListModels(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			// TODO: Handle error.
		}
		// TODO: Use resp.
		_ = resp
	}
}

func ExampleBuildClient_ListBuilds() {
	// import moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	// import "google.golang.org/api/iterator"

	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &moblabpb.ListBuildsRequest{
		// TODO: Fill request struct fields.
	}
	it := c.ListBuilds(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			// TODO: Handle error.
		}
		// TODO: Use resp.
		_ = resp
	}
}

func ExampleBuildClient_CheckBuildStageStatus() {
	// import moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &moblabpb.CheckBuildStageStatusRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.CheckBuildStageStatus(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleBuildClient_StageBuild() {
	// import moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &moblabpb.StageBuildRequest{
		// TODO: Fill request struct fields.
	}
	op, err := c.StageBuild(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}

	resp, err := op.Wait(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleBuildClient_FindMostStableBuild() {
	// import moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	ctx := context.Background()
	c, err := moblab.NewBuildClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &moblabpb.FindMostStableBuildRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.FindMostStableBuild(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}
