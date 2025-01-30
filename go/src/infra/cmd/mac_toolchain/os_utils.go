// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"
	"regexp"

	"go.chromium.org/luci/common/errors"
)

var MajorVersionRegex = regexp.MustCompile(`(\d+)\..*`)

func isMacOSVersionOrLater(ctx context.Context, requiredMajorVersion string) (bool, error) {
	curVersion, err := getCurVersion(ctx)
	if err != nil {
		return false, errors.Annotate(err, "failed to get current os version").Err()
	}

	result := MajorVersionRegex.FindStringSubmatch(curVersion)
	if len(result) == 0 {
		return false, errors.Reason("unable to parse MacOS Version from %s", curVersion).Err()
	}

	curMajorVersion := result[1]

	return curMajorVersion >= requiredMajorVersion, nil
}

func isMacOS13OrLater(ctx context.Context) (bool, error) {
	return isMacOSVersionOrLater(ctx, "13")
}

func isMacOS14OrLater(ctx context.Context) (bool, error) {
	return isMacOSVersionOrLater(ctx, "14")
}

func getCurVersion(ctx context.Context) (string, error) {
	out, err := RunOutput(ctx, "sw_vers", "-productVersion")
	if err != nil {
		return "", errors.Annotate(err, "failed to run sw_vers -productVersion").Err()
	}
	return out, nil
}

func renameDirectory(originDirectoryName string, destinationDirectoryName string) error {
	// Check if the origin directory exists
	if _, err := os.Stat(originDirectoryName); os.IsNotExist(err) {
		return errors.Annotate(err, "Error: origin directory %s does not exist", originDirectoryName).Err()
	}

	// Check if the destination directory exists
	if _, err := os.Stat(destinationDirectoryName); err == nil {
		// Destination directory exists, remove it
		err = os.RemoveAll(destinationDirectoryName)
		if err != nil {
			return errors.Annotate(err, "Error removing existing directory: %s", destinationDirectoryName).Err()
		}
	}

	// Rename the directory
	err := os.Rename(originDirectoryName, destinationDirectoryName)
	if err != nil {
		return errors.Annotate(err, "Error renaming directory.").Err()
	}

	return nil
}
