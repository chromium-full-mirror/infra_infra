// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tools

import (
	"context"
	"fmt"
	"log"
	"path"
	"time"

	"go.chromium.org/chromiumos/test/ctpv2/partner-staging/moblab"

	gax "github.com/googleapis/gax-go/v2"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
)

// MoblabClient interface provides subset of Moblab API methods relevant to CTPV2
type MoblabClient interface {
	StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*moblab.StageBuildOperation, error)
	CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error)
}

// MaxStageTime is the maximum time allowed for staging a build.
const MaxStageTime = 120 * time.Second

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
func StageImageToBucket(ctx context.Context, moblabClient MoblabClient, stageImageParams *StageImageParams, cft bool, log *log.Logger) error {
	buildTarget := fmt.Sprintf("buildTargets/%s/models/%s", stageImageParams.Board, stageImageParams.Model)
	artifactName := fmt.Sprintf("%s/builds/%s/artifacts/%s", buildTarget, stageImageParams.BuildVersion, stageImageParams.Bucket)
	stageReq := &moblabpb.StageBuildRequest{
		Name:   artifactName,
		Filter: stageImageParams.BuildType.String(),
	}

	log.Printf("Staging: %s, Filter: %s\n", artifactName, stageImageParams.BuildType.String())
	if _, err := moblabClient.StageBuild(ctx, stageReq); err != nil {
		log.Printf("Failed to stage %s: %v\n", artifactName, err)
		return err
	}

	var stageStatus *moblabpb.CheckBuildStageStatusResponse
	var err error
	var delay time.Duration = 1 * time.Second
	maxDelay := 10 * time.Second
	totalElapsedTime := time.Duration(0)

	for {
		req := &moblabpb.CheckBuildStageStatusRequest{
			Name: artifactName,
		}
		stageStatus, err = moblabClient.CheckBuildStageStatus(ctx, req)
		if err != nil {
			return err
		}
		if stageStatus.IsBuildStaged {
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

	// TODO(b/343738269) Replace this sleep step with call to MoblabAPI
	// check Staging status once MoblabAPI is updated to include container staging status.
	if cft {
		log.Println("Waiting for CFT container to be staged...")
		time.Sleep(MaxStageTime)
	}
	return nil
}
