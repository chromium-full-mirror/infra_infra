// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"strings"

	conf "go.chromium.org/chromiumos/config/go"
)

// GetABArtifactPath returns the path to the named artifact from Android Build.
func GetABArtifactPath(buildId, buildTarget, artifactName string) string {
	return fmt.Sprintf(
		AndroidBuildPrefix+"%s/%s/%s",
		buildId, buildTarget, artifactName)
}

// GetABStoragePath returns the path to the named artifact from Android Build, wrapped a StoragePath object.
func GetABStoragePath(buildId, buildTarget, artifactName string) *conf.StoragePath {
	return &conf.StoragePath{
		HostType: conf.StoragePath_ANDROID_BUILD,
		Path:     GetABArtifactPath(buildId, buildTarget, artifactName),
	}
}

// GetABOTAPath returns the Android Build path to the *-ota-*.zip artifact.
// buildTarget is the full target name in Android Build, such as
// brya-trunk_staging-userdebug, whereas board is the short name of the board,
// such as brya.
func GetABOTAPath(buildId, buildTarget, board string) string {
	return GetABArtifactPath(
		buildId,
		buildTarget,
		fmt.Sprintf("%s-ota-%s.zip", board, buildId),
	)
}

// ParseAndroidPath returns the build ID, build target, and artifact name from an Android Build path.
// Typical path: "android-build/build_explorer/artifacts_list/${BUILD_ID}/${BUILD_TARGET}/${ARTIFACT_NAME}"
func ParseAndroidPath(fullPath string) (buildID, buildTarget, artifactName string, err error) {
	if !strings.HasPrefix(fullPath, AndroidBuildPrefix) {
		return "", "", "", fmt.Errorf("path does not have expected prefix '%s': %s", AndroidBuildPrefix, fullPath)
	}
	relativePath := strings.TrimPrefix(fullPath, AndroidBuildPrefix)
	relativePath = strings.TrimPrefix(relativePath, "/")
	parts := strings.SplitN(relativePath, "/", 3)
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("path format invalid after prefix: expected buildID/target/artifactName, got '%s'", relativePath)
	}
	buildID = parts[0]
	buildTarget = parts[1]
	artifactName = parts[2]
	return buildID, buildTarget, artifactName, nil
}
