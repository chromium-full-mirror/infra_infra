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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/infra/cmd/vmlab/internal/config"
	"go.chromium.org/infra/libs/vmlab"
	"go.chromium.org/infra/libs/vmlab/api"
)

var LeaseCmd = &subcommands.Command{
	UsageLine: "lease",
	ShortDesc: "lease a virtual machine from given inputs",
	CommandRun: func() subcommands.CommandRun {
		c := &leaseRun{}
		c.leaseFlags.register(&c.Flags)
		c.gcloudBackendFlags.register(&c.Flags)
		return c
	},
}

type leaseFlags struct {
	configName      string
	expireAfter     int
	swarmingBotName string
	json            bool
}

type gcloudBackendFlags struct {
	gceImageProject string
	gceImageName    string
}

func (c *leaseFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.configName, "config", "", "Config name to use. cts-prototype is the only valid option at this momenet.")
	f.IntVar(&c.expireAfter, "expire-after", -1, "Created VM instance should expire and be destroyed after given seconds. Not all backends support this.")
	f.StringVar(&c.swarmingBotName, "swarming-bot-name", "", "Name of the swarming bot name. You can use the same same to do bulk cleanup for some backends.")
	f.BoolVar(&c.json, "json", false, "Output json result.")
}

func (c *gcloudBackendFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.gceImageProject, "gce-image-project", "", "GCE image project, example: betty-cloud-prototype")
	f.StringVar(&c.gceImageName, "gce-image-name", "", "GCE image name, example: betty-arc-r-release-r112-15350-0-0")
}

type leaseRun struct {
	subcommands.CommandRunBase
	leaseFlags
	gcloudBackendFlags
}

func generateCreateRequest(createConfig *config.BuiltinConfig, c *leaseRun, tags map[string]string) (*api.CreateVmInstanceRequest, error) {
	switch provider := createConfig.ProviderId; provider {
	case api.ProviderId_GCLOUD:
		if c.gcloudBackendFlags.gceImageProject == "" {
			return nil, errors.New("gce-image-project must be set.")
		}
		if c.gcloudBackendFlags.gceImageName == "" {
			return nil, errors.New("gce-image-name must be set.")
		}
		gcloudBackendConfig := proto.Clone(&createConfig.GcloudConfig).(*api.Config_GCloudBackend)
		gcloudBackendConfig.Image = &api.GceImage{
			Project: c.gcloudBackendFlags.gceImageProject,
			Name:    c.gcloudBackendFlags.gceImageName,
		}
		return &api.CreateVmInstanceRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: gcloudBackendConfig,
				},
			},
			Tags: tags,
		}, nil
	default:
		return nil, errors.New("Cannot identify backend provider for given config.")
	}
}

func (c *leaseRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := context.Background()
	if c.leaseFlags.configName == "" {
		fmt.Fprintln(os.Stderr, "Config name must be set.")
		return 1
	}
	createConfig, ok := config.ConfigMapping[c.leaseFlags.configName]
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot find config.")
		return 1
	}
	if createConfig.ProviderId == api.ProviderId_GCLOUD {
	}
	tags := map[string]string{}
	if c.swarmingBotName != "" {
		tags["swarming-bot-name"] = c.swarmingBotName
	}
	request, err := generateCreateRequest(createConfig, c, tags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot generate request: %v", err)
	}
	ins, err := vmlab.NewInstanceApi(createConfig.ProviderId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create instance: %v", err)
	}
	createdInstance, err := ins.Create(ctx, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create instance: %v", err)
		return 1
	}
	if c.leaseFlags.json {
		if instanceJson, err := protojson.Marshal(createdInstance); err != nil {
			fmt.Fprintf(os.Stderr, "BUG! Instance created bug cannot convert output to josn: %v", err)
		} else {
			fmt.Println(string(instanceJson))
		}
	} else {
		fmt.Printf("Instance named %s created at ssh %s:%d\n", createdInstance.Name, createdInstance.Ssh.Address, createdInstance.Ssh.Port)
	}
	return 0
}
