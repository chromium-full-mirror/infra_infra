// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	moblabapipb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/models"
	"go.chromium.org/infra/cros/satlab/common/services/build_service"
	"go.chromium.org/infra/cros/satlab/common/site"
)

func setupTempStableVersionDir(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0777); err != nil {
		return err
	}
	return nil
}

// TestWriteLocalStableVersion tests stable version file creation
func TestWriteLocalStableVersion(t *testing.T) {
	t.Parallel()

	rv := &models.RecoveryVersion{
		Board:     "zork",
		Model:     "gumboz",
		OsImage:   "R115-15474.70.0",
		FwVersion: "Google_Berknip.13434.356.0",
		FwImage:   "zork-firmware/R87-13434.819.0",
	}

	// Perform our test in a temporary file managed by the test framework.
	path := filepath.Join(t.TempDir(), "tmp", "recovery_versions")

	if err := setupTempStableVersionDir(path); err != nil {
		t.Errorf("Unexpected err: %v", err)
	}
	if err := WriteLocalStableVersion(rv, path); err != nil {
		t.Errorf("Unexpected err: %v", err)
	}
	file := fmt.Sprintf("%s%s-%s.json", path, rv.Board, rv.Model)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		t.Errorf("Unexpected err: %v", err)
	}

	savedRecoveryVersion, err := os.ReadFile(file)
	if err != nil {
		t.Errorf("Unexpected err: %v", err)
	}

	rv2 := &models.RecoveryVersion{}
	_ = json.Unmarshal([]byte(savedRecoveryVersion), rv2)

	if !reflect.DeepEqual(rv, rv2) {
		t.Errorf("Recovery version saved incorrectly")
	}
}

func TestStageAndWriteLocalStableVersionShouldWork(t *testing.T) {
	t.Parallel()

	const (
		board        = "zork"
		model        = "gumboz"
		osImage      = "R115-15474.70.0"
		osImageBuild = "15474.70.0"
		fwVersion    = "Google_Berknip.13434.356.0"
		fwImage      = "zork-firmware/R87-13434.819.0"
		fwImageBuild = "13434.819.0"
	)

	bucketName := site.GetGCSImageBucket()
	mockArtifactOs := &moblabapipb.BuildArtifact{
		Build:  osImageBuild,
		Name:   fmt.Sprintf("buildTargets/%s/models/%s/builds/%s/artifacts/%s", board, model, osImageBuild, bucketName),
		Bucket: bucketName,
		Path:   fmt.Sprintf("%s-release/%s", board, osImage),
	}
	mockArtifactFw := &moblabapipb.BuildArtifact{
		Build:  osImageBuild,
		Name:   fmt.Sprintf("buildTargets/%s/models/%s/builds/%s/artifacts/%s", board, model, fwImageBuild, bucketName),
		Bucket: bucketName,
		Path:   fwImage,
	}

	ctx := context.Background()
	mockBuildService := new(build_service.MockBuildService)
	mockBuildService.On("StageBuild", ctx, board, model, osImageBuild, bucketName, build_service.Release).Return(mockArtifactOs, nil)
	mockBuildService.On("StageBuild", ctx, board, model, fwImageBuild, bucketName, build_service.Firmware).Return(mockArtifactFw, nil)
	path := filepath.Join(t.TempDir(), "tmp", "recovery_versions")
	if err := setupTempStableVersionDir(path); err != nil {
		t.Errorf("Setup temp dir for stable version: %v", err)
	}

	rv := &models.RecoveryVersion{
		Board:     board,
		Model:     model,
		OsImage:   osImage,
		FwVersion: fwVersion,
		FwImage:   fwImage,
	}

	if err := StageAndWriteLocalStableVersion(ctx, mockBuildService, rv, path); err != nil {
		t.Errorf("StageAndWriteLocalStableVersion() error = %v", err)
	}

}
