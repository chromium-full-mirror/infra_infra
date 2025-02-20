// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/maruel/subcommands"

	"infra/cmd/vmlab/internal/config"
	"infra/libs/vmlab"
	"infra/libs/vmlab/api"
)

var CleanupInstancesCmd = &subcommands.Command{
	UsageLine: "cleanup-instances",
	ShortDesc: "clean up VM instances in the given config and filters",
	CommandRun: func() subcommands.CommandRun {
		c := &cleanupInstancesRun{}
		c.cleanupInstancesFlags.register(&c.Flags)
		return c
	},
}

type cleanupInstancesFlags struct {
	configName      string
	swarmingBotName string
	rate            int
	dryRun          bool
	json            bool
}

func (c *cleanupInstancesFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.configName, "config", "", "Config name to use. cts-prototype is the only valid option at this momenet.")
	f.StringVar(&c.swarmingBotName, "swarming-bot-name", "", "Only cleanup instances matching given swarming bot name.")
	f.IntVar(&c.rate, "rate", 1, "Rate limit for delete API calls in requests/second.")
	f.BoolVar(&c.dryRun, "dry-run", false, "Test run without really deleting instances. Default is false.")
	f.BoolVar(&c.json, "json", false, "Output json result.")
}

type cleanupInstancesRun struct {
	subcommands.CommandRunBase
	cleanupInstancesFlags
}

type cleanupInstancesResult struct {
	Total   int
	Deleted []string
	Failed  []string
}

func (c *cleanupInstancesRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := context.Background()
	if c.cleanupInstancesFlags.configName == "" {
		fmt.Fprintln(os.Stderr, "Config name must be set.")
		return 1
	}
	cleanupConfig, ok := config.ConfigMapping[c.cleanupInstancesFlags.configName]
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot find config.")
		return 1
	}

	insApi, err := vmlab.NewInstanceApi(cleanupConfig.ProviderId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot get instance api provider: %v\n", err)
		return 1
	}

	result, err := cleanupInstances(ctx, insApi, cleanupConfig, c.cleanupInstancesFlags.swarmingBotName, c.cleanupInstancesFlags.rate, c.cleanupInstancesFlags.dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clean up instances: %v\n", err)
		return 1
	}

	if c.cleanupInstancesFlags.json {
		if jsonResult, err := json.Marshal(result); err != nil {
			fmt.Fprintf(os.Stderr, "BUG! Cannot convert output to json: %v\n", err)
		} else {
			fmt.Println(string(jsonResult))
		}
	} else {
		fmt.Printf("Total instances: %d\n", result.Total)
		fmt.Println("Deleted instances:")
		for _, instanceName := range result.Deleted {
			fmt.Println(instanceName)
		}
		fmt.Println("Failed to delete instances:")
		for _, instanceName := range result.Failed {
			fmt.Println(instanceName)
		}
	}

	if len(result.Failed) > 0 {
		return 1
	}

	return 0
}

func generateListVmRequest(cleanupConfig *config.BuiltinConfig, swarmingBotName string) (*api.ListVmInstancesRequest, error) {
	tags := map[string]string{}
	if len(swarmingBotName) > 0 {
		tags["swarming-bot-name"] = swarmingBotName
	}
	switch provider := cleanupConfig.ProviderId; provider {
	case api.ProviderId_GCLOUD:
		return &api.ListVmInstancesRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &cleanupConfig.GcloudConfig,
				},
			},
			TagFilters: tags,
		}, nil
	default:
		return nil, errors.New("Cannot identify backend provider for given config.")
	}
}

// cleanupInstances cleans up instances that matches swarmingBotName. `rate`
// sets a limit on the number of delete instance API requests per second. When
// `dryRun` is true it doesn't call instance delete API. Returns deleted,
// failed to delete instances in `cleanupInstancesResult`.
func cleanupInstances(ctx context.Context, insApi api.InstanceApi, config *config.BuiltinConfig, swarmingBotName string, rate int, dryRun bool) (cleanupInstancesResult, error) {
	result := cleanupInstancesResult{
		Total:   0,
		Deleted: []string{},
		Failed:  []string{},
	}

	req, err := generateListVmRequest(config, swarmingBotName)
	if err != nil {
		return result, fmt.Errorf("Cannot generate list request: %w", err)
	}
	instances, err := insApi.List(ctx, req)
	if err != nil {
		return result, fmt.Errorf("Failed to list instances: %w", err)
	}
	result.Total = len(instances)

	var wg sync.WaitGroup
	var mu sync.Mutex
	limiter := time.NewTicker(time.Second / time.Duration(rate))

	for _, instance := range instances {
		<-limiter.C
		if dryRun {
			result.Deleted = append(result.Deleted, instance.Name)
			continue
		}

		wg.Add(1)
		go func(instance *api.VmInstance) {
			defer wg.Done()
			if err := insApi.Delete(ctx, instance); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to delete instances%s: %v\n", instance.Name, err)
				mu.Lock()
				result.Failed = append(result.Failed, instance.Name)
				mu.Unlock()
				return
			}
			mu.Lock()
			result.Deleted = append(result.Deleted, instance.Name)
			mu.Unlock()
		}(instance)
	}
	wg.Wait()

	return result, nil
}
