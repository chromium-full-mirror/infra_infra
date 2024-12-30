// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package build_service

import (
	"context"

	moblabapipb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	"infra/cros/recovery/models"
)

type BuildVersion struct {
	Version string
	Status  BuildStatus
}

type BuildStatus int

const (
	AVAILABLE BuildStatus = iota
	FAILED
	RUNNING
	ABORTED
)

var FromGCSBucketBuildStatusMap = map[moblabapipb.Build_BuildStatus]BuildStatus{
	moblabapipb.Build_PASS:    AVAILABLE,
	moblabapipb.Build_FAIL:    FAILED,
	moblabapipb.Build_RUNNING: RUNNING,
	moblabapipb.Build_ABORTED: ABORTED,
}

type FilterType string

const (
	Unset    FilterType = ""
	Release             = "release"
	Firmware            = "firmware"
)

// IBuildService is the interface that provide the services
// It should not contain any `Business Logic` here, because it
// is to mock the interface for testing.
type IBuildService interface {
	// ListBuildTargets returns all the board.
	ListBuildTargets(ctx context.Context) ([]string, error)

	// ListModels returns all models by given board.
	ListModels(ctx context.Context, board string) ([]string, error)

	// ListAvailableMilestones returns all available milestones by given board and model.
	ListAvailableMilestones(ctx context.Context, board, model string, filterType FilterType) ([]string, error)

	// ListBuildsForMilestone returns all build versions by given board, model, and milestone.
	ListBuildsForMilestone(ctx context.Context, board, model string, milestone int32, filterType FilterType) ([]*BuildVersion, error)

	// FindMostStableBuild find the stable build version by given board.
	FindMostStableBuild(ctx context.Context, board string) (string, error)

	// FindMostStableBuildByBoardAndModel find the stable recovery version by board and model
	FindMostStableBuildByBoardAndModel(ctx context.Context, board, model string) (*models.RecoveryVersion, error)

	// CheckBuildStageStatus check the build version is staged by given board, model, build version, and bucket name.
	CheckBuildStageStatus(ctx context.Context, board, model, buildVersion, bucketName string) (bool, error)

	// StageBuild stage the build version in the bucket by given board, model, build version, and bucket name.
	StageBuild(ctx context.Context, board, model, buildVersion, bucketName string, filterType FilterType) (*moblabapipb.BuildArtifact, error)

	// Close clean up
	Close() error
}
