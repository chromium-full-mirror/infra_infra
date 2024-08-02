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

// Rack is a group of arguments for adding a rack.
type Rack struct {
	Name      string
	Namespace string
	Zone      string
}

// CheckAndAdd runs check and then update if the item does not exist.
func (r *Rack) CheckAndAdd(executor executor.IExecCommander, w io.Writer) (bool, error) {
	exists, err := r.exists(executor, w)
	if err != nil {
		return false, errors.Annotate(err, "check and update").Err()
	}
	if !exists {
		return false, r.add(executor, w)
	} else {
		fmt.Fprintf(w, "Rack already added\n\n")
	}
	return true, nil
}

// check checks if a rack exists.
// For now does so based on whether `shivas get rack` errors :(
func (r *Rack) exists(executor executor.IExecCommander, w io.Writer) (exists bool, err error) {
	flags := map[string][]string{
		"namespace": {r.Namespace},
	}
	args := (&commands.CommandWithFlags{
		Commands:       []string{paths.ShivasCLI, "get", "rack"},
		PositionalArgs: []string{r.Name},
		Flags:          flags,
		AuthRequired:   true,
	}).ToCommand()
	fmt.Fprintf(w, "Check rack exists: run %s\n", args)

	cmd := exec.Command(args[0], args[1:]...)
	// Don't use `CombinedOutput` here because it returns
	// `rpc error: code = NotFound` that means the rack doesn't exist.
	// As we check only the length of output.
	stdout, err := executor.Output(cmd)

	if err != nil {
		return false, errors.Annotate(e.HandleExitError(err), "add rack").Err()
	}

	// if rack not found, shivas returns output in stderr, and stdout is empty.
	exists = (len(stdout) != 0)

	return exists, nil
}

// add adds a rack unconditionally to UFS.
func (r *Rack) add(executor executor.IExecCommander, w io.Writer) error {
	flags := map[string][]string{
		// TODO(gregorynisbet): Default to OS for everything.
		"namespace": {r.Namespace},
		"name":      {r.Name},
		"zone":      {r.Zone},
	}

	fmt.Fprintf(w, "Adding rack\n")
	args := (&commands.CommandWithFlags{
		Commands:     []string{paths.ShivasCLI, "add", "rack"},
		Flags:        flags,
		AuthRequired: true,
	}).ToCommand()
	fmt.Fprintf(w, "Add rack: run %s\n", args)

	command := exec.Command(args[0], args[1:]...)
	out, err := executor.CombinedOutput(command)

	fmt.Fprintln(w, misc.TrimOutput(out))

	return e.HandleExitError(err)
}
