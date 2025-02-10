// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type ChromeOSReleaseVersion []int

func (v ChromeOSReleaseVersion) String() string {
	segments := make([]string, len(v))
	for i, segment := range v {
		segments[i] = strconv.Itoa(segment)
	}
	return strings.Join(segments, ".")
}

// ParseChromeOSReleaseVersion parses the release version string from the
// lsb-release into its integral parts as a ChromeOSReleaseVersion instance.
func ParseChromeOSReleaseVersion(version string) (ChromeOSReleaseVersion, error) {
	if version == "" {
		return nil, errors.New("cannot parse version from empty string")
	}
	var result ChromeOSReleaseVersion
	for _, segmentStr := range strings.Split(version, ".") {
		segmentInt, err := strconv.Atoi(segmentStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse chromeos release version %q: %w", version, err)
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
