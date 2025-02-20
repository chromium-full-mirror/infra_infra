// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/urlpath"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// wifiRouterArtifactsGCSObjectBasePath is the base Google Cloud Storage
	// Object path for all wifi-related ChromeOS connectivity test artifact GCS
	// objects.
	wifiRouterArtifactsGCSObjectBasePath = "gs://chromeos-connectivity-test-artifacts/wifi_router"

	// wifiRouterArtifactsGCSObjectBasePath is the base Google Cloud Storage
	// public URL path for all wifi-related ChromeOS connectivity test artifact
	// GCS objects.
	wifiRouterArtifactsGCSPublicURLBasePath = "https://storage.googleapis.com/chromeos-connectivity-test-artifacts/wifi_router"
)

const (
	// defaultPostRebootSSHDelay is the default amount of time to wait after
	// triggering a reboot to start trying to connect to the device over ssh.
	defaultPostRebootSSHDelay = 10 * time.Second

	// defaultPostRebootSSHInterval is the default amount of time to wait
	// after a failed post-reboot ssh attempt before retrying.
	defaultPostRebootSSHInterval = 10 * time.Second

	// defaultPostRebootSSHTimeout is the default overall timeout for post-reboot
	// ssh connection attempts.
	defaultPostRebootSSHTimeout = 3 * time.Minute
)

// invalidDeviceNameCharacterRegex matches one or more invalid device name
// characters and underscores so that they may be replaced with a single
// underscore.
var invalidDeviceNameCharacterRegex = regexp.MustCompile(`([^a-zA-Z0-9\-]|_)+`)

// buildModelName builds a tlw.WifiRouterHost.model name that is a combination
// of the deviceType name and a sanitized deviceName. The deviceName is expected
// to be a unique model identifier of the device for the given deviceType.
func buildModelName(deviceType labapi.WifiRouterDeviceType, deviceName string) string {
	shortDeviceTypeName := strings.TrimPrefix(deviceType.String(), "WIFI_ROUTER_DEVICE_TYPE_")
	deviceName = invalidDeviceNameCharacterRegex.ReplaceAllString(deviceName, "_")
	return fmt.Sprintf("%s[%s]", shortDeviceTypeName, deviceName)
}

// CleanWifiRouterFeatures removes duplicate router features, sorts them by
// name, and replaces invalid router features with
// labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID.
func CleanWifiRouterFeatures(features []labapi.WifiRouterFeature) []labapi.WifiRouterFeature {
	result := removeDuplicateWifiRouterFeatures(features)
	SortWifiRouterFeaturesByName(result)
	replaceInvalidWifiRouterFeatures(result)
	return result
}

// removeDuplicateWifiRouterFeatures returns a new list of features with any
// duplicates removed.
func removeDuplicateWifiRouterFeatures(features []labapi.WifiRouterFeature) []labapi.WifiRouterFeature {
	var result []labapi.WifiRouterFeature
	includedFeatures := make(map[int32]bool)
	for _, feature := range features {
		value := int32(feature.Number())
		if includedFeatures[value] {
			continue
		}
		includedFeatures[value] = true
		result = append(result, feature)
	}
	return result
}

// replaceInvalidWifiRouterFeatures replaces any features that are not declared
// in the labapi.WifiRouterFeature Enum with labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID.
//
// This is to highlight errors in feature lists that are pulled from external
// sources that may or may not have had their values checked against the enum
// so that they may be fixed.
func replaceInvalidWifiRouterFeatures(features []labapi.WifiRouterFeature) {
	for i, feature := range features {
		if _, ok := labapi.WifiRouterFeature_name[int32(feature)]; !ok {
			features[i] = labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID
		}
	}
}

// SortWifiRouterFeaturesByName sorts the list of features by their proto enum
// names. Unknown value names sorted by value at the end of the list.
func SortWifiRouterFeaturesByName(features []labapi.WifiRouterFeature) {
	sort.SliceStable(features, func(i, j int) bool {
		aValue := int32(features[i].Number())
		bValue := int32(features[j].Number())
		aName, aKnown := labapi.WifiRouterFeature_name[aValue]
		bName, bKnown := labapi.WifiRouterFeature_name[bValue]
		if aKnown && bKnown {
			return aName < bName
		}
		if !aKnown && !bKnown {
			return aValue < bValue
		}
		return aKnown && !bKnown
	})
}

// collectCommonWifiRouterFeatures returns a new feature set that only includes
// features present in all featureSets. Features in excludedFeatures will be
// ignored and not present in the result.
func collectCommonWifiRouterFeatures(featureSets [][]labapi.WifiRouterFeature, excludedFeatures []labapi.WifiRouterFeature) []labapi.WifiRouterFeature {
	if len(featureSets) == 0 {
		return nil
	}
	excludedFeatureLookup := make(map[int32]bool)
	for _, feature := range excludedFeatures {
		excludedFeatureLookup[int32(feature)] = true
	}
	totalFeatureCounts := make(map[int32]int32)
	for _, featureSet := range featureSets {
		featuresInSet := make(map[int32]bool)
		for _, feature := range featureSet {
			featureValue := int32(feature)
			if excludedFeatureLookup[featureValue] || featuresInSet[featureValue] {
				continue // Excluded or already counted.
			}
			featuresInSet[featureValue] = true
			totalFeatureCounts[featureValue] = totalFeatureCounts[featureValue] + 1
		}
	}
	totalFeatureSets := len(featureSets)
	var commonFeatures []labapi.WifiRouterFeature
	for featureValue, totalSetsWithFeature := range totalFeatureCounts {
		if int(totalSetsWithFeature) != totalFeatureSets {
			continue // Not in all sets, so not common.
		}
		commonFeatures = append(commonFeatures, labapi.WifiRouterFeature(featureValue))
	}
	return commonFeatures
}

// CollectOverallTestbedWifiRouterFeatures returns a single list of router
// features supported by all routers in the testbed.
//
// If any router does not have features or has an unknown feature, a list with
// just one unknown feature is returned since it is not possible to know what
// features all routers support. It is expected that all routers support at
// least one valid feature.
//
// If any router has an invalid feature, the returned list will have one invalid
// feature included to denote this for maintenance purposes.
//
// If there are no common features across all routers, a list with just one
// invalid feature is returned to denote this for maintenance purposes.
func CollectOverallTestbedWifiRouterFeatures(routers []*tlw.WifiRouterHost) []labapi.WifiRouterFeature {
	if len(routers) == 0 {
		// There are no routers, so there are no features.
		return nil
	}
	// Collect features of each router, taking note of unknown/invalid features
	// present in any set.
	var allRouterFeatureSets [][]labapi.WifiRouterFeature
	anyRouterHasAnInvalidFeature := false
	for _, router := range routers {
		routerFeaturesAreUnknown := false
		if len(router.Features) == 0 {
			// Treat an unset list of features as unknown, as it is expected that all
			// routers have at least one feature. This would mean the router has not
			// been evaluated yet, therefore the features are unknown.
			routerFeaturesAreUnknown = true
		} else {
			for _, feature := range router.Features {
				if feature == labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN {
					routerFeaturesAreUnknown = true
					break
				}
				if feature == labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID {
					anyRouterHasAnInvalidFeature = true
				}
			}
		}
		if routerFeaturesAreUnknown {
			// Since one router has unknown features, we cannot be sure of the
			// overall testbed's router features.
			return []labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
			}
		}
		allRouterFeatureSets = append(allRouterFeatureSets, router.Features)
	}
	// Collect the common router features across all routers, ignoring invalid
	// features as that is handled below.
	commonFeatures := collectCommonWifiRouterFeatures(allRouterFeatureSets, []labapi.WifiRouterFeature{
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
	})
	if len(commonFeatures) == 0 {
		// There are no common, valid features. Return just a single invalid entry
		// to highlight this for testbed maintenance since it means these routers
		// are unusable.
		return []labapi.WifiRouterFeature{
			labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
		}
	}
	if anyRouterHasAnInvalidFeature {
		// Include a single invalid feature entry to note that there is at least one
		// invalid feature among the testbed's routers to highlight this for testbed
		// maintenance. These routers are still usable for their common, valid
		// supported testing features.
		commonFeatures = append(commonFeatures, labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID)
	}
	return commonFeatures
}

// remoteFileContentsMatch checks if the file at remoteFilePath on the remote
// host exists and that its contents match using the regex string matchRegex.
//
// Returns true if the file exists and its contents matches. Returns false
// with a nil error if the file does not exist.
// Returns:
// - true, nil: the file exists and its contents match.
// - false, nil: the file does not exist.
// - false, <error>: failed to check file or the file exists and its contents do not match.
func remoteFileContentsMatch(ctx context.Context, sshRunner ssh.Runner, remoteFilePath, matchRegex string) (bool, error) {
	// Verify that the file exists.
	fileExists, err := ssh.TestFileExists(ctx, sshRunner, remoteFilePath)
	if err != nil {
		return false, errors.Annotate(err, "failed to check for the existence of file %q", remoteFilePath).Err()
	}
	if !fileExists {
		return false, nil
	}

	// Verify that the file contents match.
	matcher, err := regexp.Compile(matchRegex)
	if err != nil {
		return false, errors.Annotate(err, "failed to compile regex string %q", matchRegex).Err()
	}
	fileContents, err := ssh.CatFile(ctx, sshRunner, remoteFilePath)
	if err != nil {
		return false, err
	}
	return matcher.MatchString(fileContents), nil
}

// CacheAccess is a subset of tlw.Access that just has the ability to access the
// cache server.
type CacheAccess interface {
	// GetCacheUrl provides URL to download requested path to file.
	// URL will use to download image to USB-drive and provisioning.
	GetCacheUrl(ctx context.Context, dutName, filePath string) (string, error)
}

// readFileFromURL downloads a file from the provided URL through
// the router host and then returns its contents as a string. No temporary file
// is used on the router host, as its contents are taken from the stdout of
// wget.
//
// If the download fails, it will be reattempted every 1 second up until the
// downloadTimeout is reached.
func readFileFromURL(ctx context.Context, sshRunner ssh.Runner, downloadTimeout time.Duration, downloadURL string) (fileContents string, lastHTTPErrorCode int, err error) {
	if err := retry.WithTimeout(ctx, time.Second, downloadTimeout, func() error {
		var err error
		fileContents, _, lastHTTPErrorCode, err = ssh.WgetURL(ctx, sshRunner, downloadTimeout, downloadURL, "-q", "-O", "-")
		return err
	}, fmt.Sprintf("router host download %q", downloadURL)); err != nil {
		return "", lastHTTPErrorCode, errors.Annotate(err, "failed to download %q from router with wget", downloadURL).Err()
	}
	return fileContents, 0, nil
}

// downloadFileFromCacheServer downloads a file from the cache server to the
// router host.
//
// The cache server will download the file from GCS if it is not already cached.
//
// If the download from the cache server fails, it will be reattempted every
// 1 second up until the downloadTimeout is reached.
func downloadFileFromCacheServer(ctx context.Context, sshRunner ssh.Runner, cacheAccess CacheAccess, dutName string, downloadTimeout time.Duration, srcFilePath, dstFilePath string) error {
	// Prepare file for download from cache server.
	downloadURL, err := cacheAccess.GetCacheUrl(ctx, dutName, srcFilePath)
	if err != nil {
		return errors.Annotate(err, "failed to get download URL from cache server for file path %q", srcFilePath).Err()
	}
	downloadURL, err = urlpath.EnrichWithTrackingIds(ctx, downloadURL)
	if err != nil {
		return errors.Annotate(err, "failed to enrich download URL from cache server with tracking IDs for file path %q", srcFilePath).Err()
	}

	// Download file from cache server to router with wget to dstFilePath.
	// Retry every second until the timeout is reached, as it make take some
	// time for the cache server to prepare the file.
	var lastHTTPErrorCode int
	if err := retry.WithTimeout(ctx, time.Second, downloadTimeout, func() error {
		var err error
		_, _, lastHTTPErrorCode, err = ssh.WgetURL(ctx, sshRunner, downloadTimeout, downloadURL, "-O", dstFilePath)
		return err
	}, fmt.Sprintf("router host download %q from cache server", downloadURL)); err != nil {
		reportCacheFailedMetric(ctx, downloadURL, lastHTTPErrorCode)
		return errors.Annotate(err, "failed to download %q from cache server to router with wget", downloadURL).Err()
	}
	return nil
}

// reportCacheFailedMetric adds metrics related to downloads from the cache
// server failing.
func reportCacheFailedMetric(ctx context.Context, sourcePath string, httpResponseCode int) {
	if httpResponseCode >= 500 {
		// Non-500 errors are recorded by caching service.
		// We are only interested in 500 errors coming from the caching service.
		if execMetric := metrics.GetDefaultAction(ctx); execMetric != nil {
			execMetric.Observations = append(execMetric.Observations,
				metrics.NewInt64Observation("cache_failed_response_code", int64(httpResponseCode)),
				metrics.NewStringObservation("cache_failed_source_path", sourcePath),
			)
		}
	}
}

// fetchWifiRouterConfig downloads the production WifiRouterConfig JSON file
// from GCS via its public URL through the router and returns its unmarshalled
// contents.
//
// Note: We use the public URL here rather than the cache to ensure we always
// use the latest version of the config file from GCS.
func fetchWifiRouterConfig(ctx context.Context, sshRunner ssh.Runner) (*labapi.WifiRouterConfig, error) {
	wifiRouterConfigFileDownloadURL, err := url.JoinPath(wifiRouterArtifactsGCSPublicURLBasePath, "wifi_router_config_prod.json")
	if err != nil {
		return nil, errors.Annotate(err, "fetch wifi router config: failed to build download URL").Err()
	}
	wifiRouterConfigJSON, _, err := readFileFromURL(ctx, sshRunner, 30*time.Second, wifiRouterConfigFileDownloadURL)
	if err != nil {
		return nil, errors.Annotate(err, "failed to read %q on the router", wifiRouterConfigFileDownloadURL).Err()
	}
	config := &labapi.WifiRouterConfig{}
	if err := protojson.Unmarshal([]byte(wifiRouterConfigJSON), config); err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal WifiRouterConfig from %q", wifiRouterConfigFileDownloadURL).Err()
	}
	return config, nil
}

func lsbReleaseFileMatches(ctx context.Context, sshRunner ssh.Runner, matchRegex string) (bool, error) {
	lsbReleaseFilePath := "/etc/lsb-release"
	matches, err := remoteFileContentsMatch(ctx, sshRunner, lsbReleaseFilePath, matchRegex)
	if err != nil {
		return false, errors.Annotate(err, "failed to check if remote file %q contents match %q", lsbReleaseFilePath, matchRegex).Err()
	}
	return matches, nil
}
