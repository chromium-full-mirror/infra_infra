// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package build_service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/option"
	moblabapipb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/recovery/models"
	moblabapi "go.chromium.org/infra/cros/satlab/common/google.golang.org/google/chromeos/moblab"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/collection"
	"go.chromium.org/infra/cros/satlab/common/utils/parser"
)

// PageSize The number of items to return in a page
const PageSize = 1000

// ParseBuildTargetsPath compose the path by given board.
func ParseBuildTargetsPath(board string) string {
	// TODO find the better way to do
	return fmt.Sprintf("buildTargets/%s", board)
}

// ParseModelPath compose the path by given board and model.
func ParseModelPath(board string, model string) string {
	// TODO find the better way to do
	return fmt.Sprintf("buildTargets/%s/models/%s", board, model)
}

// ParseBuildArtifactPath compose the path by given board, model, buildVersion, and bucket.
func ParseBuildArtifactPath(board string, model string, buildVersion string, bucket string) string {
	// TODO find the better way to do
	return fmt.Sprintf("buildTargets/%s/models/%s/builds/%s/artifacts/%s", board, model, buildVersion, bucket)
}

// BuildServiceImpl is an object for connecting the build client.
type BuildServiceImpl struct {
	// client the `BuildClient`
	client *moblabapi.BuildClient
}

// New sets up the `BuildClient` and returns a BuildConnector.
// The service account is set in the global environment.
func New(ctx context.Context) (IBuildService, error) {
	// create moblab client using service account json file
	client, err := moblabapi.NewBuildClient(ctx, option.WithCredentialsFile(site.GetServiceAccountPath()))
	if err != nil {
		return nil, err
	}
	return &BuildServiceImpl{
		client: client,
	}, nil
}

// ListBuildTargets returns all the board.
func (b *BuildServiceImpl) ListBuildTargets(ctx context.Context) ([]string, error) {
	log.Println("Trying to list build targets")

	req := &moblabapipb.ListBuildTargetsRequest{
		PageSize: PageSize,
	}

	iter := b.client.ListBuildTargets(ctx, req)
	res, err := collection.Collect(
		iter.Next,
		func(board *moblabapipb.BuildTarget) (string, error) {
			return board.GetName(), nil
		},
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}

// ListModels returns all models by given board.
//
// string board is the board name that we use it as a filter.
func (b *BuildServiceImpl) ListModels(ctx context.Context, board string) ([]string, error) {
	log.Println("Trying to list models")

	parent := ParseBuildTargetsPath(board)

	req := &moblabapipb.ListModelsRequest{
		Parent:   parent,
		PageSize: PageSize,
	}

	iter := b.client.ListModels(ctx, req)

	res, err := collection.Collect(
		iter.Next,
		func(model *moblabapipb.Model) (string, error) {
			return model.GetName(), nil
		},
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}

// ListAvailableMilestones returns all available milestones by given board and model.
//
// string board is the board name that we use it as a filter.
// string model is the model name that we use it as a filter.
func (b *BuildServiceImpl) ListAvailableMilestones(ctx context.Context, board, model string, filterType FilterType) ([]string, error) {
	filter := toFilter(filterType)

	fm := &field_mask.FieldMask{
		Paths: []string{"milestone"},
	}

	req := &moblabapipb.ListBuildsRequest{
		Parent:   ParseModelPath(board, model),
		ReadMask: fm,
		GroupBy:  fm,
		PageSize: PageSize,
		Filter:   filter,
	}

	iter := b.client.ListBuilds(ctx, req)

	res, err := collection.Collect(
		iter.Next,
		func(build *moblabapipb.Build) (string, error) {
			milestone, err := parser.ExtractMilestoneFrom(build.GetMilestone())
			if err != nil {
				log.Printf("the milestone format isn't match %v\n", build.GetMilestone())
				return "", err
			}
			return milestone, nil
		},
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (b *BuildServiceImpl) findMostStableBuildByBoard(ctx context.Context, board string) (*moblabapipb.Build, error) {
	buildTarget := ParseBuildTargetsPath(board)

	req := &moblabapipb.FindMostStableBuildRequest{
		BuildTarget: buildTarget,
	}

	resp, err := b.client.FindMostStableBuild(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.GetBuild(), nil
}

func (b *BuildServiceImpl) findStableBuildByBoardAndModel(ctx context.Context, board, model string) (*moblabapipb.Build, error) {
	path := ParseModelPath(board, model)

	req := &moblabapipb.FindMostStableBuildRequest{
		Model: path,
	}

	resp, err := b.client.FindMostStableBuild(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.GetBuild(), nil
}

func buildToOS(milestone, build string) string {
	return fmt.Sprintf("R%s-%s", milestone, build)
}

// FindMostStableBuild find the stable build version by given board.
//
// string board is the board name that we use it as a filter.
func (b *BuildServiceImpl) FindMostStableBuild(ctx context.Context, board string) (string, error) {
	resp, err := b.findMostStableBuildByBoard(ctx, board)
	if err != nil {
		return "", err
	}

	milestone, err := parser.ExtractMilestoneFrom(resp.GetMilestone())
	if err != nil {
		return "", fmt.Errorf("milestone pattern doesn't match %v\n", resp.GetMilestone())
	}

	return buildToOS(milestone, resp.GetBuildVersion()), nil
}

// FindMostStableBuildByBoardAndModel find the stable recovery version by board and model
func (b *BuildServiceImpl) FindMostStableBuildByBoardAndModel(ctx context.Context, board, model string) (*models.RecoveryVersion, error) {
	resp, err := b.findStableBuildByBoardAndModel(ctx, board, model)
	if err != nil {
		return nil, err
	}
	milestone, err := parser.ExtractMilestoneFrom(resp.GetMilestone())
	os := buildToOS(milestone, resp.GetBuildVersion())
	fw := resp.GetRwFirmwareVersion()
	fwBuildVersion, err := parser.ExtractFwBuildVersionFrom(fw)
	if err != nil {
		return nil, err
	}

	// We don't want to list all milestones and use the build version to filter the correct one.
	// We can use `CheckBuildStageStatus` to get the correct milestone.
	req := &moblabapipb.CheckBuildStageStatusRequest{
		Name:   ParseBuildArtifactPath(board, model, fwBuildVersion, site.GetGCSImageBucket()),
		Filter: "type=firmware",
	}

	res, err := b.client.CheckBuildStageStatus(ctx, req)
	if err != nil {
		return nil, err
	}

	return &models.RecoveryVersion{
		Board:     board,
		Model:     model,
		OsImage:   os,
		FwVersion: fw,
		// The source path includes the correct milestone and build
		FwImage: res.SourceBuildArtifact.Path,
	}, nil

}

// ListBuildsForMilestone returns all build versions by given board, model, and milestone.
//
// string board is the board name that we use it as a filter.
// string model is the model name that we use it as a filter.
// int32 milestone is the milestone that we use it as a filter.
func (b *BuildServiceImpl) ListBuildsForMilestone(
	ctx context.Context,
	board string,
	model string,
	milestone int32,
	filterType FilterType,
) ([]*BuildVersion, error) {
	filter := fmt.Sprintf("milestone=milestones/%d", milestone)
	f := toFilter(filterType)
	filter = fmt.Sprintf("%s %s", filter, f)
	req := &moblabapipb.ListBuildsRequest{
		Parent:   ParseModelPath(board, model),
		Filter:   filter,
		PageSize: PageSize,
	}

	iter := b.client.ListBuilds(ctx, req)

	res, err := collection.Collect(
		iter.Next,
		func(build *moblabapipb.Build) (*BuildVersion, error) {
			status := FromGCSBucketBuildStatusMap[build.GetStatus()]
			return &BuildVersion{
				Version: build.GetBuildVersion(),
				Status:  status,
			}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// CheckBuildStageStatus check the build version is staged by given board, model, build version, and bucket name.
//
// string board is the board name that we use it as a filter.
// string model is the model name that we use it as a filter.
// string buildVersion is the build version that we use it as a filter.
// string bucketName the bucket we need to check the build version is in this bucket.\fc
func (b *BuildServiceImpl) CheckBuildStageStatus(
	ctx context.Context,
	board string,
	model string,
	buildVersion string,
	bucketName string,
) (bool, error) {
	req := &moblabapipb.CheckBuildStageStatusRequest{
		Name: ParseBuildArtifactPath(board, model, buildVersion, bucketName),
	}

	res, err := b.client.CheckBuildStageStatus(ctx, req)
	if err != nil {
		return false, err
	}

	return res.IsBuildStaged, nil
}

func toFilter(filterType FilterType) string {
	filter := ""
	if filterType == Firmware {
		filter = "type=firmware"
	} else if filterType == Release {
		filter = "type=release"
	}
	return filter
}

// StageBuild stage the build version in the bucket by given board, model, build version, and bucket name.
//
// string board is the board that we want to stage.
// string model is the model that we want to stage.
// string buildVersion is the build version that we want to stage.
// string bucketName which bucket we want to put the build version in.
func (b *BuildServiceImpl) StageBuild(ctx context.Context,
	board string,
	model string,
	buildVersion string,
	bucketName string,
	filterType FilterType,
) (*moblabapipb.BuildArtifact, error) {
	artifactName := ParseBuildArtifactPath(board, model, buildVersion, bucketName)
	filter := toFilter(filterType)
	req := &moblabapipb.StageBuildRequest{
		Name:   artifactName,
		Filter: filter,
	}

	_, err := b.client.StageBuild(ctx, req)
	if err != nil {
		return nil, err
	}

	// Use polling here because we encountered
	// The GRPC target is not implemented on the server, host: chromeosmoblab.googleapis.com, method: /google.longrunning.Operations/GetOperation.
	var stageStatus *moblabapipb.CheckBuildStageStatusResponse
	c := 10
	for {
		c--
		req := &moblabapipb.CheckBuildStageStatusRequest{
			Name:   artifactName,
			Filter: filter,
		}

		stageStatus, err = b.client.CheckBuildStageStatus(ctx, req)
		logging.Infof(ctx, "check build stage status: %v", stageStatus)
		if err != nil {
			logging.Errorf(ctx, "check build stage status error: %s", err.Error())
			return nil, err
		}

		if stageStatus.IsBuildStaged {
			break
		}
		if c == 0 {
			return nil, errors.New("stage not completed within 10 retries")
		}

		time.Sleep(time.Second * time.Duration(10-c))
	}

	return stageStatus.StagedBuildArtifact, nil
}

// Close to close the client connection.
func (b *BuildServiceImpl) Close() error {
	return b.client.Close()
}
