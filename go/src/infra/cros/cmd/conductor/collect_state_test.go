// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	bbpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cros/internal/assert"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

func TestCollectState_MaxRetries(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					MaxRetries: 3,
				},
			},
		}})
	build := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
	}

	retries := 0
	for collectState.canRetry(build, "12345") {
		collectState.recordRetry(build, "12345")
		retries += 1
	}
	assert.IntsEqual(t, retries, 3)
}

func TestCollectState_MaxRetriesPerBuild(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					MaxRetries:         3,
					MaxRetriesPerBuild: 2,
				},
			},
		}})
	build := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "paygen",
		},
	}

	retries := 0
	for collectState.canRetry(build, "12345") {
		collectState.recordRetry(build, "12345")
		retries += 1
		build.Id = build.Id + 1
	}
	assert.IntsEqual(t, retries, 2)

	otherBuild := &bbpb.Build{
		Id:     22345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
	}
	retries = 0
	for collectState.canRetry(otherBuild, "22345") {
		collectState.recordRetry(otherBuild, "22345")
		retries += 1
		otherBuild.Id = otherBuild.Id + 1
	}
	assert.IntsEqual(t, retries, 1)
}

type fakeClock struct {
	currentTime int64
}

func (f *fakeClock) Now() int64 {
	return f.currentTime
}

func TestCollectState_CutoffSeconds(t *testing.T) {
	t.Parallel()

	fakeClock := &fakeClock{
		currentTime: 100,
	}

	collectState := initCollectStateTest(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					CutoffSeconds: 300, // Can't retry after time 400.
				},
			},
		}}, fakeClock)
	build := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
	}

	assert.Assert(t, collectState.canRetry(build, "12345"))
	fakeClock.currentTime = 200
	assert.Assert(t, collectState.canRetry(build, "12345"))
	fakeClock.currentTime = 300
	assert.Assert(t, collectState.canRetry(build, "12345"))
	fakeClock.currentTime = 500
	assert.Assert(t, !collectState.canRetry(build, "12345"))
}

func TestCollectState_CutoffPercent(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					CutoffPercent: 0.5,
				},
			},
		},
		initialBuildCount: 4})
	build := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
	}

	retries := 0
	for collectState.canRetry(build, "12345") {
		collectState.recordRetry(build, "12345")
		retries += 1
	}
	// Should only retry 0.5 * 4 = 2 builds.
	assert.IntsEqual(t, retries, 2)
}

func TestCollectState_BuildMatches(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					Status: []int32{
						int32(bbpb.Status_FAILURE),
						int32(bbpb.Status_INFRA_FAILURE),
					},
					BuilderNameRe: []string{
						"coral-.*",
						"eve-.*",
					},
					SummaryMarkdownRe: []string{
						".*source cache.*",
						".*gclient.*",
					},
					FailedCheckpoint: pb.RetryStep_STAGE_ARTIFACTS,
				},
			},
		}})

	outputProperties, err := structpb.NewStruct(map[string]any{})
	assert.NilError(t, err)
	err = bb.SetProperty(outputProperties,
		"retry_summary.STAGE_ARTIFACTS",
		"FAILED")
	assert.NilError(t, err)

	failedBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
		Output: &bbpb.Build_Output{
			Properties: outputProperties,
		},
	}
	assert.Assert(t, collectState.canRetry(failedBuild, "12345"))

	startedOutputProperties, err := structpb.NewStruct(map[string]any{})
	assert.NilError(t, err)
	err = bb.SetProperty(startedOutputProperties,
		"retry_summary.STAGE_ARTIFACTS",
		"STARTED")
	assert.NilError(t, err)

	failedBuildStarted := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
		Output: &bbpb.Build_Output{
			Properties: startedOutputProperties,
		},
	}
	assert.Assert(t, collectState.canRetry(failedBuildStarted, "12345"))

	successfulBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_SUCCESS,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "coral-release-main",
		},
		SummaryMarkdown: "gclient error",
	}
	assert.Assert(t, !collectState.canRetry(successfulBuild, "12345"))
	otherBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "atlas-release-main",
		},
		SummaryMarkdown: "unknown error",
	}
	assert.Assert(t, !collectState.canRetry(otherBuild, "12345"))
}

func TestCollectState_BuildMatches_BeforeCheckpoint(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					Status: []int32{
						int32(bbpb.Status_FAILURE),
						int32(bbpb.Status_INFRA_FAILURE),
					},
					BeforeCheckpoint: pb.RetryStep_STAGE_ARTIFACTS,
				},
			},
		}})

	noRetrySummary := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
	}
	assert.Assert(t, collectState.canRetry(noRetrySummary, "12345"))

	outputProperties, err := structpb.NewStruct(map[string]any{})
	assert.NilError(t, err)
	err = bb.SetProperty(outputProperties,
		"retry_summary", map[string]any{})
	assert.NilError(t, err)
	hasSummary := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
		Output: &bbpb.Build_Output{
			Properties: outputProperties,
		},
	}
	assert.Assert(t, collectState.canRetry(hasSummary, "12345"))

	startedOutputProperties, err := structpb.NewStruct(map[string]any{})
	assert.NilError(t, err)
	err = bb.SetProperty(startedOutputProperties,
		"retry_summary.STAGE_ARTIFACTS",
		"STARTED")
	assert.NilError(t, err)

	hasStep := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
		Output: &bbpb.Build_Output{
			Properties: startedOutputProperties,
		},
	}
	assert.Assert(t, !collectState.canRetry(hasStep, "12345"))
}

func TestCollectState_Status(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					Status: []int32{
						int32(bbpb.Status_FAILURE),
						int32(bbpb.Status_INFRA_FAILURE),
					},
				},
			},
		}})

	successfulBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_SUCCESS,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "coral-release-main",
		},
		SummaryMarkdown: "gclient error",
	}
	assert.Assert(t, !collectState.canRetry(successfulBuild, "12345"))
}

func TestCollectState_BuilderNameRe(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					BuilderNameRe: []string{
						"coral-.*",
						"eve-.*",
					},
				},
			},
		}})

	failedBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
	}
	assert.Assert(t, collectState.canRetry(failedBuild, "12345"))

	successfulBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_SUCCESS,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "coral-release-main",
		},
		SummaryMarkdown: "gclient error",
	}
	assert.Assert(t, collectState.canRetry(successfulBuild, "12345"))
}

func TestCollectState_SummaryMarkdown(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					SummaryMarkdownRe: []string{
						".*source cache.*",
						".*gclient.*",
					},
				},
			},
		}})

	failedBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
	}
	assert.Assert(t, collectState.canRetry(failedBuild, "12345"))

	successfulBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_SUCCESS,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "coral-release-main",
		},
		SummaryMarkdown: "random error",
	}
	assert.Assert(t, !collectState.canRetry(successfulBuild, "12345"))
}

func TestCollectState_FailedCheckpoint(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					FailedCheckpoint: pb.RetryStep_STAGE_ARTIFACTS,
				},
			},
		}})

	outputProperties, err := structpb.NewStruct(map[string]any{})
	assert.NilError(t, err)
	err = bb.SetProperty(outputProperties,
		"retry_summary.STAGE_ARTIFACTS",
		"FAILED")
	assert.NilError(t, err)

	failedBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
		Output: &bbpb.Build_Output{
			Properties: outputProperties,
		},
	}
	assert.Assert(t, collectState.canRetry(failedBuild, "12345"))

	successfulBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_SUCCESS,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "coral-release-main",
		},
		SummaryMarkdown: "gclient error",
	}
	assert.Assert(t, !collectState.canRetry(successfulBuild, "12345"))
}

func TestCollectState_Insufficient(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					BuilderNameRe: []string{
						".*",
					},
					MaxRetries:   3,
					Insufficient: true,
				},
				{
					BuilderNameRe: []string{
						"coral-.*",
						"eve-.*",
					},
					MaxRetries: 5,
				},
			},
		}})

	atlasBuild := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "atlas-release-main",
		},
		SummaryMarkdown: "gclient error",
	}
	// Only matches insufficient rules.
	assert.Assert(t, !collectState.canRetry(atlasBuild, "12345"))

	eveBuild := &bbpb.Build{
		Id:     12346,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		SummaryMarkdown: "wah, I have a bad source cache.",
	}
	assert.Assert(t, collectState.canRetry(eveBuild, "12346"))

	retries := 0
	for collectState.canRetry(eveBuild, "12345") {
		collectState.recordRetry(eveBuild, "12345")
		retries += 1
	}
	assert.IntsEqual(t, retries, 3)
}

func TestCollectState_BuildRuntimeCutoff(t *testing.T) {
	t.Parallel()

	collectState := initCollectState(&collectStateOpts{
		config: &pb.CollectConfig{
			Rules: []*pb.RetryRule{
				{
					// 30 minutes.
					BuildRuntimeCutoff: 30 * 60,
				},
			},
		},
		initialBuildCount: 1})
	startTime := time.Now()
	endTime := startTime.Add(40 * time.Minute)
	build := &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		StartTime: timestamppb.New(startTime),
		EndTime:   timestamppb.New(endTime),
	}
	assert.Assert(t, !collectState.canRetry(build, "12345"))

	// If the build never started, we're not past the runtime cutoff.
	build = &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
	}
	assert.Assert(t, collectState.canRetry(build, "12345"))

	// Malformatted build should not be retried.
	build = &bbpb.Build{
		Id:     12345,
		Status: bbpb.Status_FAILURE,
		Builder: &bbpb.BuilderID{
			Project: "chromeos",
			Bucket:  "release",
			Builder: "eve-release-main",
		},
		StartTime: timestamppb.New(startTime),
	}
	assert.Assert(t, !collectState.canRetry(build, "12345"))
}
