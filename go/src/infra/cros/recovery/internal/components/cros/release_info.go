// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/logger"
)

const (
	releaseExtactValueGlob = "cat /etc/lsb-release | grep %s"
	releaseValueRegexpGlob = `%s=(\S+)`
	releaseBoardKey        = "CHROMEOS_RELEASE_BOARD"
	releaseTrackKey        = "CHROMEOS_RELEASE_TRACK"
	releaseBuilderPath     = "CHROMEOS_RELEASE_BUILDER_PATH"
)

// ExtractValueFromLeaseInfo reads release info and extract value by provided key.
func ExtractValueFromLeaseInfo(ctx context.Context, run components.Runner, log logger.Logger, key string) (string, error) {
	extactValueCommand := fmt.Sprintf(releaseExtactValueGlob, key)
	output, err := run(ctx, time.Minute, extactValueCommand)
	if err != nil {
		return "", errors.Annotate(err, "extract value from release info").Err()
	}
	valueRegexpCommand := fmt.Sprintf(releaseValueRegexpGlob, key)
	compiledRegexp, err := regexp.Compile(valueRegexpCommand)
	if err != nil {
		return "", errors.Annotate(err, "extract value from release info").Err()
	}
	matches := compiledRegexp.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", errors.Reason("extract value from release info: values is not found").Err()
	}
	value := matches[1]
	log.Debugf("Release info %q:%q", key, value)
	return value, nil
}

// ReleaseBoard reads release board info from lsb-release.
func ReleaseBoard(ctx context.Context, run components.Runner, log logger.Logger) (string, error) {
	board, err := ExtractValueFromLeaseInfo(ctx, run, log, releaseBoardKey)
	if err != nil {
		return "", errors.Annotate(err, "release %q", releaseBoardKey).Err()
	}
	log.Debugf("Release %q: %q.", releaseBoardKey, board)
	return board, nil
}

// ReleaseTrack reads release track info from lsb-release.
func ReleaseTrack(ctx context.Context, run components.Runner, log logger.Logger) (string, error) {
	track, err := ExtractValueFromLeaseInfo(ctx, run, log, releaseTrackKey)
	if err != nil {
		return "", errors.Annotate(err, "release %q", releaseTrackKey).Err()
	}
	log.Debugf("Release %q: %q.", releaseTrackKey, track)
	return track, nil
}

// ReleaseBuildPath reads release build info from lsb-release.
func ReleaseBuildPath(ctx context.Context, run components.Runner, log logger.Logger) (string, error) {
	buildPath, err := ExtractValueFromLeaseInfo(ctx, run, log, releaseBuilderPath)
	if err != nil {
		return "", errors.Annotate(err, "release %q", releaseBuilderPath).Err()
	}
	log.Debugf("Release %q: %q.", releaseBuilderPath, buildPath)
	return buildPath, nil
}

// ParseReleaseVersionFromBuilderPath parses the ChromeOSReleaseVersion from
// a CHROMEOS_RELEASE_BUILDER_PATH from lsb-release.
//
// For example, a releaseBuilderPath of "board-release/R90-13816.47.0" would
// have a release version of "13816.47.0".
func ParseReleaseVersionFromBuilderPath(releaseBuilderPath string) (string, error) {
	if releaseBuilderPath == "" {
		return "", errors.Reason("cannot parse version from empty string").Err()
	}
	pathParts := strings.Split(releaseBuilderPath, "/")
	versionPathSegmentParts := strings.Split(pathParts[len(pathParts)-1], "-")
	releaseVersion := versionPathSegmentParts[len(versionPathSegmentParts)-1]
	releaseVersionRegex := regexp.MustCompile(`(\d+)(\.\d+)*`)
	if !releaseVersionRegex.MatchString(releaseVersion) {
		return "", errors.Reason("parsed invalid release version %q from release builder path %q", releaseVersion, releaseBuilderPath).Err()
	}
	return releaseVersion, nil
}

// ChromeOSReleaseVersion is the integral representation of a ChromeOS release
// version. The version is read from left to right, with each segment separated
// by "." parsed as individual integers.
//
// For example, a version of "13816.47.0" is equivalent to
// ChromeOSReleaseVersion{13816,47,0}.
type ChromeOSReleaseVersion []int

// String returns the ChromeOSReleaseVersion as a string, with each release
// segment joined by a period.
//
// For example, ChromeOSReleaseVersion{13816,47,0} would return "13816.47.0".
func (v ChromeOSReleaseVersion) String() string {
	var segments []string
	for _, segment := range v {
		segments = append(segments, strconv.Itoa(segment))
	}
	return strings.Join(segments, ".")
}

// ParseChromeOSReleaseVersion parses the release version string from the
// lsb-release into its integral parts as a ChromeOSReleaseVersion instance.
func ParseChromeOSReleaseVersion(version string) (ChromeOSReleaseVersion, error) {
	if version == "" {
		return nil, errors.Reason("cannot parse version from empty string").Err()
	}
	var result ChromeOSReleaseVersion
	for _, segmentStr := range strings.Split(version, ".") {
		segmentInt, err := strconv.Atoi(segmentStr)
		if err != nil {
			return nil, errors.Annotate(err, "failed to parse chromeos release version %q", version).Err()
		}
		result = append(result, segmentInt)
	}
	return result, nil
}

// IsChromeOSReleaseVersionLessThan compares two ChromeOSReleaseVersion
// instances, returning true if the first version comes before the second.
//
// Can be used to sort a slice of ChromeOSReleaseVersion instances.
func IsChromeOSReleaseVersionLessThan(a ChromeOSReleaseVersion, b ChromeOSReleaseVersion) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] == b[i] {
			continue
		}
		return a[i] < b[i]
	}
	return len(a) < len(b)
}
