// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package release

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/fileutils"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/image"
)

func uploadCmd() *cobra.Command {
	var setAsCurrent bool
	var setAsNext bool
	cmd := &cobra.Command{
		Use:          "upload <path_to_image_file_in_pi_gen_deploy_dir>",
		Short:        "Uploads a local image to GCS and adds it to the image release config",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			imageFilePath := args[0]
			releaseManager := release.SharedManagerInstance()
			// Upload image.
			imageConfig, err := releaseManager.UploadImage(cmd.Context(), imageFilePath)
			if err != nil {
				return fmt.Errorf("failed to upload image at %q: %w", imageFilePath, err)
			}
			imageConfigJSON, err := fileutils.MarshalPrettyJSON(imageConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal image config to JSON: %w", err)
			}
			log.Logger.Printf("Successfully uploaded new image:\n%s\n", string(imageConfigJSON))

			// Update the config with new image.
			config := releaseManager.Config()
			imageConfigReplaced := false
			for i, existingImageConfig := range config.GetImages() {
				if existingImageConfig.GetUuid() == imageConfig.Uuid {
					config.GetImages()[i] = imageConfig
					imageConfigReplaced = true
					break
				}
			}
			if !imageConfigReplaced {
				config.Images = append(config.GetImages(), imageConfig)
			}
			if setAsCurrent {
				config.CurrentImageUuid = imageConfig.Uuid
			}
			if setAsNext {
				config.NextImageUuid = imageConfig.Uuid
			}
			if _, err := releaseManager.UpdateConfig(cmd.Context(), config, false); err != nil {
				return fmt.Errorf("failed to save new image updates to config: %w", err)
			}

			log.Logger.Println("See 'btpeer_manager image release config --help' for ways to update the config further if needed")

			return nil
		},
	}
	cmd.Flags().BoolVar(
		&setAsCurrent,
		"current",
		false,
		"Sets the CurrentImageUuid to this new image's UUID",
	)
	cmd.Flags().BoolVar(
		&setAsNext,
		"next",
		false,
		"Sets the NextImageUuid to this new image's UUID",
	)
	return cmd
}
