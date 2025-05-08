// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package release

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/fileutils"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/chameleond"
)

func UploadBundleWithCrosVersion(ctx context.Context, srcBundlePath, minCrosVersion string, uploadConfig UploadConfig) error {
	srcBundleFileName := filepath.Base(srcBundlePath)
	bundleCommitMatcher := regexp.MustCompile(`^chameleond-[^\-]+-([^.]+)\.tar\.gz$`)
	bundleMatches := bundleCommitMatcher.FindAllStringSubmatch(srcBundleFileName, -1)
	if len(bundleMatches) != 1 {
		return fmt.Errorf("invalid chameleond bundle filename %q: must match %s", srcBundleFileName, bundleCommitMatcher.String())
	}
	chameleondCommit := bundleMatches[0][1]
	log.Logger.Printf("Bundle ChameleondCommit: %q\n", chameleondCommit)

	// Check existing bundles for any incompatibilities, then remove any as allowed.
	releaseManager := release.SharedManagerInstance()
	config := releaseManager.Config()
	for _, existingBundleConfig := range config.Bundles {
		if existingBundleConfig.MinDutReleaseVersion == minCrosVersion {
			if !uploadConfig.OverwriteCrosVersion {
				return fmt.Errorf("existing bundle found (%s) already configured with the same MinDutReleaseVersion (%q); Add the --overwrite_cros_version flag to remove this existing bundle config or specify a different version", existingBundleConfig.ChameleondCommit, minCrosVersion)
			}
			if config.NextChameleondCommit != "" &&
				strings.EqualFold(existingBundleConfig.ChameleondCommit, config.NextChameleondCommit) &&
				!uploadConfig.SetAsNext {
				return fmt.Errorf("existing bundle found (%s) already configured with the same MinDutReleaseVersion (%q) and it is the next bundle; Add the --next flag to remove this existing bundle config and set this new bundle as next", existingBundleConfig.ChameleondCommit, minCrosVersion)
			}
		}
		if !uploadConfig.OverwriteExistingBundle && existingBundleConfig.ChameleondCommit == chameleondCommit {
			bundleConfigJSON, err := fileutils.MarshalPrettyJSON(existingBundleConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal prexisting bundle config to JSON: %w", err)
			}
			log.Logger.Printf("Existing bundle with matching ChameleondCommit found:\n%s", bundleConfigJSON)
			return fmt.Errorf("bundle found to exist in config already (include --overwrite_existing_bundle to overwrite it)")
		}
	}
	var newBundleConfigs []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle
	for _, existingBundleConfig := range config.Bundles {
		if !strings.EqualFold(existingBundleConfig.ChameleondCommit, chameleondCommit) && existingBundleConfig.MinDutReleaseVersion != minCrosVersion {
			newBundleConfigs = append(newBundleConfigs, existingBundleConfig)
		}
	}
	config.Bundles = newBundleConfigs

	// Upload the bundle and log new bundle config.
	bundleConfig, err := releaseManager.UploadChameleondBundle(
		ctx,
		srcBundlePath,
		chameleondCommit,
		minCrosVersion,
	)
	if err != nil {
		return fmt.Errorf("failed to upload bundle at %q: %w", srcBundlePath, err)
	}
	bundleConfigJSON, err := fileutils.MarshalPrettyJSON(bundleConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal bundle config to JSON: %w", err)
	}
	log.Logger.Printf("Successfully uploaded new chameleond bundle:\n%s", bundleConfigJSON)

	// Update the config with new bundle.
	config.Bundles = append(config.Bundles, bundleConfig)
	if uploadConfig.SetAsNext {
		config.NextChameleondCommit = bundleConfig.ChameleondCommit
	}
	if _, err := releaseManager.UpdateConfig(ctx, config, false); err != nil {
		return fmt.Errorf("failed to save new bundle updates to config: %w", err)
	}

	log.Logger.Println("See 'btpeer_manager chameleond release config --help' for ways to update the config further if needed")

	return nil
}

type UploadConfig struct {
	OverwriteExistingBundle bool
	SetAsNext               bool
	OverwriteCrosVersion    bool
}

func uploadCmd() *cobra.Command {
	uploadConfig := UploadConfig{}

	cmd := &cobra.Command{
		Use:          "upload <min_cros_version> <path_to_bundle_archive>",
		Short:        "Uploads a local bundle to GCS and adds it to the release config",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the min dut version for the new bundle.
			minCrosVersion := args[0]
			parsedVersion, err := release.ParseChromeOSReleaseVersion(minCrosVersion)
			if err != nil {
				return fmt.Errorf("invalid min_cros_version: %w", err)
			}
			minCrosVersion = parsedVersion.String()

			// Parse chameleon commit from bundle filename.
			srcBundlePath := args[1]
			return UploadBundleWithCrosVersion(cmd.Context(), srcBundlePath, minCrosVersion, uploadConfig)
		},
	}

	cmd.Flags().BoolVar(
		&uploadConfig.OverwriteExistingBundle,
		"overwrite_existing_bundle",
		false,
		"Allows overwriting of an bundle that already exists with the same ChameleondCommit",
	)
	cmd.Flags().BoolVar(
		&uploadConfig.SetAsNext,
		"next",
		false,
		"Sets the NextChameleondCommit to this new bundle's ChameleondCommit",
	)
	cmd.Flags().BoolVar(
		&uploadConfig.OverwriteCrosVersion,
		"overwrite_cros_version",
		false,
		"Allows for replacing other bundles in the bundle config with matching min_cros_version values",
	)

	return cmd
}
