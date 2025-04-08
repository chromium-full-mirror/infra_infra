// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finders

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/util/finder"

	statuserrors "go.chromium.org/infra/cros/cmd/cft/common/errors"
)

// GetUniqueParfiles extracts unique parfile names from a list of TestCaseMetadata.
func GetUniqueParfiles(mdList []*api.TestCaseMetadata) ([]string, error) {
	parfiles := make(map[string]bool)
	for _, md := range mdList {
		if md.TestCaseInfo == nil || len(md.TestCaseInfo.ExtraInfo) == 0 {
			continue
		}
		executableName, ok := md.TestCaseInfo.ExtraInfo["executable_name"]
		if !ok {
			continue
		}

		parfile := filepath.Base(executableName)

		// Only interested in par files
		if !strings.HasSuffix(parfile, ".par") {
			continue
		}

		parfiles[parfile] = true
	}

	uniqueParfiles := make([]string, 0, len(parfiles))
	for parfile := range parfiles {
		uniqueParfiles = append(uniqueParfiles, parfile)
	}
	return uniqueParfiles, nil
}

// FetchAndInstallParfiles fetches parfiles from GCS and installs them locally.
func FetchAndInstallParfiles(ctx context.Context, logger *log.Logger, gcsBasePath string, parfiles []string) error {
	const moblyInstallDir = "/usr/local/mobly"

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucketName, pf, err := finder.ExtractBucketAndPrefixFromPath(gcsBasePath)
	if err != nil {
		return err
	}

	bucket := client.Bucket(bucketName)
	newestDir, err := finder.FindNewestDirInGcsBucket(ctx, gcsBasePath, bucket, pf)
	if err != nil {
		return err
	}

	for _, parfile := range parfiles {
		gcsPath := filepath.Join(newestDir, parfile)
		logger.Printf("Fetching parfile %s from GCS", gcsPath)

		// Get the file from GCS
		rc, err := bucket.Object(gcsPath).NewReader(ctx)
		if err != nil {
			return fmt.Errorf("error getting reader for %s: %w", gcsPath, err)
		}
		defer rc.Close()

		// Create a local file for the parfile
		localPath := filepath.Join(moblyInstallDir, parfile)
		file, err := os.Create(localPath)
		if err != nil {
			return statuserrors.NewStatusError(statuserrors.IOCreateError, fmt.Errorf("failed to create local file %v: %w", localPath, err))
		}
		defer file.Close()

		// Copy the GCS file to the local file
		if _, err := io.Copy(file, rc); err != nil {
			return fmt.Errorf("failed to copy GCS file %s to local file %s: %w", gcsPath, localPath, err)
		}
		logger.Printf("Successfully fetched and saved parfile to %s", localPath)

		// Change file permissions
		if err := os.Chmod(localPath, 0755); err != nil {
			return fmt.Errorf("failed to change file permissions for %s: %w", localPath, err)
		}

		// Change file ownership to chromeos-test:chromeos-test
		if err := exec.Command("chown", "chromeos-test:chromeos-test", localPath).Run(); err != nil {
			return fmt.Errorf("failed to change file ownership for %s: %w", localPath, err)
		}
		logger.Printf("Successfully changed permissions and ownership for %s", localPath)
	}

	return nil
}
