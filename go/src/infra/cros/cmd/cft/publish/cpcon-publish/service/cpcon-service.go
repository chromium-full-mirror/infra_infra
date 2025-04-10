// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils"
)

// CpconPublishService serves only one publish request. Not to be confused with a
// server that creates a new service instance for each request.
type CpconPublishService struct {
	LocalArtifactPath           string
	RetryCount                  int
	ServiceAccountCredsFilePath string
	Bucket                      string
	Build                       string
	Suite                       string
	ParentJobID                 string
}

func NewCpconPublishService(req *api.PublishRequest) (*CpconPublishService, error) {
	var m api.PublishCpconMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return nil, fmt.Errorf("improperly formatted input proto metadata, received %v, expected PublishCpconMetadata: %s", req, err)
	}

	bucket, err := resolveGcsPathToBucket(m.GcsPath.GetPath())
	if err != nil {
		return nil, fmt.Errorf("improperly formatted gcspath: %s", err)
	}

	retryCount := 0
	if req.GetRetryCount() > 0 {
		retryCount = int(req.GetRetryCount())
	}

	return &CpconPublishService{
		LocalArtifactPath:           req.GetArtifactDirPath().GetPath(),
		RetryCount:                  retryCount,
		ServiceAccountCredsFilePath: m.GetServiceAccountCredsFilePath().GetPath(),
		Bucket:                      bucket,
		Build:                       m.GetBuild(),
		Suite:                       m.GetSuite(),
		ParentJobID:                 m.GetParentSwarmingTaskId(),
	}, nil
}

func (ts *CpconPublishService) UploadToCpcon(ctx context.Context) error {
	cmd, err := uploadResultsCmd(ctx, CpconUploadRequest{
		ResultsDir:                  ts.LocalArtifactPath,
		ServiceAccountCredsFilePath: ts.ServiceAccountCredsFilePath,
		Bucket:                      ts.Bucket,
		Build:                       ts.Build,
		Suite:                       ts.Suite,
		ParentJobID:                 ts.ParentJobID,
	})
	if err != nil {
		log.Printf("error while creating upload results command: %s", err)
		return fmt.Errorf("error while creating upload results command: %s", err)
	}
	stdout, stderr, err := commonutils.RunCommand(ctx, cmd, "cpcon_upload_results", nil, true)
	if err != nil {
		log.Printf("cpcon upload cmd stdout: %s, cpcon upload cmd stderr: %s", stdout, stderr)
		return fmt.Errorf("error in cpcon upload results: %s", err)
	}
	return nil
}

// resolveGcsPathToBucket translate the gcsUrl for the gcs upload to a bucket
// name which can be used for the cpcon upload tool
func resolveGcsPathToBucket(gcsUrl string) (string, error) {
	re := regexp.MustCompile(`gs:\/\/([^\/]*)\/(.*)`)
	gcsPathMatches := re.FindStringSubmatch(gcsUrl)
	if len(gcsPathMatches) != 3 {
		return "", fmt.Errorf("must be in format gs://bucket-name/results/path/here")
	}
	return gcsPathMatches[1], nil
}
