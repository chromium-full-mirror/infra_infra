// Copyright 2025 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package preverification provides commands for managing chameleond pre-verification for btpeers.
package preverification

import (
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/spf13/cobra"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/cmd/common"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/dirs"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/chameleond"
)

func RootCmd(dirContext *dirs.DirContext, initDirContext func() error) *cobra.Command {
	managerConfig := &release.ManagerConfig{
		StorageBucketName: release.ChromeOSConnectivityTestArtifactsStorageBucket,
	}

	cmd := &cobra.Command{
		Use:   "pre-verification",
		Short: "Commands related to pre-verification of chameleond for btpeers",
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := initDirContext(); err != nil {
				return err
			}
			releaseManager, err := release.NewManager(cmd.Context(), managerConfig)
			if err != nil {
				return fmt.Errorf("failed to initialize a new release manager: %w", err)
			}
			if err := releaseManager.FetchConfig(cmd.Context()); err != nil {
				if errors.Is(err, storage.ErrObjectNotExist) {
					configPath, _ := releaseManager.ConfigObject()
					answer, err := common.PromptYesNo(cmd.Context(), fmt.Sprintf("No existing BluetoothPeerChameleondConfig found at %q.\n\nWant to initialize a new config file?", configPath), false)
					if err != nil {
						return err
					}
					if !answer {
						return errors.New("aborted by user")
					}
					if err := releaseManager.InitializeNewBTPeerChameleondConfig(cmd.Context(), false); err != nil {
						return fmt.Errorf("failed to initialize new BluetoothPeerChameleondConfig: %w", err)
					}
				} else {
					return fmt.Errorf("failed to fetch config: %w", err)
				}
			}
			release.SetSharedManagerInstance(releaseManager)
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(
		&managerConfig.GCSCredentialsFilePath,
		"gcloud_cred_file",
		"",
		"The gcloud credential file to use with the GCS API (uses gcloud CLI application-default credentials when unset)",
	)

	cmd.PersistentFlags().BoolVarP(
		&managerConfig.UseProdConfig,
		"prod",
		"p",
		false,
		"Uses the production config when present, or the test config when not present",
	)

	cmd.AddCommand(
		startCmd(dirContext),
		cancelCmd(),
		releaseCmd(),
	)

	return cmd
}
