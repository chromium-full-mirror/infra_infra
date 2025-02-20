// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fakes

import (
	"context"

	"google.golang.org/grpc"

	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

type UFSClient struct{}

func (uc *UFSClient) GetMachineLSE(context.Context, *ufsAPI.GetMachineLSERequest, ...grpc.CallOption) (*models.MachineLSE, error) {
	panic("GetMachineLSE")
}

func (uc *UFSClient) GetDeviceData(context.Context, *ufsAPI.GetDeviceDataRequest, ...grpc.CallOption) (*ufsAPI.GetDeviceDataResponse, error) {
	panic("GetDeviceData")
}

func (uc *UFSClient) GetDUTsForLabstation(context.Context, *ufsAPI.GetDUTsForLabstationRequest, ...grpc.CallOption) (*ufsAPI.GetDUTsForLabstationResponse, error) {
	panic("GetDUTsForLabstation")
}
