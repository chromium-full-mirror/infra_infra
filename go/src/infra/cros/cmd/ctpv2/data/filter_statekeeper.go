// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"container/list"

	"cloud.google.com/go/bigquery"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

// FilterStateKeeper represents all the data pre local test execution flow requires.
type FilterStateKeeper struct {
	interfaces.StateKeeper

	CtpReq                  *testapi.CTPRequest
	InitialInternalTestPlan *testapi.InternalTestplan
	TestPlanStates          []*testapi.InternalTestplan
	Scheduler               testapi.SchedulerInfo_Scheduler
	SuiteTestResults        map[string]*TestResults
	BuildsMap               map[string]*BuildRequest
	Config                  *config.Config
	DockerKeyFile           string
	CTPversion              string
	Environment             string
	IsPartnerRun            bool
	HasAshChrome            bool
	// For V1 req, it will be key provided with input,
	// for direct v2, this will be index of the request.
	// Cannot use suite here coz two different suites can be present with different
	// suite metadata in the request.
	RequestKey string

	// Al run related
	AlStateInfo *AlStateInfo
	IsAlRun     bool

	// Build related
	BuildState *build.State

	// Container info queue
	FiltersQueue            *list.List
	ContainerRequestChannel chan commontypes.ContainerManagementRequest
	ContainerLogsChannel    chan *commontypes.ContainerLogInfo

	// Dictionaries
	ContainerMetadataMap map[string]*buildapi.ContainerImageInfo

	// Tools and their related dependencies
	Ctr *crostoolrunner.CrosToolRunner

	MiddledOutResp *MiddleOutResponse

	// BQ Client for writing CTP level task info to.
	BQClient *bigquery.Client

	ExecutionError error

	//Curating logs to be consumed by AI for execution context
	ExecutionAIContext string
}

// AppendToAIExecutionContext exposes an api to easily append to AIExecutionContext
func (sk *FilterStateKeeper) AppendToAIExecutionContext(context string) {
	sk.ExecutionAIContext = sk.ExecutionAIContext + "\n" + context
}
