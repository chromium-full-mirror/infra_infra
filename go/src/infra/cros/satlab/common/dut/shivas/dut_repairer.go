// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shivas

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"

	"infra/cros/satlab/common/commands"
	"infra/cros/satlab/common/paths"
	"infra/cros/satlab/common/site"
	e "infra/cros/satlab/common/utils/errors"
	"infra/cros/satlab/common/utils/executor"
)

type RepairAction string

const (
	// Verify run only verify actions.
	Verify RepairAction = "-verify"
	// DeepRepair use deep-repair task when scheduling a task.
	DeepRepair RepairAction = "-deep"
	// Normal don't specify `verify` and `deep` flag to shivas CLI
	Normal RepairAction = ""
)

// DUTRepairer repairs a DUT with the given name.
type DUTRepairer struct {
	Name      string
	Namespace string
	Executor  executor.IExecCommander
}

type DUTRepairResponse struct {
	BuildLink string
	TaskLink  string
}

var linkRe = regexp.MustCompile(`(?:https?:\/\/)?[\w/\-?=%.]+\.[\w/\-&?=%.]+[\d]`)

// repair invokes shivas with the required arguments to repair a DUT.
func (u *DUTRepairer) Repair(
	ctx context.Context,
	action RepairAction,
) (*DUTRepairResponse, error) {
	cmd := []string{paths.ShivasCLI, "repair-duts"}
	if string(action) != "" {
		cmd = append(cmd, string(action))
	}
	args := (&commands.CommandWithFlags{
		Commands: cmd,
		Flags: map[string][]string{
			"bucket":    {site.GetDeployBucket()},
			"builder":   {site.RepairBuilderName},
			"namespace": {u.Namespace},
		},
		PositionalArgs: []string{u.Name},
		AuthRequired:   true,
	}).ToCommand()
	command := exec.CommandContext(ctx, args[0], args[1:]...)

	// Don't use `CombinedOutput` here because it returns
	// `rpc error: code = NotFound desc = requested resources not found`
	// "anonymous:anonymous" does not have permission to view it.
	out, err := u.Executor.Output(command)
	if err != nil {
		return nil, e.HandleExitError(err)
	}

	rawData := string(out)
	// extract the urls from the output
	matches := linkRe.FindAllString(rawData, -1)
	// we expected there are two urls
	if len(matches) != 2 {
		return nil, errors.New(fmt.Sprintf("Can't parse the url from the output: %v\n", rawData))
	}

	return &DUTRepairResponse{BuildLink: matches[0], TaskLink: matches[1]}, nil
}
