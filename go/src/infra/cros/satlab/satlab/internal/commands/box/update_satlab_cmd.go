// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package box

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type readContentsFunc func(context.Context, executor.IExecCommander) (bool, error)

// UpdateSatlabCmd is the command to check whether reboot is needed for Satlab to update
var UpdateSatlabCmd = &subcommands.Command{
	UsageLine: "satlab",
	ShortDesc: "check if there is a satlab update available",
	LongDesc:  "Check if there is a satlab update available",
	CommandRun: func() subcommands.CommandRun {
		return &updateSatlabRun{}
	},
}

// updateSatlabRun struct contains the arguments needed to run UpdateSatlabCmd
type updateSatlabRun struct {
	subcommands.CommandRunBase
}

// Run is what is called when a user inputs the updateSatlabRun command
func (c *updateSatlabRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun contains business logic
func (c *updateSatlabRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	return c.runCmdInjected(ctx, os.Stdout, satlabcommands.IsUpdateAvailable)
}

func (c *updateSatlabRun) runCmdInjected(ctx context.Context, w io.Writer, readContents readContentsFunc) error {
	contents, err := readContents(ctx, &executor.ExecCommander{})
	if err != nil {
		return errors.Annotate(err, "read contents").Err()
	}

	fmt.Fprintf(w, "%t\n", contents)
	return nil
}
