// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Responsible for the abstraction layer representing each command grouping
package cli

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"go.chromium.org/infra/cros/cmd/provision/foil-provision/constants"
)

// AbstractCommand represents a CLI grouping (e.g.: run as server, run as CLI, etc)
type AbstractCommand interface {
	// Run runs the command
	Run() error

	// Is checks if the string is representative of the command
	Is(string) bool

	// Init is an initializer for the command given the trailing args
	Init([]string) error

	// Name is the command name (for debugging)
	Name() string
}

// SetUpLog sets up the logging for the CLI
func SetUpLog(dir string) (*log.Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %v: %w", dir, err)
	}
	lfp := filepath.Join(dir, "log.txt")
	lf, err := os.Create(lfp)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %v: %w", lfp, err)
	}
	newLog := log.New(io.MultiWriter(lf, os.Stderr), "<foil-provision>", log.LstdFlags|log.LUTC)
	newLog.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)

	// CLEAN(b/408454320): Remove once adb-logcat is containerized.
	constants.LogFileDir = dir

	return newLog, nil
}

// ParseInputs is a helper method which parses input arguments. It is
// effectively a factory method.
func ParseInputs() (AbstractCommand, error) {
	if len(os.Args) < 1 {
		return nil, errors.New("CLI arguments must be specified")
	}

	cmds := []AbstractCommand{
		NewServerCommand(),
	}

	subcommand := os.Args[1]
	options := []string{}

	for _, cmd := range cmds {
		options = append(options, cmd.Name())
		if cmd.Is(subcommand) {
			if err := cmd.Init(os.Args[2:]); err != nil {
				return nil, fmt.Errorf("failed to initialize cli command, %w", err)
			}
			return cmd, nil
		}
	}

	return nil, fmt.Errorf("Unknown subcommand: %s. \nOptions are: [%s]", subcommand, options)

}
