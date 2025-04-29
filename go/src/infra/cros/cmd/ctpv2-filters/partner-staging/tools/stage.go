// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tools provides tooling for staging builds through moblab.
package tools

import (
	"context"
	"fmt"
	"log"
	"path"
	"time"

	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
)

// MaxStageTime is the maximum time allowed for staging a build.
const MaxStageTime = 180 * time.Second

// The type of the build artifact
type BuildType string

// Supports two build types, release builds and firmware builds
const (
	BuildTypeReleas   BuildType = "type=release"
	BuildTypeFirmware BuildType = "type=firmware"
)

// String returns the string representation of the BuildType.
func (s BuildType) String() string {
	return string(s)
}

// StageImageParams is a struct that holds parameters for staging a Chrome OS image.
type StageImageParams struct {
	Bucket       string
	Board        string
	Model        string
	BuildVersion string
	BuildType    BuildType
}

// StageImageToBucket stages the specified Chrome OS image to the user GCS bucket
func StageImageToBucket(ctx context.Context, moblabClient MobLabAPI, stageImageParams *StageImageParams, cft bool, log *log.Logger) error {
	buildTarget := fmt.Sprintf("buildTargets/%s/models/%s", stageImageParams.Board, stageImageParams.Model)
	artifactName := fmt.Sprintf("%s/builds/%s/artifacts/%s", buildTarget, stageImageParams.BuildVersion, stageImageParams.Bucket)
	stageReq := &moblabpb.StageBuildRequest{
		Name:   artifactName,
		Filter: stageImageParams.BuildType.String(),
	}

	log.Printf("Staging: %s, Filter: %s\n", artifactName, stageImageParams.BuildType.String())
	stageBuildMetadata, err := moblabClient.StageBuild(ctx, stageReq)
	if err != nil {
		log.Printf("Failed to stage %s: %v\n", artifactName, err)
		return err
	}

	req := &moblabpb.CheckBuildStageStatusRequest{Name: artifactName}
	if cloudBuild := stageBuildMetadata.CloudBuild; cloudBuild != nil {
		req.Filter = fmt.Sprintf("cloud_build_id=%s", cloudBuild.Id)
	}
	var stageStatus *moblabpb.CheckBuildStageStatusResponse
	var delay = 1 * time.Second
	maxDelay := 10 * time.Second
	totalElapsedTime := time.Duration(0)

	for {
		stageStatus, err = moblabClient.CheckBuildStageStatus(ctx, req)
		if err != nil {
			return err
		}
		if stageStatus.IsBuildStaged && (stageStatus.CloudBuild == nil || stageStatus.CloudBuild.Status == moblabpb.CloudBuild_SUCCEEDED) {
			break
		}
		if totalElapsedTime >= MaxStageTime {
			return fmt.Errorf("stage %s not completed within %v seconds", artifactName, MaxStageTime.Seconds())
		}

		log.Printf("Build %s not staged yet. Retrying in %v...\n", artifactName, delay)
		time.Sleep(delay)
		totalElapsedTime += delay

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	destPath := stageStatus.StagedBuildArtifact.Path
	log.Printf("Artifacts staged to %s\n", path.Join(stageImageParams.Bucket, destPath))
	return nil
}
