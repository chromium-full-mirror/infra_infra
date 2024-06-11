// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli represents the CLI command grouping
package cli

import (
	"flag"
	"strings"

	"infra/cros/cmd/container_uprev/executions"
)

// RevertCommand runs revert. This is only used for reversions in prod/staging.
type RevertCommand struct {
	flagSet *flag.FlagSet
	args    *revertArgs
}

type revertArgs struct {
	containerNames string
	isProd         bool
}

func NewRevertCommand() *RevertCommand {
	cc := &RevertCommand{
		flagSet: flag.NewFlagSet("revert", flag.ContinueOnError),
	}

	return cc
}

func (cc *RevertCommand) Is(group string) bool {
	return strings.HasPrefix(group, "r")
}

func (cc *RevertCommand) Name() string {
	return "revert"
}

func (cc *RevertCommand) Init(args []string) error {
	a := revertArgs{}
	cc.args = &a
	cc.flagSet.StringVar(&a.containerNames, "containers", "all", "the containers to be reverted")
	cc.flagSet.BoolVar(&a.isProd, "prod", false, "indicates to revert prod")

	err := cc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

func (cc *RevertCommand) Run() error {
	containerNames := strings.Split(cc.args.containerNames, ",")
	executions.RevertExecution(containerNames, cc.args.isProd)
	return nil
}
