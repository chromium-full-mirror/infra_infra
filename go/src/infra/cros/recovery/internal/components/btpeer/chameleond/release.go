// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cache"
	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/components/linux"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
)

const (
	// btpeerArtifactsGCSObjectBasePath is the base GCS storage Object path for
	// all btpeer-related artifacts.
	btpeerArtifactsGCSObjectBasePath = "gs://chromeos-connectivity-test-artifacts/btpeer"

	// btpeerArtifactsGCSPublicURLBasePath is the base GCS storage public URL path
	// for all btpeer-related artifacts.
	btpeerArtifactsGCSPublicURLBasePath = "https://storage.googleapis.com/chromeos-connectivity-test-artifacts/btpeer"
)

// CacheAccess is a subset of tlw.Access that just has the ability to access the
// cache server.
type CacheAccess interface {
	// GetCacheUrl provides URL to download requested path to file.
	// URL will use to download image to USB-drive and provisioning.
	GetCacheUrl(ctx context.Context, dutName, filePath string) (string, error)
}

// DownloadChameleondBundle downloads the bundle archive for the bundleConfig to
// the btpeer from GCS via the cache server. Returns the path of the bundle on
// the btpeer.
func DownloadChameleondBundle(ctx context.Context, sshRunner ssh.Runner, cacheAccess CacheAccess, dutName string, bundleConfig *labapi.BluetoothPeerChameleondConfig_ChameleondBundle) (string, error) {
	bundleArchivePath := bundleConfig.GetArchivePath()
	if !strings.HasPrefix(bundleArchivePath, btpeerArtifactsGCSObjectBasePath) {
		return "", errors.Reason("invalid bundle archive path %q", bundleArchivePath).Err()
	}
	bundleFilename := filepath.Base(bundleArchivePath)
	dstPath := filepath.Join(installDir, bundleFilename)
	downloadURL, err := cacheAccess.GetCacheUrl(ctx, dutName, bundleArchivePath)
	if err != nil {
		return "", errors.Annotate(err, "failed to get download URL from cache server for file path %q", bundleArchivePath).Err()
	}
	if _, err := cache.CurlFile(ctx, sshRunner.Run, downloadURL, dstPath, 1*time.Minute); err != nil {
		return "", errors.Annotate(err, "failed to download bundle archive %q to btpeer at %q", bundleArchivePath, dstPath).Err()
	}
	return dstPath, nil
}

// FetchBtpeerChameleondReleaseConfig downloads the production
// BluetoothPeerChameleondConfig JSON file from GCS via its public URL through
// the host and returns its unmarshalled contents.
//
// Note: We use the public URL here rather than the cache to ensure we always
// use the latest version of the config file from GCS.
func FetchBtpeerChameleondReleaseConfig(ctx context.Context, sshRunner ssh.Runner, fetchTimeout time.Duration) (*labapi.BluetoothPeerChameleondConfig, error) {
	btpeerChameleondConfigProdGCSPublicURL, err := url.JoinPath(btpeerArtifactsGCSPublicURLBasePath, "btpeer_chameleond_config_prod.json")
	if err != nil {
		return nil, errors.Annotate(err, "fetch btpeer chameleond release config: failed to build download URL").Err()
	}

	var btpeerChameleondConfigJSON string
	err = retry.WithTimeout(ctx, 1*time.Second, fetchTimeout, func() error {
		var err error
		btpeerChameleondConfigJSON, _, err = linux.CurlURL(ctx, sshRunner.Run, 5*time.Second, btpeerChameleondConfigProdGCSPublicURL, nil)
		return err
	}, "fetch config")
	if err != nil {
		return nil, errors.Annotate(err, "failed to curl %q on the host", btpeerChameleondConfigProdGCSPublicURL).Err()
	}

	config := &labapi.BluetoothPeerChameleondConfig{}
	if err := protojson.Unmarshal([]byte(btpeerChameleondConfigJSON), config); err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal BluetoothPeerChameleondConfig from %q", btpeerChameleondConfigProdGCSPublicURL).Err()
	}
	return config, nil
}

// MarshalBtpeerChameleondReleaseConfig marshals the config into JSON using
// the same settings as btpeer_manager, which is what is used to create the
// config JSON that this would be parsed from, so that the look is consistent.
func MarshalBtpeerChameleondReleaseConfig(config *labapi.BluetoothPeerChameleondConfig) (string, error) {
	marshaller := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}
	configJSON, err := marshaller.Marshal(config)
	if err != nil {
		return "", errors.Annotate(err, "marshal btpeer chameleond release config").Err()
	}
	return string(configJSON), nil
}

// SelectChameleondBundleByChameleondCommit returns the bundle config from the
// chameleond config that has a matching chameleond commit. Returns a non-nil
// error if no matching bundle config is found.
func SelectChameleondBundleByChameleondCommit(config *labapi.BluetoothPeerChameleondConfig, chameleondCommit string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	for _, bundleConfig := range config.GetBundles() {
		if strings.EqualFold(bundleConfig.GetChameleondCommit(), chameleondCommit) {
			return bundleConfig, nil
		}
	}
	return nil, errors.Reason("select chameleond bundle by chameleond commit: found no bundle with ChameleondCommit %q configured", chameleondCommit).Err()
}

// SelectChameleondBundleByNextCommit returns the bundle config that is
// specified as the next bundle by commit in the chameleond config. Returns a
// non-nil error if the chameleond config has no next bundle configured or if
// the chameleond commit it specifies does not match any configured bundles.
func SelectChameleondBundleByNextCommit(config *labapi.BluetoothPeerChameleondConfig) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	if config.GetNextChameleondCommit() == "" {
		return nil, errors.Reason("select chameleon bundle by next commit: next commit is empty").Err()
	}
	bundleConfig, err := SelectChameleondBundleByChameleondCommit(config, config.GetNextChameleondCommit())
	if err != nil {
		return nil, errors.Annotate(err, "select chameleon bundle by next commit").Err()
	}
	return bundleConfig, nil
}

// SelectChameleondBundleByCrosReleaseVersion returns the bundle config that has
// the latest MinDutReleaseVersion that is less than or equal to the provided
// dutCrosReleaseVersion among all bundles configured in the chameleond config,
// excluding the bundle configured as the next bundle.
func SelectChameleondBundleByCrosReleaseVersion(config *labapi.BluetoothPeerChameleondConfig, dutCrosReleaseVersion string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	if len(config.GetBundles()) == 0 {
		return nil, errors.Reason("select chameleond bundle by release version: no bundles are configured").Err()
	}
	dutVersion, err := cros.ParseChromeOSReleaseVersion(dutCrosReleaseVersion)
	if err != nil {
		return nil, errors.Annotate(err, "select chameleond bundle by release version").Err()
	}

	// Collect all matching versions.
	var allMatchingBundleVersions []cros.ChromeOSReleaseVersion
	bundleVersionToConfig := make(map[string]*labapi.BluetoothPeerChameleondConfig_ChameleondBundle)
	for i, bundleConfig := range config.GetBundles() {
		if strings.EqualFold(bundleConfig.GetChameleondCommit(), config.GetNextChameleondCommit()) {
			// Do not include next bundle when matching by version.
			continue
		}
		bundleMinVersion, err := cros.ParseChromeOSReleaseVersion(bundleConfig.GetMinDutReleaseVersion())
		if err != nil {
			return nil, errors.Annotate(err, "select chameleond bundle by release version: parse for config.Bundles[%d].MinDutReleaseVersion", i).Err()
		}
		if !cros.IsChromeOSReleaseVersionLessThan(dutVersion, bundleMinVersion) {
			allMatchingBundleVersions = append(allMatchingBundleVersions, bundleMinVersion)
			bundleVersionToConfig[bundleMinVersion.String()] = bundleConfig
		}
	}
	if len(allMatchingBundleVersions) == 0 {
		return nil, errors.Reason("select chameleond bundle by release version: none of the %d bundles configured have a MinDutReleaseVersion greater than or equal to %q", len(config.GetBundles()), dutVersion.String()).Err()
	}

	// Sort them and use the highest matching min version.
	sort.SliceStable(allMatchingBundleVersions, func(i, j int) bool {
		return cros.IsChromeOSReleaseVersionLessThan(allMatchingBundleVersions[i], allMatchingBundleVersions[j])
	})
	highestMatchingVersion := allMatchingBundleVersions[len(allMatchingBundleVersions)-1]
	if len(allMatchingBundleVersions) > 1 {
		secondHighestMatchingVersion := allMatchingBundleVersions[len(allMatchingBundleVersions)-2]
		if !cros.IsChromeOSReleaseVersionLessThan(secondHighestMatchingVersion, highestMatchingVersion) {
			// Versions are the same, and thus we cannot pick between the two bundles
			// they belong to (this is a config error we'd need to fix manually).
			return nil, errors.Reason("select chameleond bundle by release version: config error: unable to choose bundle for CHROMEOS_RELEASE_VERSION %q, as multiple matching bundles were found with the same MinDutReleaseVersion %q", dutVersion.String(), highestMatchingVersion.String()).Err()
		}
	}
	return bundleVersionToConfig[highestMatchingVersion.String()], nil
}

// SelectChameleondBundleForDut returns the bundle config that the btpeer should
// be using for chameleond based on the chameleond config, the dut's hostname,
// and the dut's ChromeOS release version.
//
// If there is a next bundle configured, the dut's hostname is in the
// NextDutHosts, and the dut's ChromeOS release version is in the
// NextDutReleaseVersions, the next chameleond bundle is selected via
// SelectChameleondBundleByNextCommit. Otherwise, the bundle is selected
// via SelectChameleondBundleByCrosReleaseVersion.
func SelectChameleondBundleForDut(ctx context.Context, config *labapi.BluetoothPeerChameleondConfig, dutHostname, dutCrosReleaseVersion string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	dutVersion, err := cros.ParseChromeOSReleaseVersion(dutCrosReleaseVersion)
	if err != nil {
		return nil, errors.Annotate(err, "select chameleond bundle for dut").Err()
	}

	// Determine if next bundle should be used.
	nextBundleConfigured := config.GetNextChameleondCommit() != ""
	dutInNextHosts := false
	dutVersionInNextVersions := false
	for _, hostname := range config.GetNextDutHosts() {
		if hostname == dutHostname {
			dutInNextHosts = true
			break
		}
	}
	for _, version := range config.GetNextDutReleaseVersions() {
		if version == dutCrosReleaseVersion {
			dutVersionInNextVersions = true
			break
		}
	}
	shouldUseNextBundle := nextBundleConfigured && dutInNextHosts && dutVersionInNextVersions
	log.Debugf(
		ctx,
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
		log.Debugf(ctx, "Selecting next bundle for DUT %q with version %q", dutHostname, dutCrosReleaseVersion)
		nextBundle, err := SelectChameleondBundleByNextCommit(config)
		if err != nil {
			return nil, errors.Annotate(err, "select chameleond bundle for dut").Err()
		}
		nextBundleMinVersion, err := cros.ParseChromeOSReleaseVersion(nextBundle.GetMinDutReleaseVersion())
		if err != nil {
			return nil, errors.Annotate(err, "select chameleond bundle for dut: failed to parse MinDutReleaseVersion for next bundle with ChameleondCommit %q", nextBundle.GetChameleondCommit()).Err()
		}
		if cros.IsChromeOSReleaseVersionLessThan(dutVersion, nextBundleMinVersion) {
			log.Warningf(ctx, "The DUT CHROMEOS_RELEASE_VERSION provided, %q, is less than the selected next bundle MinDutReleaseVersion %q", dutVersion.String(), nextBundle.String())
		}
		return nextBundle, nil
	}

	// Not using next bundle, so select by version.
	log.Debugf(ctx, "Selecting bundle by version for DUT %q with version %q", dutHostname, dutCrosReleaseVersion)
	return SelectChameleondBundleByCrosReleaseVersion(config, dutCrosReleaseVersion)
}
