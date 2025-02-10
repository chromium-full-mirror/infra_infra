// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"infra/cros/cmd/btpeer_manager/dirs"
	"infra/cros/cmd/btpeer_manager/fileutils"
	"infra/cros/cmd/btpeer_manager/log"
	release "infra/cros/cmd/btpeer_manager/release/chameleond"
)

func getCmd(dirContext *dirs.DirContext) *cobra.Command {
	var printOnly bool
	var dstFilePath string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Downloads the chameleond release config from storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dstFilePath == "" {
				dstFilePath = filepath.Join(dirContext.Working.ConfigDirPath, defaultLocalConfigFilename)
			}
			releaseManager := release.SharedManagerInstance()
			config := releaseManager.Config()
			configJSON, err := fileutils.MarshalPrettyJSON(config)
			if err != nil {
				return fmt.Errorf("failed to marshal config to JSON: %w", err)
			}
			if printOnly {
				log.Logger.Printf("BluetoothPeerChameleondConfig:\n%s", configJSON)
			} else {
				if err := fileutils.WriteBytesToFile(cmd.Context(), configJSON, dstFilePath); err != nil {
					return err
				}
				absDstPath, err := filepath.Abs(dstFilePath)
				if err != nil {
					return fmt.Errorf("failed to get absolute file path dst file %q", dstFilePath)
				}
				log.Logger.Printf("Saved copy of config to \"file://%s\"\n", absDstPath)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(
		&printOnly,
		"print",
		false,
		"Print the config rather than save it to a file",
	)
	cmd.Flags().StringVar(
		&dstFilePath,
		"dst",
		"",
		fmt.Sprintf("Path to save config to (defaults to \"<working_dir>/config/%s\"", defaultLocalConfigFilename),
	)

	return cmd
}
