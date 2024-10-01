// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"flag"
	"fmt"
	"strings"
)

// helpCommand prints help.
type helpCommand struct {
	flagSet *flag.FlagSet
}

func NewHelpCommand() *helpCommand {
	cc := &helpCommand{
		flagSet: flag.NewFlagSet("help", flag.ContinueOnError),
	}
	return cc
}

func (cc *helpCommand) Is(group string) bool {
	return strings.HasPrefix(group, "help")
}

func (cc *helpCommand) Name() string {
	return "help"
}

func (cc *helpCommand) Init(args []string) error {
	return nil
}

func (cc *helpCommand) Run() error {
	fmt.Println("The tool support: cli|revert|build commands!")
	return nil
}
