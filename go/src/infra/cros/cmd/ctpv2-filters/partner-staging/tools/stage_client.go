// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tools provides tooling for staging builds through moblab.
package tools

import (
	"context"

	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/partner-staging/moblab"
)

// MobLabAPI interface provides subset of Moblab API methods relevant to CTPV2
type MobLabAPI interface {
	StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*moblabpb.StageBuildResponse, error)
	CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error)
}

type MobLabClient struct {
	c *moblab.BuildClient
}

func NewMobLabClient(ctx context.Context, opts ...option.ClientOption) (*MobLabClient, error) {
	client, err := moblab.NewBuildClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &MobLabClient{c: client}, nil
}

func (m *MobLabClient) StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*moblabpb.StageBuildResponse, error) {
	stageBuildOperation, err := m.c.StageBuild(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	stageBuildResponse, err := stageBuildOperation.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return stageBuildResponse, nil
}

func (m *MobLabClient) CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error) {
	return m.c.CheckBuildStageStatus(ctx, req, opts...)
}
