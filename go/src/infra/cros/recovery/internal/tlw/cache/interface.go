// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"

	"google.golang.org/grpc"

	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

type UFSClient interface {
	GetMachineLSE(ctx context.Context, req *ufsapi.GetMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error)
	GetMachine(ctx context.Context, req *ufsapi.GetMachineRequest, opts ...grpc.CallOption) (*ufsModels.Machine, error)
	ListCachingServices(ctx context.Context, req *ufsapi.ListCachingServicesRequest, opts ...grpc.CallOption) (*ufsapi.ListCachingServicesResponse, error)
}
