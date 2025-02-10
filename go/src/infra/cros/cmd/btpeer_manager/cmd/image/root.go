// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package image provides commands for managing images on btpeers.
package image

import (
	"github.com/spf13/cobra"

	"infra/cros/cmd/btpeer_manager/cmd/image/release"
	"infra/cros/cmd/btpeer_manager/dirs"
)

func RootCmd(dirContext *dirs.DirContext, initDirContext func() error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Commands related to managing images on btpeers",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		release.RootCmd(dirContext, initDirContext),
	)

	return cmd
}
