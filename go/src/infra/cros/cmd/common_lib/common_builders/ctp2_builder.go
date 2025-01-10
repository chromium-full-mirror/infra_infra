// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_builders

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/interfaces"
)

type ManifestFetcher func(ctx context.Context, s string) (string, error)

type CTPV2FromV1 struct {
	interfaces.CTPv2Builder

	ctx             context.Context
	v2              *testapi.CTPv2Request
	v1              map[string]*test_platform.Request
	manifestFetcher ManifestFetcher
	buildState      *build.State
}

type V2WithKey struct {
	Key string
	V2  *testapi.CTPRequest
}

func NewV2WithKey(key string, v2 *testapi.CTPRequest) *V2WithKey {
	return &V2WithKey{Key: key, V2: v2}
}

func NewCTPV2FromV1(ctx context.Context, v1 map[string]*test_platform.Request, buildState *build.State) *CTPV2FromV1 {
	return &CTPV2FromV1{
		v1: v1,
		v2: &testapi.CTPv2Request{
			Requests: []*testapi.CTPRequest{},
		},
		ctx:             ctx,
		manifestFetcher: GetBuilderManifestFromContainer,
		buildState:      buildState,
	}
}

func NewCTPV2FromV1WithCustomManifestFetcher(ctx context.Context, v1 map[string]*test_platform.Request, manifestFetcher ManifestFetcher, buildState *build.State) *CTPV2FromV1 {
	if manifestFetcher == nil {
		manifestFetcher = GetBuilderManifestFromContainer
	}
	return &CTPV2FromV1{
		v1: v1,
		v2: &testapi.CTPv2Request{
			Requests: []*testapi.CTPRequest{},
		},
		ctx:             ctx,
		manifestFetcher: manifestFetcher,
		buildState:      buildState,
	}
}

func (builder *CTPV2FromV1) BuildRequest() (map[string]*api.CTPRequest, map[string][]string, map[string]bool) {
	reqsChainMap := map[string][]string{}
	v2sWithKeyList := []*V2WithKey{}
	reqKeyMap := map[string]*api.CTPRequest{}
	dddTrackerMap := map[string]bool{}
	for key, v1Request := range builder.v1 {
		ctpReq := buildCTPRequest(v1Request, builder.buildState)
		builder.v2.Requests = append(builder.v2.Requests, ctpReq)
		v2sWithKeyList = append(v2sWithKeyList, &V2WithKey{Key: key, V2: ctpReq})
		dddTrackerMap[key] = ctpReq.GetSuiteRequest().GetDddSuite()
	}

	v2sWithKeyList, reqsChainMap = GroupV2Requests(builder.ctx, v2sWithKeyList, builder.manifestFetcher)

	for _, v2 := range v2sWithKeyList {
		reqKeyMap[v2.Key] = v2.V2
	}
	return reqKeyMap, reqsChainMap, dddTrackerMap
}
