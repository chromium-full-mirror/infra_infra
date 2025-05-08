// Copyright 2025 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package preverification

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/chameleond"
)

func releaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release next bundle to all btpeers",
		Long:  "Remove NextChameleondCommit and nextDutReleaseVersions and start use of new bundle in prod",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Retrieve the config.
			releaseManager := release.SharedManagerInstance()
			config := releaseManager.Config()

			if len(config.NextDutReleaseVersions) == 0 {
				return fmt.Errorf("NextDutReleaseVersions is empty, failed to release")
			}

			versionForRelease := config.NextDutReleaseVersions[0]

			log.Logger.Printf("Update nextDutReleaseVersions to []")
			config.NextDutReleaseVersions = []string{}

			log.Logger.Printf("Update bundle with nextChameleondCommit to have real version")
			bundle, err := release.SelectChameleondBundleByNextCommit(config)
			if err != nil {
				return err
			}
			bundle.MinDutReleaseVersion = versionForRelease

			log.Logger.Printf("Removing commit from NextChameleondCommit field")
			config.NextChameleondCommit = ""

			releaseManager.UpdateConfig(cmd.Context(), config, false)

			return nil
		},
	}
	return cmd
}
