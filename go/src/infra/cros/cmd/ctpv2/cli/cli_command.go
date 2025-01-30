// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli command grouping
package cli

import (
	"flag"
	"log"
	"strings"
)

// CLICommand runs CTPv2 in CLI mode. This will only be used for local debugging, not deployment.
type CLICommand struct {
	flagSet *flag.FlagSet
	args    *argsStruct
}

type argsStruct struct {
	// Common input params.
	inputPath string
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
	a := argsStruct{}
	cc.args = &a
	cc.flagSet.StringVar(&a.inputPath, "input", "/tmp/test/ctp2Request", "specify the ctp2 request json input file")

	err := cc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

// Run runs the commands to publish test results
func (cc *CLICommand) Run() error {
	log.Printf("Running CLI Mode: %s", cc.args.inputPath)
	// TODO: implement local execution flow
	//_, err := internal.Execute(cc.args.inputPath, false)
	return nil
}
