// Copyright 2025 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package preverification

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/cmd/chameleond/make"
	release_cmd "go.chromium.org/infra/cros/cmd/btpeer_manager/cmd/chameleond/release"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/dirs"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/chameleond"
)

const NUMBER_OF_VERSIONS_TO_ADD = 5
const MAX_CROS_VERSION = "999999999"

func updateNextDutReleaseVersionsInConfig(config *labapi.BluetoothPeerChameleondConfig, version release.ChromeOSReleaseVersion) error {
	for range NUMBER_OF_VERSIONS_TO_ADD {
		config.NextDutReleaseVersions = append(config.NextDutReleaseVersions, version.String())
		version[0] += 1
	}
	config.NextDutReleaseVersions = append(config.NextDutReleaseVersions, MAX_CROS_VERSION)
	return nil
}

func execute(ctx context.Context, dirContext *dirs.DirContext, minVersion string) error {
	bundlePath, err := make.RunChameleondMakeCommand(ctx, dirContext)
	if err != nil {
		return err
	}

	uploadConfig := release_cmd.UploadConfig{
		OverwriteExistingBundle: true,
		OverwriteCrosVersion:    true,
		SetAsNext:               true,
	}

	startedVersion, err := release.ParseChromeOSReleaseVersion(minVersion)
	if err != nil {
		return err
	}

	// Upload MAX_CROS_VERSION instead of real one to prevent unexpected usage during pre-verification
	err = release_cmd.UploadBundleWithCrosVersion(ctx, bundlePath, MAX_CROS_VERSION, uploadConfig)
	if err != nil {
		return err
	}

	releaseManager := release.SharedManagerInstance()
	config := releaseManager.Config()
	if err = updateNextDutReleaseVersionsInConfig(config, startedVersion); err != nil {
		return err
	}

	_, err = releaseManager.UpdateConfig(ctx, config, false)
	if err != nil {
		return err
	}
	return nil
}

func startCmd(dirContext *dirs.DirContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <min_cros_version>",
		Short: "Start pre-verification process",
		Long:  "Start pre-verification process by triggering a make, uploading the bundle, and updating the config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			minCrosVersion := args[0]
			parsedVersion, err := release.ParseChromeOSReleaseVersion(minCrosVersion)
			if err != nil {
				return fmt.Errorf("invalid min_cros_version: %w", err)
			}
			minCrosVersion = parsedVersion.String()

			return execute(cmd.Context(), dirContext, minCrosVersion)
		},
	}
	return cmd
}
