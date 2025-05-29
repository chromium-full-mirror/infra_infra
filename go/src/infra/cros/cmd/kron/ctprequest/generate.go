// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctprequest will build and return a CTP request to be handled by the CTP
// BuildBucket builder.
package ctprequest

import (
	"fmt"
	"path"
	"strings"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	requestpb "go.chromium.org/chromiumos/infra/proto/go/test_platform"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"go.chromium.org/infra/cros/cmd/kron/common"
	"go.chromium.org/infra/cros/cmd/kron/configparser"
)

const (
	GSformat                  = "gs://%s/%s"
	ContainerMetadataLocation = "metadata/containers.jsonpb"
	DefaultImageBucket        = "chromeos-image-archive"

	MaxRetry = 3

	Fortnightly = int64(240)
	Weekly      = int64(230)
	Daily       = int64(200)
	PostBuild   = int64(170)

	// CTP requests with a qs account will not require a priority so apply the
	// default swarming value.
	Default = int64(140)
)

// priorityMap returns the proper swarming priority value for the given launch profile type.
var priorityMap = map[suschpb.SchedulerConfig_LaunchCriteria_LaunchProfile]int64{
	suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD:   PostBuild,
	suschpb.SchedulerConfig_LaunchCriteria_DAILY:       Daily,
	suschpb.SchedulerConfig_LaunchCriteria_WEEKLY:      Weekly,
	suschpb.SchedulerConfig_LaunchCriteria_FORTNIGHTLY: Fortnightly,
}

// getSwarmingDimensions reads the configs runOptions dimensions and formats
// them how the CTP request expects them.
func getSwarmingDimensions(config *suschpb.SchedulerConfig) []string {
	dims := []string{}

	for _, dim := range config.GetRunOptions().GetDimensions() {
		str := fmt.Sprintf("%s:%s", dim.Key, dim.Value)

		dims = append(dims, str)
	}
	return dims
}

// getSchedulingFields transforms SuSch SchedulerConfig_PoolOptions into ctp SchedulerConfig_LaunchCriteria_LaunchProfile.
func getSchedulingFields(PoolOptions *suschpb.SchedulerConfig_PoolOptions, launchType suschpb.SchedulerConfig_LaunchCriteria_LaunchProfile) *requestpb.Request_Params_Scheduling {
	schedParams := &requestpb.Request_Params_Scheduling{
		QsAccount: PoolOptions.GetQsAccount(),
	}

	if PoolOptions.GetQsAccount() == "" {
		// Get the priority for the run.
		priority := Default
		if val, ok := priorityMap[launchType]; ok {
			priority = val
		}
		schedParams.Priority = priority

	}

	// Because of the proto typing we need cast the pool to one of these values.
	// In the CTP run the key of managedPool versus unManagedPool matters.
	if managedPool, ok := requestpb.Request_Params_Scheduling_ManagedPool_value[PoolOptions.GetPool()]; ok {
		schedParams.Pool = &requestpb.Request_Params_Scheduling_ManagedPool_{ManagedPool: requestpb.Request_Params_Scheduling_ManagedPool(managedPool)}
	} else {
		schedParams.Pool = &requestpb.Request_Params_Scheduling_UnmanagedPool{UnmanagedPool: PoolOptions.GetPool()}

	}

	return schedParams
}

func getTimeoutSeconds(timeoutMins int32, isStaging bool) int64 {
	timeoutSeconds := int64(timeoutMins) * 60

	if isStaging && timeoutSeconds > common.MaxStagingSeconds {
		timeoutSeconds = common.MaxStagingSeconds
	}

	return timeoutSeconds
}

func getTags(board, model, build, branchTrigger string, config *suschpb.SchedulerConfig) []string {
	tags := []string{
		fmt.Sprintf("build:%s", build),
		fmt.Sprintf("label-pool:%s", config.GetPoolOptions().GetPool()),
		fmt.Sprintf("ctp-fwd-task-name:%s", config.GetName()),
		fmt.Sprintf("label-suite:%s", config.GetSuite()),
		fmt.Sprintf("suite:%s", config.GetSuite()),
		fmt.Sprintf("analytics_name:%s", config.GetAnalyticsName()),
		fmt.Sprintf("branch-trigger:%s", branchTrigger),
	}

	if board != "" {
		tags = append(tags, fmt.Sprintf("label-board:%s", board))
	}
	if model != "" {
		tags = append(tags, fmt.Sprintf("label-model:%s", model))
	}

	return tags
}

func getHardwareAttributes(model string) *requestpb.Request_Params_HardwareAttributes {
	if model != "" {
		return &requestpb.Request_Params_HardwareAttributes{Model: model}
	}

	return nil
}

func getRetryParams(retry bool) *requestpb.Request_Params_Retry {
	if retry {
		return &requestpb.Request_Params_Retry{
			Allow: true,
			Max:   MaxRetry,
		}
	}
	return nil

}

func getTestPlan(config *suschpb.SchedulerConfig) *requestpb.Request_TestPlan {
	testPlan := &requestpb.Request_TestPlan{
		Suite: []*requestpb.Request_Suite{
			{
				Name: config.GetSuite(),
			},
		},
		EnableAutotestSharding: config.GetEnableAutotestSharding(),
		TagCriteria:            config.GetRunOptions().GetTagCriteria(),
		IgnoreVariantCategory:  config.GetRunOptions().GetIgnoreVariantCategory(),
	}

	if config.GetRunOptions().GetMaxInShard() > 0 {
		testPlan.MaxInShard = config.GetRunOptions().GetMaxInShard()
	}

	if config.GetRunOptions().GetTotalShards() > 0 {
		testPlan.TotalShards = config.GetRunOptions().GetTotalShards()
	}

	if config.GetTestArgs() != "" && len(testPlan.GetSuite()) > 0 {
		testPlan.Suite[0].TestArgs = config.GetTestArgs()
	}

	testPlan.Iterations = config.GetRunOptions().GetIterations()

	return testPlan
}

func formBuildImage(buildTarget, buildMilestone, buildVersion string) string {
	return buildTarget + "-release" + "/R" + buildMilestone + "-" + buildVersion
}

// getGCSImageBucket returns the custom image bucket defined by the config or
// the default one used by public release.
func getGCSImageBucket(config *suschpb.SchedulerConfig) string {
	if config.GetRunOptions().GetCrosImageBucket() != "" {
		return config.GetRunOptions().GetCrosImageBucket()
	}

	return DefaultImageBucket
}

// formGCSPath returns a path in the form of gs://<bucket>/<dir>/<dir/...
func formGCSPath(config *suschpb.SchedulerConfig, items ...string) string {
	dirPath := path.Join(items...)
	return fmt.Sprintf(GSformat, getGCSImageBucket(config), dirPath)
}

// BuildCTPRequest takes information from a SuSch config and builds the
// corresponding CTP request.
func BuildCTPRequest(config *suschpb.SchedulerConfig, board, model, buildTarget, buildMilestone, buildVersion, branchTrigger string) *requestpb.Request {
	isStaging := common.IsStagingConfig(config)
	buildImage := formBuildImage(buildTarget, buildMilestone, buildVersion)

	request := &requestpb.Request{
		Params: &requestpb.Request_Params{
			HardwareAttributes: getHardwareAttributes(model),
			SoftwareAttributes: &requestpb.Request_Params_SoftwareAttributes{
				BuildTarget: &chromiumos.BuildTarget{
					Name: board,
				},
			},

			FreeformAttributes: &requestpb.Request_Params_FreeformAttributes{
				SwarmingDimensions: getSwarmingDimensions(config),
			},
			SoftwareDependencies: []*requestpb.Request_Params_SoftwareDependency{
				{
					Dep: &requestpb.Request_Params_SoftwareDependency_ChromeosBuild{
						ChromeosBuild: buildImage,
					},
				},
				{
					Dep: &requestpb.Request_Params_SoftwareDependency_ChromeosBuildGcsBucket{
						ChromeosBuildGcsBucket: getGCSImageBucket(config),
					},
				},
			},
			Scheduling: getSchedulingFields(config.GetPoolOptions(), config.GetLaunchCriteria().GetLaunchProfile()),
			Retry:      getRetryParams(config.GetRunOptions().GetRetry()),
			Metadata: &requestpb.Request_Params_Metadata{
				TestMetadataUrl:        formGCSPath(config, buildImage),
				DebugSymbolsArchiveUrl: formGCSPath(config, buildImage),
				ContainerMetadataUrl:   formGCSPath(config, buildImage, ContainerMetadataLocation),
			},
			Time: &requestpb.Request_Params_Time{
				MaximumDuration: &durationpb.Duration{Seconds: getTimeoutSeconds(config.GetRunOptions().GetTimeoutMins(), isStaging)},
			},
			Decorations: &requestpb.Request_Params_Decorations{
				Tags: getTags(board, model, buildImage, branchTrigger, config),
			},
			RunViaCft:            config.GetRunOptions().GetRunViaCft(),
			RunViaTrv2:           config.GetRunOptions().GetRunViaTrv2(),
			IsAlRun:              strings.HasPrefix(config.GetName(), "AL."),
			TranslateTrv2Request: config.GetRunOptions().GetDynamicTrv2(),
			UserDefinedFilters:   append(config.GetKarbonFilters(), config.GetKoffeeFilters()...),
		},
		TestPlan: getTestPlan(config),
	}
	if config.GetLaunchCriteria().GetLaunchProfile() == suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD_3D {
		request.Params.DddSuite = true
	}

	return request
}

// BuildAllCTPRequests Generates all potential CTP options for the given
// configuration.
// FIX(b/321095387): This needs to build all CTPRequests and not require that
// the targets are passed in.
// TODO: Needs the build milestone and version passed in for proper CTP Requests
// to work.
func BuildAllCTPRequests(config *suschpb.SchedulerConfig, targets configparser.TargetOptions) CTPRequests {
	requests := CTPRequests{}

	for _, target := range targets {
		buildTargets := configparser.GetBuildTargets(target, target.VariantsOnly)
		for _, buildTarget := range buildTargets {
			// If only variants are being targeted then skip the base board
			// target option
			if target.VariantsOnly && buildTarget == configparser.BuildTarget(target.Board) {
				continue
			}

			if len(target.Models) > 0 {
				for _, model := range target.Models {
					request := BuildCTPRequest(config, string(target.Board), model, string(buildTarget), "", "", "")
					requests = append(requests, request)
				}
			} else {
				request := BuildCTPRequest(config, string(target.Board), "", string(buildTarget), "", "", "")
				requests = append(requests, request)
			}
		}
	}

	return requests
}

// AddTagToRequest adds a new string tag in the form of <key>:<value> to the
// request's list of tags.
func AddTagToRequest(key, value string, request *requestpb.Request) *requestpb.Request {
	decorations := request.GetParams().GetDecorations()
	tags := request.GetParams().GetDecorations().GetTags()
	newTag := fmt.Sprintf("%s:%s", key, value)
	tags = append(tags, newTag)

	if decorations != nil {
		decorations.Tags = tags
	}

	return request
}
