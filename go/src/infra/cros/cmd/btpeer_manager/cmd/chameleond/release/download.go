// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package release

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"infra/cros/cmd/btpeer_manager/cmd/common"
	"infra/cros/cmd/btpeer_manager/dirs"
	"infra/cros/cmd/btpeer_manager/fileutils"
	"infra/cros/cmd/btpeer_manager/log"
	release "infra/cros/cmd/btpeer_manager/release/chameleond"
)

func downloadCmd(dirContext *dirs.DirContext) *cobra.Command {
	var selectNextBundle bool
	var selectByDutHostname string
	var selectByCrosReleaseVersion string
	var selectByChameleondCommit string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads a released chameleond bundle",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Retrieve the config.
			releaseManager := release.SharedManagerInstance()
			config := releaseManager.Config()
			configJSON, err := fileutils.MarshalPrettyJSON(config)
			if err != nil {
				return fmt.Errorf("failed to marshal config to JSON: %w", err)
			}
			log.Logger.Printf("BluetoothPeerChameleondConfig:\n%s", configJSON)

			// Select the bundle.
			requireUserConfirmation := false
			var bundleConfig *labapi.BluetoothPeerChameleondConfig_ChameleondBundle
			if selectByChameleondCommit != "" {
				bundleConfig, err = release.SelectChameleondBundleByChameleondCommit(config, selectByChameleondCommit)
			} else if selectByDutHostname != "" {
				requireUserConfirmation = true
				if selectByCrosReleaseVersion == "" {
					return errors.New("--cros_version must also be provided when selecting with --dut")
				}
				bundleConfig, err = release.SelectChameleondBundleForDut(config, selectByDutHostname, selectByCrosReleaseVersion)
			} else if selectByCrosReleaseVersion != "" {
				requireUserConfirmation = true
				bundleConfig, err = release.SelectChameleondBundleByCrosReleaseVersion(config, selectByCrosReleaseVersion)
			} else if selectNextBundle {
				bundleConfig, err = release.SelectChameleondBundleByNextCommit(config)
			} else {
				return errors.New("at least one selection criterion is required (--commit, --next, --cros_version, or --cros_version and --dut)")
			}
			if err != nil {
				return fmt.Errorf("failed to select bundle: %w", err)
			}
			bundleConfigJSON, err := fileutils.MarshalPrettyJSON(bundleConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal bundle config to JSON: %w", err)
			}
			log.Logger.Printf("Selected bundle:\n%s", bundleConfigJSON)
			if requireUserConfirmation {
				answer, err := common.PromptYesNo(cmd.Context(), "Download the selected bundle?", true)
				if err != nil {
					return err
				}
				if !answer {
					return errors.New("aborted by user")
				}
			}

			// Download the selected bundle.
			if _, err := releaseManager.DownloadChameleondBundle(cmd.Context(), bundleConfig, dirContext.Working.ChameleondBundlesDirPath); err != nil {
				return fmt.Errorf("failed to download bundle: %w", err)
			}
			return err
		},
	}

	cmd.Flags().StringVar(
		&selectByChameleondCommit,
		"commit",
		"",
		"Download the bundle for this chameleond commit (case-insensitive)",
	)
	cmd.Flags().StringVar(
		&selectByDutHostname,
		"dut",
		"",
		"Download the next bundle if the dut is in the verification pool, or the select it by cros_version otherwise (--cros_version required)",
	)
	cmd.Flags().StringVar(
		&selectByCrosReleaseVersion,
		"cros_version",
		"",
		"Download the bundle for this ChromeOS release version (compatible with --dut)",
	)
	cmd.Flags().BoolVar(
		&selectNextBundle,
		"next",
		false,
		"Download the next bundle",
	)
	return cmd
}
