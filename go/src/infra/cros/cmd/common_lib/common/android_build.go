// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"

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
