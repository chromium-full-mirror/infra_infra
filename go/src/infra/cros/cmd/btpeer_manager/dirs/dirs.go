// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dirs defines the working and source directories used by
// btpeer_manager.
package dirs

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"infra/cros/cmd/btpeer_manager/log"
)

const defaultWorkingDirPath = "/tmp/btpeer_manager"

// DirContext is a container for working and src directories.
type DirContext struct {
	Working *WorkingDirectory
	Src     *SrcDirectory
}

func (dc *DirContext) String() string {
	dcJSON, err := json.MarshalIndent(dc, "", "  ")
	if err != nil {
		panic("failed to marshal DirContext to JSON: " + err.Error())
	}
	return string(dcJSON)
}

// WorkingDirectory defines related working directories and provides functions
// for cleaning them.
type WorkingDirectory struct {
	// BaseDirPath is the base working directory containing all other working
	// subdirectories.
	BaseDirPath string

	// ChameleondBundlesDirPath is the path to the directory where
	// built chameleond bundles are stored.
	ChameleondBundlesDirPath string

	// ImagesDirPath is the path to the directory where downloaded images are stored.
	ImagesDirPath string

	// ConfigDirPath is the path to the directory where config files are
	// downloaded to and uploaded from
	ConfigDirPath string
}

// NewWorkingDirectory initializes a new WorkingDirectory, creating any missing
// directories as needed.
func NewWorkingDirectory(workingDirPath string) (*WorkingDirectory, error) {
	if workingDirPath == "" {
		workingDirPath = defaultWorkingDirPath
	}
	workingDirAbsPath, err := filepath.Abs(workingDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of workingDirPath %q: %w", workingDirPath, err)
	}
	baseDirPath := filepath.Join(workingDirAbsPath, "build")
	workingDir := &WorkingDirectory{
		BaseDirPath:              baseDirPath,
		ChameleondBundlesDirPath: filepath.Join(baseDirPath, "bundles"),
		ImagesDirPath:            filepath.Join(baseDirPath, "images"),
		ConfigDirPath:            filepath.Join(baseDirPath, "config"),
	}
	dirPathsToCreate := []string{
		workingDir.BaseDirPath,
		workingDir.ChameleondBundlesDirPath,
		workingDir.ImagesDirPath,
		workingDir.ConfigDirPath,
	}
	for _, dirPath := range dirPathsToCreate {
		if err := os.MkdirAll(dirPath, fs.FileMode(0777)); err != nil {
			return nil, fmt.Errorf("failed to initialize directory %q: %w", dirPath, err)
		}
	}
	return workingDir, nil
}

// Clean removes all working subdirectories.
func (wd *WorkingDirectory) Clean() error {
	log.Logger.Printf("Removing working directory %q\n", wd.BaseDirPath)
	if err := os.RemoveAll(wd.BaseDirPath); err != nil {
		return fmt.Errorf("failed to remove working directory %q: %w", wd.BaseDirPath, err)
	}
	return nil
}

// SrcDirectory defines related source directories.
type SrcDirectory struct {
	// ChromiumosDirPath is the path to the root chromiumos source directory.
	ChromiumosDirPath string

	// BTPeerManagerDirPath is the path the source directory for btpeer_manager.
	BTPeerManagerDirPath string

	// ChameleonDirPath is the path to the chameleon repo source directory.
	ChameleonDirPath string

	// ChameleonDistPath is the path to the chameleon dist directory where
	// chameleond bundles are built to.
	ChameleonDistDirPath string
}

// NewSrcDirectory creates a new SrcDirectory and validates that all required
// directories exist under chromiumosDirPath.
func NewSrcDirectory(chromiumosDirPath string) (*SrcDirectory, error) {
	chromiumosDirPathAbs, err := filepath.Abs(chromiumosDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of chromiumos dir %q: %w", chromiumosDirPath, err)
	}
	if err := assertPathExists(chromiumosDirPathAbs); err != nil {
		return nil, err
	}
	srcDir := &SrcDirectory{
		ChromiumosDirPath:    chromiumosDirPathAbs,
		BTPeerManagerDirPath: filepath.Join(chromiumosDirPathAbs, "src", "platform", "dev", "contrib", "btpeer_manager"),
		ChameleonDirPath:     filepath.Join(chromiumosDirPathAbs, "src", "platform", "chameleon"),
		ChameleonDistDirPath: filepath.Join(chromiumosDirPathAbs, "src", "platform", "chameleon", "dist"),
	}
	pathsToCheck := []string{
		srcDir.ChromiumosDirPath,
		srcDir.BTPeerManagerDirPath,
		srcDir.ChameleonDirPath,
	}
	for _, path := range pathsToCheck {
		if err := assertPathExists(path); err != nil {
			return nil, err
		}
	}
	return srcDir, nil
}

func assertPathExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path %q does not exist: %w", path, err)
		}
		return fmt.Errorf("failed to get file info for path %q: %w", path, err)
	}
	return nil
}
