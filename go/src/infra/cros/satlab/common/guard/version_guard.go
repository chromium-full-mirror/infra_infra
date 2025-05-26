// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package guard

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type SatlabVersion struct {
	Major int
	Minor int
	Patch int
}

func VerifySatlabVersion(ctx context.Context, executor executor.IExecCommander, requiredSatlabVersionString string) error {
	osVersion, err := satlabcommands.GetOsVersion(ctx, executor)
	if err != nil {
		return fmt.Errorf("Failed to get OS version: %w", err)
	}

	if !shouldVerifyVersion(osVersion.Track) {
		return nil
	}

	actualSatlabVersionString, err := satlabcommands.GetSatlabVersion(ctx, executor)
	if err != nil {
		return fmt.Errorf("Failed to get Satlab version: %w", err)
	}

	actualSatlabVersion, err := parseVersion(actualSatlabVersionString)
	if err != nil {
		return fmt.Errorf("Failed to parse actual Satlab version '%s': %w", actualSatlabVersionString, err)
	}
	requiredSatlabVersion, err := parseVersion(requiredSatlabVersionString)
	if err != nil {
		return fmt.Errorf("Failed to parse required Satlab version '%s': %w", requiredSatlabVersionString, err)
	}

	if !IsVersionGreaterOrEqual(requiredSatlabVersion, actualSatlabVersion) {
		return fmt.Errorf("Your Satlab version (%s) is too old. This functionality requires at least version %s. Please update your device to access this feature.", actualSatlabVersionString, requiredSatlabVersionString)
	}

	return nil
}

func VerifyOsMilestone(ctx context.Context, executor executor.IExecCommander, requiredMilestone int) error {
	osVersion, err := satlabcommands.GetOsVersion(ctx, executor)
	if err != nil {
		return fmt.Errorf("Failed to retrieve OS version: %w", err)
	}

	if !shouldVerifyVersion(osVersion.Track) {
		return nil
	}

	actualMilestone, err := parseMilestoneNumber(osVersion.Version)
	if err != nil {
		return fmt.Errorf("Failed to parse OS milestone number from version '%s': %w", osVersion.Version, err)
	}

	if actualMilestone < requiredMilestone {
		return fmt.Errorf("Your Satlab device's OS milestone (%d) is outdated. This functionality requires at least %d. Please update your device to access this feature.", actualMilestone, requiredMilestone)
	}

	return nil
}

func shouldVerifyVersion(s string) bool {
	return strings.Contains(s, "beta-channel") || strings.Contains(s, "stable-channel")
}

// parseVersion parses a version string like "R-X.Y.Z" into its components.
func parseVersion(version string) (SatlabVersion, error) {
	trimmed := strings.TrimPrefix(version, "R-")
	if len(trimmed) == len(version) {
		return SatlabVersion{}, fmt.Errorf("invalid version format: missing 'R-' prefix in %s", version)
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return SatlabVersion{}, fmt.Errorf("invalid version format: expected X.Y.Z, got %s", trimmed)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return SatlabVersion{}, fmt.Errorf("invalid major version: %s", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return SatlabVersion{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return SatlabVersion{}, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return SatlabVersion{Major: major, Minor: minor, Patch: patch}, nil
}

func IsVersionGreaterOrEqual(requiredSatlabVersion SatlabVersion, actualSatlabVersion SatlabVersion) bool {
	if actualSatlabVersion.Major > requiredSatlabVersion.Major {
		return true
	}
	if actualSatlabVersion.Major < requiredSatlabVersion.Major {
		return false
	}

	if actualSatlabVersion.Minor > requiredSatlabVersion.Minor {
		return true
	}
	if actualSatlabVersion.Minor < requiredSatlabVersion.Minor {
		return false
	}

	if actualSatlabVersion.Patch >= requiredSatlabVersion.Patch {
		return true
	}

	return false
}

// ParseMilestoneNumber safely extracts the milestone number from a string
// in the format RXXX-YYY.ZZZ.PPP, where XXX is the milestone.
// It returns the milestone number and an error if parsing fails.
func parseMilestoneNumber(s string) (int, error) {
	re := regexp.MustCompile(`^R(\d+)-\d+\.\d+\.\d+$`)

	matches := re.FindStringSubmatch(s)

	if len(matches) < 2 {
		return 0, fmt.Errorf("string does not match expected format: %s", s)
	}
	milestoneStr := matches[1]

	milestone, err := strconv.Atoi(milestoneStr)
	if err != nil {
		return 0, fmt.Errorf("failed to convert milestone '%s' to integer: %w", milestoneStr, err)
	}

	return milestone, nil
}
