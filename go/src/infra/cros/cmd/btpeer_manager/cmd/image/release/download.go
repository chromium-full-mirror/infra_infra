// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package release

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/spf13/cobra"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/dirs"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/fileutils"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/image"
)

func downloadCmd(dirContext *dirs.DirContext) *cobra.Command {
	var selectByDutHostname string
	var selectByImageUUID string
	var selectNext bool
	var selectCurrent bool

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads a released image",
		Long: `Downloads a released image

Select the image by passing one of the selection flags. Selection flags cannot
be combined.
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Retrieve the config.
			releaseManager := release.SharedManagerInstance()
			config := releaseManager.Config()
			configJSON, err := fileutils.MarshalPrettyJSON(config)
			if err != nil {
				return fmt.Errorf("failed to marshal config to JSON: %w", err)
			}
			log.Logger.Printf("RaspiosCrosBtpeerImageConfig:\n%s", configJSON)

			// Select the image.
			var imageConfig *labapi.RaspiosCrosBtpeerImageConfig_OSImage
			if selectByImageUUID != "" {
				imageConfig, err = release.SelectBtpeerImageByUUID(config, selectByImageUUID)
			} else if selectByDutHostname != "" {
				imageConfig, err = release.SelectBtpeerImageForDut(config, selectByDutHostname)
			} else if selectCurrent {
				imageConfig, err = release.SelectCurrentBtpeerImage(config)
			} else if selectNext {
				imageConfig, err = release.SelectNextBtpeerImage(config)
			} else {
				return errors.New("missing required selection criterion flag (--uuid, --dut, --current, or --next)")
			}
			if err != nil {
				return fmt.Errorf("failed to select image: %w", err)
			}
			imageConfigJSON, err := fileutils.MarshalPrettyJSON(imageConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal image config to JSON: %w", err)
			}
			log.Logger.Printf("Selected image:\n%s\n", string(imageConfigJSON))

			// Download the selected image.
			downloadDstDir := path.Join(dirContext.Working.ImagesDirPath, imageConfig.Uuid)
			if err := os.MkdirAll(downloadDstDir, fs.FileMode(0777)); err != nil {
				return fmt.Errorf("failed to initialize image download directory %q: %w", downloadDstDir, err)
			}
			if _, err := releaseManager.DownloadImage(cmd.Context(), imageConfig, downloadDstDir); err != nil {
				return fmt.Errorf("failed to download image: %w", err)
			}
			return err
		},
	}

	cmd.Flags().StringVar(
		&selectByImageUUID,
		"uuid",
		"",
		"Download the image with a matching UUID known by the image config",
	)
	cmd.Flags().StringVar(
		&selectByDutHostname,
		"dut",
		"",
		"Download the next image if the dut is in the verification pool, otherwise download the current image",
	)
	cmd.Flags().BoolVar(
		&selectCurrent,
		"current",
		false,
		"Download the current image",
	)
	cmd.Flags().BoolVar(
		&selectNext,
		"next",
		false,
		"Download the next image",
	)
	return cmd
}
