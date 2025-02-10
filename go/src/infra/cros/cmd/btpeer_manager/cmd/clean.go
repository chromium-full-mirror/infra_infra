// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"infra/cros/cmd/btpeer_manager/dirs"
)

func cleanCmd(dirContext *dirs.DirContext) *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Deletes temporary/build files created by btpeer_manager.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := dirContext.Working.Clean(); err != nil {
				return fmt.Errorf("failed to clean build directories: %w", err)
			}
			return nil
		},
	}
}
