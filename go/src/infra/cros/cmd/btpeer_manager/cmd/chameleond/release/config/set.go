// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/dirs"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/fileutils"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/chameleond"
)

func setCmd(dirContext *dirs.DirContext) *cobra.Command {
	var srcFilePath string
	var forceUpdate bool

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Updates the chameleond release config in storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if srcFilePath == "" {
				srcFilePath = filepath.Join(dirContext.Working.ConfigDirPath, defaultLocalConfigFilename)
			}
			log.Logger.Printf("Reading config file %q\n", srcFilePath)
			configJSON, err := fileutils.ReadFile(cmd.Context(), srcFilePath)
			if err != nil {
				return fmt.Errorf("failed to read config file %q: %w", srcFilePath, err)
			}
			config := &labapi.BluetoothPeerChameleondConfig{}
			if err := protojson.Unmarshal(configJSON, config); err != nil {
				return fmt.Errorf("failed to unmarshal config file %q as a BluetoothPeerChameleondConfig: %w", srcFilePath, err)
			}
			configJSON, err = fileutils.MarshalPrettyJSON(config)
			if err != nil {
				return fmt.Errorf("failed to re-marshal config unmarshed from %q: %w", srcFilePath, err)
			}
			log.Logger.Printf("Successfully read BluetoothPeerChameleondConfig from %q:\n%s", srcFilePath, configJSON)

			releaseManager := release.SharedManagerInstance()
			if _, err := releaseManager.UpdateConfig(cmd.Context(), config, forceUpdate); err != nil {
				return fmt.Errorf("failed to update config: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&srcFilePath,
		"src",
		"",
		fmt.Sprintf("Path to read new config from (defaults to \"<working_dir>/config/%s\"", defaultLocalConfigFilename),
	)
	cmd.Flags().BoolVar(
		&forceUpdate,
		"force",
		false,
		"Bypass config validation failures and forcefully update the config",
	)

	return cmd
}
