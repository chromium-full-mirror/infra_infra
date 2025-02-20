// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"github.com/spf13/cobra"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/cmd/chameleond/release"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/dirs"
)

func RootCmd(dirContext *dirs.DirContext, initDirContext func() error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chameleond",
		Short: "Commands related to managing chameleond on btpeers",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		makeCmd(dirContext),
		release.RootCmd(dirContext, initDirContext),
	)

	return cmd
}
