// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
)

func SelectChameleondBundleByChameleondCommit(config *labapi.BluetoothPeerChameleondConfig, chameleondCommit string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	for _, bundleConfig := range config.Bundles {
		if strings.EqualFold(bundleConfig.ChameleondCommit, chameleondCommit) {
			return bundleConfig, nil
		}
	}
	return nil, fmt.Errorf("found no bundle with ChameleondCommit %q configured", chameleondCommit)
}

func SelectChameleondBundleByNextCommit(config *labapi.BluetoothPeerChameleondConfig) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	if config.NextChameleondCommit == "" {
		return nil, errors.New("no next bundle configured by chameleon commit")
	}
	bundleConfig, err := SelectChameleondBundleByChameleondCommit(config, config.NextChameleondCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to select bundle set as next by commit: %w", err)
	}
	return bundleConfig, nil
}

func SelectChameleondBundleByCrosReleaseVersion(config *labapi.BluetoothPeerChameleondConfig, dutCrosReleaseVersion string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	if len(config.Bundles) == 0 {
		return nil, fmt.Errorf("no bundles are configured")
	}
	dutVersion, err := ParseChromeOSReleaseVersion(dutCrosReleaseVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid CHROMEOS_RELEASE_VERSION %q: %w", dutCrosReleaseVersion, err)
	}

	// Collect all matching versions.
	var allMatchingBundleVersions []ChromeOSReleaseVersion
	bundleVersionToConfig := make(map[string]*labapi.BluetoothPeerChameleondConfig_ChameleondBundle)
	for i, bundleConfig := range config.Bundles {
		if strings.EqualFold(bundleConfig.ChameleondCommit, config.NextChameleondCommit) {
			// Do not include next bundle when matching by version.
			continue
		}
		bundleMinVersion, err := ParseChromeOSReleaseVersion(bundleConfig.MinDutReleaseVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config.Bundles[%d].MinDutReleaseVersion: %w", i, err)
		}
		if !IsChromeOSReleaseVersionLessThan(dutVersion, bundleMinVersion) {
			allMatchingBundleVersions = append(allMatchingBundleVersions, bundleMinVersion)
			bundleVersionToConfig[bundleMinVersion.String()] = bundleConfig
		}
	}
	if len(allMatchingBundleVersions) == 0 {
		return nil, fmt.Errorf("none of the %d bundles configured have a MinDutReleaseVersion greater than or equal to %q", len(config.Bundles), dutVersion.String())
	}

	// Sort them and use the highest matching min version.
	sort.SliceStable(allMatchingBundleVersions, func(i, j int) bool {
		return IsChromeOSReleaseVersionLessThan(allMatchingBundleVersions[i], allMatchingBundleVersions[j])
	})
	highestMatchingVersion := allMatchingBundleVersions[len(allMatchingBundleVersions)-1]
	if len(allMatchingBundleVersions) > 1 {
		secondHighestMatchingVersion := allMatchingBundleVersions[len(allMatchingBundleVersions)-2]
		if !IsChromeOSReleaseVersionLessThan(secondHighestMatchingVersion, highestMatchingVersion) {
			// Versions are the same, and thus we cannot pick between the two bundles
			// they belong to (this is a config error we'd need to fix manually).
			return nil, fmt.Errorf("config error: unable to choose bundle for CHROMEOS_RELEASE_VERSION %q, as multiple matching bundles were found with the same MinDutReleaseVersion %q", dutVersion.String(), highestMatchingVersion.String())
		}
	}
	return bundleVersionToConfig[highestMatchingVersion.String()], nil
}

func SelectChameleondBundleForDut(config *labapi.BluetoothPeerChameleondConfig, dutHostname, dutCrosReleaseVersion string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	dutVersion, err := ParseChromeOSReleaseVersion(dutCrosReleaseVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid CHROMEOS_RELEASE_VERSION %q: %w", dutCrosReleaseVersion, err)
	}

	// Determine if next bundle should be used.
	nextBundleConfigured := config.NextChameleondCommit != ""
	dutInNextHosts := false
	dutVersionInNextVersions := false
	for _, hostname := range config.NextDutHosts {
		if hostname == dutHostname {
			dutInNextHosts = true
			break
		}
	}
	for _, version := range config.NextDutReleaseVersions {
		if version == dutCrosReleaseVersion {
			dutVersionInNextVersions = true
			break
		}
	}
	shouldUseNextBundle := nextBundleConfigured && dutInNextHosts && dutVersionInNextVersions
	log.Logger.Printf(
		"Should DUT %q with version %q use the next bundle? nextBundleConfigured=%t, dutInNextHosts=%t, dutVersionInNextVersions=%t => shouldUseNextBundle=%t",
		dutHostname,
		dutCrosReleaseVersion,
		nextBundleConfigured,
		dutInNextHosts,
		dutVersionInNextVersions,
		shouldUseNextBundle,
	)

	if shouldUseNextBundle {
		// Select next bundle.
		log.Logger.Printf("Selecting next bundle for DUT %q with version %q\n", dutHostname, dutCrosReleaseVersion)
		nextBundle, err := SelectChameleondBundleByNextCommit(config)
		if err != nil {
			return nil, err
		}
		nextBundleMinVersion, err := ParseChromeOSReleaseVersion(nextBundle.MinDutReleaseVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MinDutReleaseVersion for next bundle with ChameleondCommit %q: %w", nextBundle.ChameleondCommit, err)
		}
		if IsChromeOSReleaseVersionLessThan(dutVersion, nextBundleMinVersion) {
			log.Logger.Printf("WARNING: The DUT CHROMEOS_RELEASE_VERSION provided, %q, is less than the selected next bundle MinDutReleaseVersion %q\n", dutVersion.String(), nextBundle.String())
		}
		return nextBundle, nil
	}

	// Not using next bundle, so select by version.
	log.Logger.Printf("Selecting bundle by version for DUT %q with version %q\n", dutHostname, dutCrosReleaseVersion)
	return SelectChameleondBundleByCrosReleaseVersion(config, dutCrosReleaseVersion)
}
