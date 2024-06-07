// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli represents the CLI command grouping
package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"infra/cros/cmd/container_uprev/executions"
)

// CLICommand runs  in CLI mode. This will only be used for local debugging, not deployment.
type CLICommand struct {
	flagSet *flag.FlagSet
	args    *cliArgs
}

type cliArgs struct {
	cipdLabel  string
	imageTag   string
	runAsAdmin bool
}

func NewCLICommand() *CLICommand {
	cc := &CLICommand{
		flagSet: flag.NewFlagSet("cli", flag.ContinueOnError),
	}

	return cc
}

func (cc *CLICommand) Is(group string) bool {
	return strings.HasPrefix(group, "c")
}

func (cc *CLICommand) Name() string {
	return "cli"
}

func (cc *CLICommand) Init(args []string) error {
	a := cliArgs{}
	cc.args = &a
	cc.flagSet.StringVar(&a.cipdLabel, "label", "latest", "the CIPD label to be used. Overridden if package has predefined cipd label")
	cc.flagSet.StringVar(&a.imageTag, "tag", "", "tag the image will be tagged with. Defaults to ${USER}-test")
	cc.flagSet.BoolVar(&a.runAsAdmin, "admin", false, "remove restrictions for staging/prod labels and allow upload to sha storage")

	err := cc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

func (cc *CLICommand) Run() error {
	tag := cc.args.imageTag
	if tag == "" {
		tag = fmt.Sprintf("%s-test", os.Getenv("USER"))
	}
	executions.LocalBuildExecution(tag, cc.args.cipdLabel, cc.args.runAsAdmin)
	return nil
}
