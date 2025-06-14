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

const MAX_CROS_VERSION = "999999999"

func updateNextDutReleaseVersionsInConfig(config *labapi.BluetoothPeerChameleondConfig) error {
	config.NextDutReleaseVersions = []string{MAX_CROS_VERSION}
	return nil
}

func execute(ctx context.Context, dirContext *dirs.DirContext, minVersion string, force bool) error {
	bundlePath, err := make.RunChameleondMakeCommand(ctx, dirContext)
	if err != nil {
		return err
	}

	uploadConfig := release_cmd.UploadConfig{
		OverwriteExistingBundle: true,
		OverwriteCrosVersion:    true,
		SetAsNext:               true,
	}
	releaseManager := release.SharedManagerInstance()
	config := releaseManager.Config()

	if config.NextChameleondCommit != "" {
		if !force {
			return fmt.Errorf("NextChameleondCommit is already set to %q. Pass --force to overwrite and remove bundle with this commit", config.NextChameleondCommit)
		}
		cancelPreverification(ctx, config)
	}

	err = release_cmd.UploadBundleWithCrosVersion(ctx, bundlePath, minVersion, uploadConfig)
	if err != nil {
		return fmt.Errorf("failed to upload bundle: %w", err)
	}

	if err = updateNextDutReleaseVersionsInConfig(config); err != nil {
		return fmt.Errorf("failed to update NextDutReleaseVersions in config: %w", err)
	}

	_, err = releaseManager.UpdateConfig(ctx, config, false)
	if err != nil {
		return fmt.Errorf("failed to save new bundle updates to config: %w", err)
	}
	return nil
}

func startCmd(dirContext *dirs.DirContext) *cobra.Command {
	var force bool

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

			return execute(cmd.Context(), dirContext, minCrosVersion, force)
		},
	}

	cmd.Flags().BoolVar(
		&force,
		"force",
		false,
		"Force the pre-verification process to overwrite nextChameleondCommit and nextDutReleaseVersions",
	)
	return cmd
}
