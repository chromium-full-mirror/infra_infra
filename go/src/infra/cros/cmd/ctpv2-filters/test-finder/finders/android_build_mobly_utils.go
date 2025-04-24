// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finders

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/finder"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/common"
)

// GetAndroidBuildMoblySourceData pulls test metadata for Android build mobly tests.
func GetAndroidBuildMoblySourceData(ctx context.Context, log *log.Logger, gcsBasePath string, dir string) ([]*api.TestCaseMetadata, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucketName, pf, err := finder.ExtractBucketAndPrefixFromPath(gcsBasePath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(pf, dir)
	log.Printf("Fetching artifacts from bucket: %s, path %s", bucketName, fullPath)

	bucket := client.Bucket(bucketName)

	data, err := common.PullAllFilesFromGcsDir(ctx, bucket, fullPath, ".json")
	if err != nil {
		return nil, err
	}

	// Can just deserialize .json directly.
	metadata := []*api.TestCaseMetadata{}
	for _, bytes := range data {
		req := &api.TestCaseMetadataList{}
		if err := protojson.Unmarshal(bytes, req); err != nil {
			// Just log for now.
			log.Printf("Failed to unmarshal data: %s, %v", string(bytes), err)
		}
		metadata = append(metadata, req.Values...)
	}

	return metadata, nil
}

// PathOrLatest returns the directory path if it exists, or checks for a LATEST
// file which points at the default path.
//
// If object is empty, then "latest" is already returned.
func PathOrLatest(ctx context.Context, gcsBasePath string, object string) (string, bool, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", false, fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucketName, pf, err := finder.ExtractBucketAndPrefixFromPath(gcsBasePath)
	if err != nil {
		return "", false, err
	}

	bucket := client.Bucket(bucketName)
	if object != "" {
		if exists, err := common.ObjectExists(ctx, bucket, object); err != nil {
			return "", false, fmt.Errorf("failed to check if object exists: %w", err)
		} else if exists {
			return object, true, nil

		}
	}

	data, err := common.PullAllFilesFromGcsDir(ctx, bucket, pf, "LATEST")
	if err != nil {
		return "", false, fmt.Errorf("failed to fetch LATEST file in %s/%s: %w", bucketName, pf, err)
	}
	if len(data) != 1 {
		return "", false, fmt.Errorf("failed to get latest Android version, expected to find 1 LATEST file, found %d", len(data))
	}
	return string(data[0]), false, nil
}

// GetUniqueMoblyZipArtifacts fetches all artifacts listed in the TestCaseInfo.ExtraInfo.
//
// AOSP Mobly tests can contain additional data required by the executable so
// we need to download the entire .zip file rather than a single exec.
func GetUniqueMoblyZipArtifacts(mdList []*api.TestCaseMetadata) ([]string, error) {
	artifacts := make(map[string]bool)
	for _, md := range mdList {
		if md.TestCaseInfo == nil || len(md.TestCaseInfo.ExtraInfo) == 0 {
			continue
		}
		artifactName, ok := md.TestCaseInfo.ExtraInfo["artifact_name"]
		if !ok {
			continue
		}

		artifact := filepath.Base(artifactName)

		// Only interested in .zip files for now.
		if !strings.HasSuffix(artifact, ".zip") {
			continue
		}

		artifacts[artifact] = true
	}

	uniqueZipFiles := make([]string, 0, len(artifacts))
	for parfile := range artifacts {
		uniqueZipFiles = append(uniqueZipFiles, parfile)
	}
	return uniqueZipFiles, nil
}

// FetchAndInstallZipArtifacts fetches test artifacts from the GS Bucket and installs them locally.
func FetchAndInstallZipArtifacts(ctx context.Context, logger *log.Logger, gcsBasePath, dir string, artifacts []string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	tempDir, err := ioutil.TempDir(os.TempDir(), "artifacts")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	bucketName, pf, err := finder.ExtractBucketAndPrefixFromPath(gcsBasePath)
	if err != nil {
		return err
	}

	bucket := client.Bucket(bucketName)
	if err := common.PullFilesFromGcsDirLocal(ctx, logger, bucket, filepath.Join(pf, dir), tempDir, artifacts); err != nil {
		return err
	}

	for _, artifact := range artifacts {
		path := filepath.Join(tempDir, artifact)
		files, err := common.UnzipFile(path, moblyInstallDir)
		if err != nil {
			return err
		}
		for _, file := range files {
			logger.Printf("From %q unpacked %q", artifact, file)
		}
	}
	return nil
}
