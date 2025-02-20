// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/dirs"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/fileutils"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
)

func makeCmd(dirContext *dirs.DirContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Builds chameleond in your chroot and prepares bundle for btpeer distribution",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Build the chameleond bundle.
			if err := runChameleondMake(cmd.Context(), dirContext, "clean"); err != nil {
				return err
			}
			if err := runChameleondMake(cmd.Context(), dirContext); err != nil {
				return err
			}

			// Identify bundle archive and commit file.
			log.Logger.Printf("Searching for built bundle files in %q\n", dirContext.Src.ChameleonDistDirPath)
			bundleFiles, err := filepath.Glob(filepath.Join(dirContext.Src.ChameleonDistDirPath, "chameleond-*.tar.gz"))
			if err != nil {
				return fmt.Errorf("failed to search for built chameleond bundle archives in %q: %w", dirContext.Src.ChameleonDistDirPath, err)
			}
			if len(bundleFiles) != 1 {
				return fmt.Errorf("expected to find exactly 1 chameleond bundle archive in %q, but found %d [%s]", dirContext.Src.ChameleonDistDirPath, len(bundleFiles), strings.Join(bundleFiles, ","))
			}
			bundleFilePath := bundleFiles[0]
			log.Logger.Printf("Found chameleond bundle archive %q\n", bundleFilePath)
			commitFilePath := filepath.Join(dirContext.Src.ChameleonDistDirPath, "commit")
			if _, err := os.Stat(commitFilePath); err != nil {
				return fmt.Errorf("failed to check for commit file in %q: %w", dirContext.Src.ChameleonDistDirPath, err)
			}
			log.Logger.Printf("Found chameleond commit file %q\n", commitFilePath)

			// Read commit from commit file.
			log.Logger.Printf("Reading chameleond commit from chameleond commit file %q\n", commitFilePath)
			commitFileContents, err := fileutils.ReadFile(cmd.Context(), commitFilePath)
			if err != nil {
				return fmt.Errorf("failed to read chameleond commit file %q: %w", commitFilePath, err)
			}

			chameleondCommit := strings.TrimSpace(string(commitFileContents))
			if chameleondCommit == "" {
				return fmt.Errorf("no commit found in chameleond commit file %q", commitFilePath)
			}
			log.Logger.Printf("Read chameleond commit as %q\n", chameleondCommit)

			// Copy and rename archive with an added commit in the filename.
			log.Logger.Println("Preparing bundle archive for distribution")
			dstBundlePath := filepath.Join(dirContext.Working.ChameleondBundlesDirPath, fmt.Sprintf("%s-%s.tar.gz", strings.TrimSuffix(filepath.Base(bundleFilePath), ".tar.gz"), chameleondCommit))
			if err := fileutils.CopyFile(cmd.Context(), bundleFilePath, dstBundlePath); err != nil {
				return err
			}
			log.Logger.Printf("New btpeer chameleond bundle archive %q ready for distribution at %q\n", filepath.Base(dstBundlePath), dstBundlePath)

			return nil
		},
	}
	return cmd
}

func runChameleondMake(ctx context.Context, dirContext *dirs.DirContext, makeArgs ...string) error {
	fullMakeCmd := "make"
	if len(makeArgs) > 0 {
		fullMakeCmd += " " + strings.Join(makeArgs, " ")
	}
	log.Logger.Printf("Running %q from within chroot in chameleon source dir %q\n", fullMakeCmd, dirContext.Src.ChameleonDirPath)
	args := []string{
		"--working-dir",
		"/mnt/host/source/src/platform/chameleon",
		"make",
	}
	args = append(args, makeArgs...)
	cmd := exec.CommandContext(
		ctx,
		"cros_sdk",
		args...,
	)
	cmd.Dir = dirContext.Src.ChromiumosDirPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %q from within chroot in chameleon source dir %q: %w", fullMakeCmd, dirContext.Src.ChameleonDirPath, err)
	}
	return nil
}
