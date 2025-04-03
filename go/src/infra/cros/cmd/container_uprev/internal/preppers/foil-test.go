// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package preppers contains preppers for configured containers.
package preppers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

const (
	ArtifactDir = "tf_artifact"
)

func fetchMoblyArtifacts(ctx context.Context, dir string) error {
	bucket := "gs://mobly_priv_artifacts"
	version := "grte" // Make this a variable

	artifactPath := fmt.Sprintf("%s/%s", bucket, version)
	localPath := filepath.Join(dir, ArtifactDir)

	if err := common.DownloadGcsFolderAsLocal(ctx, artifactPath, localPath); err != nil {
		return errors.Annotate(err, "foil-test prepper: download %q", artifactPath).Err()
	}

	return nil
}

func fetchTradefedBinary(ctx context.Context, dir string) error {
	bucket := "gs://cros-xts-metadata"
	object := "tradefed.zip"

	localPath := filepath.Join(dir, ArtifactDir, "tradefed")
	os.Mkdir(localPath, common.DirPermission)

	artifactPath := fmt.Sprintf("%s/%s", bucket, object)

	if _, err := common.DownloadGcsFileToLocal(ctx, artifactPath, localPath); err != nil {
		return errors.Annotate(err, "foil-test prepper: download %q", artifactPath).Err()
	}

	return nil
}

// PrepFoilTestInternal implements the prepper for foil-test internal container.
func PrepFoilTestInternal(ctx context.Context, dir string) error {
	os.Mkdir(filepath.Join(dir, ArtifactDir), common.DirPermission)

	err := fetchMoblyArtifacts(ctx, dir)
	if err != nil {
		logging.Infof(ctx, "foil-test prepper: fetchMoblyArtifacts failed, %s", err)
	}

	return nil
}

// PrepFoilTestAosp implements the prepper for foil-test-aosp container for partners.
func PrepFoilTestAosp(ctx context.Context, dir string) error {
	os.Mkdir(filepath.Join(dir, ArtifactDir), common.DirPermission)

	err := fetchTradefedBinary(ctx, dir)
	if err != nil {
		logging.Infof(ctx, "foil-test prepper: fetchTradefedBinary failed, %s", err)
	}

	return nil
}
