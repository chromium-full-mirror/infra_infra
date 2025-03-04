// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"cloud.google.com/go/bigquery"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

// PrePostFilterStateKeeper represents all the data pre and post filter execution flow requires.
type PrePostFilterStateKeeper struct {
	interfaces.StateKeeper

	// Requests
	CtpV1Requests           map[string]*test_platform.Request
	CtpV2Request            *api.CTPv2Request          // Direct Ctpv2 Request
	V1KeyToCTPv2Req         map[string]*api.CTPRequest // V2 reqs converted from V1
	RequestToTargetChainMap map[string]map[string]string
	ExecuteResponses        *steps.ExecuteResponses
	DddTrackerMap           map[string]bool // v1 request key 3d bool map
	IsPartnerRun            bool
	SchedukeDisallowList    []string

	// Al run related
	AlStateInfo *AlStateInfo

	// Results
	AllTestResults map[string][]*TestResults

	// Tools and their related dependencies
	Ctr                   *crostoolrunner.CrosToolRunner
	DockerKeyFileLocation string

	// BQ Client for writing CTP level task info to.
	BQClient   *bigquery.Client
	BuildState *build.State
}
