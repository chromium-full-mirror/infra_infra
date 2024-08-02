// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shivas

import (
	"fmt"
	"io"
	"os/exec"

	"go.chromium.org/luci/common/errors"

	"infra/cros/satlab/common/commands"
	"infra/cros/satlab/common/paths"
	e "infra/cros/satlab/common/utils/errors"
	"infra/cros/satlab/common/utils/executor"
	"infra/cros/satlab/common/utils/misc"
)

// Asset is a group of parameters needed to add an asset to UFS.
type Asset struct {
	Asset     string
	Rack      string
	Zone      string
	Model     string
	Board     string
	Namespace string
	Type      string
}

// CheckAndAdd adds the asset if it does not already exist.
func (a *Asset) CheckAndAdd(executor executor.IExecCommander, w io.Writer) (bool, error) {
	exists, err := a.exists(executor, w)
	if err != nil {
		return false, errors.Annotate(err, "check and update").Err()
	}
	if !exists {
		return false, a.add(executor, w)
	} else {
		fmt.Fprintf(w, "Asset already added\n\n")
	}
	return true, nil
}

// exists checks for the existence of the UFS asset.
// For now does so based on whether `shivas get asset` has stdout :(
func (a *Asset) exists(executor executor.IExecCommander, w io.Writer) (bool, error) {
	flags := map[string][]string{
		"rack":      {a.Rack},
		"zone":      {a.Zone},
		"model":     {a.Model},
		"board":     {a.Board},
		"namespace": {a.Namespace},
		// Type cannot be provided when getting a DUT.
	}

	args := (&commands.CommandWithFlags{
		Commands:       []string{paths.ShivasCLI, "get", "asset"},
		PositionalArgs: []string{a.Asset},
		Flags:          flags,
		AuthRequired:   true,
	}).ToCommand()
	fmt.Fprintf(w, "Check asset exists: run %s\n", args)

	command := exec.Command(args[0], args[1:]...)
	// Don't use `CombinedOutput` here because it returns
	// `rpc error: code = NotFound` that means the asset doesn't exist.
	// As we check only the length of output.
	stdout, err := executor.Output(command)

	if err != nil {
		return false, errors.Annotate(e.HandleExitError(err), "add asset").Err()
	}

	// if asset not found, shivas returns output in stderr, stdout is empty.
	exists := (len(stdout) != 0)

	return exists, nil
}

// Add adds an asset unconditionally to UFS.
func (a *Asset) add(executor executor.IExecCommander, w io.Writer) error {
	// Add the asset.
	fmt.Fprintf(w, "Adding asset\n")
	flags := map[string][]string{
		"model":     {a.Model},
		"board":     {a.Board},
		"rack":      {a.Rack},
		"zone":      {a.Zone},
		"name":      {a.Asset},
		"namespace": {a.Namespace},
		"type":      {a.Type},
	}

	args := (&commands.CommandWithFlags{
		Commands:     []string{paths.ShivasCLI, "add", "asset"},
		Flags:        flags,
		AuthRequired: true,
	}).ToCommand()
	fmt.Fprintf(w, "Add asset: run %s\n", args)

	command := exec.Command(args[0], args[1:]...)
	out, err := executor.CombinedOutput(command)
	fmt.Fprintln(w, misc.TrimOutput(out))

	return e.HandleExitError(err)
}
