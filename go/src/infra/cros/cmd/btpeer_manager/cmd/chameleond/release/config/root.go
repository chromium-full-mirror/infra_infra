// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"github.com/spf13/cobra"

	"infra/cros/cmd/btpeer_manager/dirs"
)

const defaultLocalConfigFilename = "btpeer_chameleond_config.json"

func RootCmd(dirContext *dirs.DirContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Commands for reading and updating the chameleond release config",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		getCmd(dirContext),
		setCmd(dirContext),
	)
	return cmd
}
