// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package meta contains functionality around management of the Satlab CLI binary itself.

package meta

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

// SkipAutoUpdateEnvVar is the env var we look at to determine if we should not
// attempt to autoupdate.
const SkipAutoUpdateEnvVar = "SKIP_AUTO_UPDATE"

// CipdSatLabBinaryPath is the path to the satlab binary.
const CipdSatLabBinaryPath = CipdRoot + "/satlab"

// Deps defines external dependencies used for updating and running the binary.
type Deps interface {
	RunCommand(app *cli.Application, args []string) int
	OpenFile(name string) (*os.File, error)
	EnvGetter(key string) string
	ReExecFunc(args []string) error
}

// DefaultDeps provides the default implementation of the dependencies.
type DefaultDeps struct{}

func (d *DefaultDeps) RunCommand(app *cli.Application, args []string) int {
	return subcommands.Run(app, args)
}

func (d *DefaultDeps) OpenFile(name string) (*os.File, error) {
	return os.Open(name)
}

func (d *DefaultDeps) EnvGetter(key string) string {
	return os.Getenv(key)
}

func (d *DefaultDeps) ReExecFunc(args []string) error {
	cmd := exec.Command(CipdSatLabBinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to re-execute updated binary: %v\n", err)
		return err
	}
	return nil
}

// Updater encapsulates the logic for updating and running the binary.
type Updater struct {
	app        *cli.Application
	binaryPath string
	deps       Deps
}

// NewUpdater creates an Updater with the default dependencies.
func NewUpdater(app *cli.Application) *Updater {
	return &Updater{
		app:        app,
		binaryPath: CipdSatLabBinaryPath,
		deps:       &DefaultDeps{},
	}
}

// NewUpdaterWithDeps creates an Updater with custom dependencies.
func NewUpdaterWithDeps(app *cli.Application, deps Deps, binaryPath string) *Updater {
	return &Updater{
		app:        app,
		binaryPath: binaryPath,
		deps:       deps,
	}
}

// UpdateThenRun is the entry point that updates and then executes the CLI command.
func UpdateThenRun(app *cli.Application) int {
	u := NewUpdater(app)
	if u.shouldUpdate() {
		updated, err := u.attemptUpdate()
		if !updated || err != nil {
			return subcommands.Run(app, nil)
		}
		fmt.Println("Updated satlab CLI.")
		if err := u.reExecuteBinary(); err != nil {
			return 1
		}
		return 0
	}
	return subcommands.Run(app, nil)
}

// shouldUpdate checks if auto-update should be attempted.
func (u *Updater) shouldUpdate() bool {
	return !misc.BoolVal(u.deps.EnvGetter(SkipAutoUpdateEnvVar))
}

// attemptUpdate tries to update the binary and returns true if an update occurred.
func (u *Updater) attemptUpdate() (bool, error) {
	oldHash, err := u.calculateFileHash(u.binaryPath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate old binary hash: %w", err)
	}

	// Execute the upgrade command.
	_ = u.deps.RunCommand(u.app, []string{"upgrade", "-silent"})

	newHash, err := u.calculateFileHash(u.binaryPath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate new binary hash: %w", err)
	}
	return oldHash != newHash, nil
}

// reExecuteBinary re-executes the current binary with the same arguments.
func (u *Updater) reExecuteBinary() error {
	args := os.Args[1:]
	return u.deps.ReExecFunc(args)
}

// calculateFileHash computes the MD5 hash of a file.
func (u *Updater) calculateFileHash(filePath string) (string, error) {
	f, err := u.deps.OpenFile(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
