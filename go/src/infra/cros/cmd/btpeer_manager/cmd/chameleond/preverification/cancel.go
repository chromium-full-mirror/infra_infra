// Copyright 2025 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package preverification

import (
	"github.com/spf13/cobra"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
	release "go.chromium.org/infra/cros/cmd/btpeer_manager/release/chameleond"
)

func removeFromBundleList(bundles []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, commitToRemove string) []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle {
	for i, bundle := range bundles {
		if bundle.ChameleondCommit == commitToRemove {
			return append(bundles[:i], bundles[i+1:]...)
		}
	}
	return bundles
}

func cancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel pre-verification process and revert all changes in config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Retrieve the config.
			releaseManager := release.SharedManagerInstance()
			config := releaseManager.Config()
			log.Logger.Printf("Removing bundle with commit %s", config.NextChameleondCommit)
			config.Bundles = removeFromBundleList(config.Bundles, config.NextChameleondCommit)
			log.Logger.Printf("Removing commit from NextChameleondCommit field")
			config.NextChameleondCommit = ""
			log.Logger.Printf("Update nextDutReleaseVersions to []")
			config.NextDutReleaseVersions = []string{}

			releaseManager.UpdateConfig(cmd.Context(), config, false)

			return nil
		},
	}
	return cmd
}
