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
	TradefedMetadataFile = "tradefed_metadata.pb"
	ArtifactDir          = "tf_artifact"
)

type Module struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Abis          []string `json:"abis"`
	Parameters    []string `json:"parameters"`
	BugComponents []int    `json:"bug_components"`
	Owners        []string `json:"owners"`
}

type TargetBuild struct {
	Target  string `json:"target"`
	BuildId string `json:"build_id"`
	Abi     string `json:"abi"`
}

type TradefedMetadata struct {
	Version      string        `json:"version"`
	Suite        string        `json:"suite"`
	Branch       string        `json:"branch"`
	BuildId      string        `json:"build_id"`
	Targets      []string      `json:"targets"`
	Modules      []Module      `json:"modules"`
	TargetBuilds []TargetBuild `json:"target_builds"`
}

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

// PrepFoilTestInternal implements the prepper for foil-test internal container.
func PrepFoilTestInternal(ctx context.Context, dir string) error {
	os.Mkdir(filepath.Join(dir, ArtifactDir), common.DirPermission)

	err := fetchMoblyArtifacts(ctx, dir)
	if err != nil {
		logging.Infof(ctx, "foil-test prepper: fetchMoblyArtifacts failed, %s", err)
	}

	return nil
}
