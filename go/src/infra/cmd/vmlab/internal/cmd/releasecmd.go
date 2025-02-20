// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/infra/cmd/vmlab/internal/config"
	"go.chromium.org/infra/libs/vmlab"
	"go.chromium.org/infra/libs/vmlab/api"
)

var ReleaseCmd = &subcommands.Command{
	UsageLine: "release",
	ShortDesc: "release a virtual machine from given inputs",
	CommandRun: func() subcommands.CommandRun {
		c := &releaseRun{}
		c.releaseFlags.register(&c.Flags)
		return c
	},
}

type releaseFlags struct {
	configName   string
	instanceName string
}

func (c *releaseFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.configName, "config", "", "Config name to use. cts-prototype is the only valid option at this momenet.")
	f.StringVar(&c.instanceName, "instance-name", "", "Name of the instance leased.")
}

type releaseRun struct {
	subcommands.CommandRunBase
	releaseFlags
}

func generateInstanceToDelete(releaseConfig *config.BuiltinConfig, c *releaseRun) (*api.VmInstance, error) {
	switch provider := releaseConfig.ProviderId; provider {
	case api.ProviderId_GCLOUD:
		return &api.VmInstance{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &releaseConfig.GcloudConfig,
				},
			},
			Name: c.instanceName,
		}, nil
	default:
		return nil, errors.New("Cannot identify backend provider for given config.")
	}
}

func (c *releaseRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := context.Background()
	if c.releaseFlags.configName == "" {
		fmt.Fprintln(os.Stderr, "Config name must be set.")
		return 1
	}
	if c.releaseFlags.instanceName == "" {
		fmt.Fprintln(os.Stderr, "Instance name must be set.")
		return 1
	}
	releaseConfig, ok := config.ConfigMapping[c.releaseFlags.configName]
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot find config.")
		return 1
	}
	instance, err := generateInstanceToDelete(releaseConfig, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot generate request: %v", err)
	}
	ins, err := vmlab.NewInstanceApi(releaseConfig.ProviderId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot delete instance: %v", err)
	}
	err = ins.Delete(ctx, instance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete instance: %v", err)
		return 1
	}
	fmt.Println("Instance deleted")
	return 0
}
