// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli represents the CLI command grouping
package cli

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/container_uprev/executions"
)

// ManualCommand submits manual SHAs into the firestore.
type ManualCommand struct {
	flagSet *flag.FlagSet
	args    *manualArgs
}

type manualArgs struct {
	name          string
	containerName string
	digest        string
	hostname      string
	project       string
	prod          bool
}

func NewManualCommand() *ManualCommand {
	cc := &ManualCommand{
		flagSet: flag.NewFlagSet("manual", flag.ContinueOnError),
	}
	return cc
}

func (cc *ManualCommand) Is(group string) bool {
	return strings.HasPrefix(group, "manual")
}

func (cc *ManualCommand) Name() string {
	return "manual"
}

func (cc *ManualCommand) Init(args []string) error {
	a := manualArgs{}
	cc.args = &a
	cc.flagSet.StringVar(&a.name, "name", "", "(Required) The name of the container's firestore name")
	cc.flagSet.StringVar(&a.containerName, "container-name", "", "The name of the container. If empty, use the firestore name")
	cc.flagSet.BoolVar(&a.prod, "prod", false, "Send to production firestore")
	cc.flagSet.StringVar(&a.digest, "digest", "", "(Required) The sha256 value of the container being stored")
	cc.flagSet.StringVar(&a.hostname, "hostname", "us-docker.pkg.dev", "Repository's hostname of where the container is stored")
	cc.flagSet.StringVar(&a.project, "project", "cros-registry/test-services", "Repository's project of where the container is stored")

	err := cc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

func (cc *ManualCommand) Run() error {
	if cc.args.name == "" {
		return fmt.Errorf("name must have a value")
	}
	if cc.args.containerName == "" {
		cc.args.containerName = cc.args.name
	}
	if !strings.HasPrefix(cc.args.digest, "sha256:") {
		cc.args.digest = "sha256:" + cc.args.digest
	}

	containerItem := &common.ContainerInfoItem{
		TimeRecord:         time.Now().UTC(),
		RepositoryHostname: cc.args.hostname,
		RepositoryProject:  cc.args.project,
		Digest:             cc.args.digest,
		ContainerName:      cc.args.containerName,
	}
	executions.ManualExecution(cc.args.name, containerItem, cc.args.prod)
	return nil
}
