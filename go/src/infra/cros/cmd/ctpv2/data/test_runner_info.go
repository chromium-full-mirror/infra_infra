// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import "go.chromium.org/chromiumos/config/go/test/api"

// TrRequest represents the what will become a TR request:
// Where it will be 1 request that contains 1 --> many tests.
// This request will be built up to contain all needed information to make a Tr(v2) request.
type TrRequest struct {
	NewReq      *api.SchedulingUnitOptions
	Tcs         []*api.TestCase_Id
	DevicesInfo *DevicesInfo

	// TODO; when HwRequirements is fully deprecated, remove this.
	Req *api.HWRequirements
}

type DevicesInfo struct {
	Dims            []string
	LabDevicesCount int64
}

type MiddleOutResponse struct {
	TrReqs    []*TrRequest
	SuiteInfo *api.SuiteInfo
}
