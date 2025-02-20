// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package build_service

import (
	"context"

	"github.com/stretchr/testify/mock"
	moblabapipb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	"go.chromium.org/infra/cros/recovery/models"
)

// MockBuildService This object is only for testing
//
// Object should provide the same functions that `IBuildServices` interfaces provide.
// TODO: I will write a generator for the interface later to generate this file
type MockBuildService struct {
	mock.Mock
}

// ListBuildTargets Mock the function instead of calling an API.
func (m *MockBuildService) ListBuildTargets(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

// ListModels Mock the function instead of calling an API.
func (m *MockBuildService) ListModels(ctx context.Context, board string) ([]string, error) {
	args := m.Called(ctx, board)
	return args.Get(0).([]string), args.Error(1)
}

// ListAvailableMilestones Mock the function instead of calling an API.
func (m *MockBuildService) ListAvailableMilestones(ctx context.Context, board, model string, filterType FilterType) ([]string, error) {
	args := m.Called(ctx, board, model, filterType)
	return args.Get(0).([]string), args.Error(1)
}

// FindMostStableBuild Mock the function instead of calling an API.
func (m *MockBuildService) FindMostStableBuild(ctx context.Context, board string) (string, error) {
	args := m.Called(ctx, board)
	return args.String(0), args.Error(1)
}

func (m *MockBuildService) FindMostStableBuildByBoardAndModel(ctx context.Context, board, model string) (*models.RecoveryVersion, error) {
	args := m.Called(ctx, board)
	return args.Get(0).(*models.RecoveryVersion), args.Error(1)
}

// ListBuildsForMilestone Mock the function instead of calling an API.
func (m *MockBuildService) ListBuildsForMilestone(ctx context.Context, board, model string, milestone int32, filterType FilterType) ([]*BuildVersion, error) {
	args := m.Called(ctx, board, model, milestone, filterType)
	return args.Get(0).([]*BuildVersion), args.Error(1)
}

// CheckBuildStageStatus Mock the function instead of calling an API.
func (m *MockBuildService) CheckBuildStageStatus(ctx context.Context, board, model, buildVersion, bucketName string) (bool, error) {
	args := m.Called(ctx, board, model, buildVersion, bucketName)
	return args.Bool(0), args.Error(1)
}

// StageBuild Mock the function instead of calling an API.
func (m *MockBuildService) StageBuild(ctx context.Context, board, model, buildVersion, bucketName string, filterType FilterType) (*moblabapipb.BuildArtifact, error) {
	args := m.Called(ctx, board, model, buildVersion, bucketName, filterType)
	return args.Get(0).(*moblabapipb.BuildArtifact), args.Error(1)
}

// Close Mock the function instead of calling an API.
func (m *MockBuildService) Close() error {
	return nil
}
