// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	croscommon "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/vm_leaser/client"
)

const abandonCmd = "abandon"

var abandon = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...]", abandonCmd),
	ShortDesc: "Abandon VMs which were previously leased via 'vm lease'",
	LongDesc: `Abandon VMs which were previously leased via 'vm lease'.

If no VM name is specified, all active leases by the current user will be
abandoned.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &abandonRun{}
		c.envFlags.register(&c.Flags)
		c.abandonFlags.register(&c.Flags)
		return c
	},
}

type abandonRun struct {
	subcommands.CommandRunBase
	envFlags
	abandonFlags
}

func (c *abandonRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		common.PrintCmdError(a, err)
		return 1
	}
	return 0
}

func (c *abandonRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)

	config, err := c.envFlags.getClientConfig()
	if err != nil {
		return err
	}
	vmLeaser, err := client.NewClient(ctx, config)
	if err != nil {
		return err
	}
	defer vmLeaser.Close()

	vms, err := listLeases(vmLeaser, ctx)
	if err != nil {
		return err
	}

	if len(vms) == 0 {
		return errors.New("No active VM leases")
	}

	abandoned, err := abandonVMs(ctx, vmLeaser, vms, c.abandonFlags.name)
	if err != nil {
		return err
	}

	if len(abandoned) == 0 {
		return errors.New("No active leased VM with matching criteria")
	}

	fmt.Printf("Abandoned %d VM(s)\n", len(abandoned))
	printVMList(abandoned, os.Stdout)

	return nil
}

// abandonVMs abandons a list of VMs. If name is not empty, only VM matching the
// name will be abandoned.
func abandonVMs(ctx context.Context, vmLeaser *client.Client, vms []*api.VM, name string) ([]*api.VM, error) {
	abandoned := []*api.VM{}
	for _, vm := range vms {
		if name != "" && name != vm.GetId() {
			continue
		}

		if _, err := vmLeaser.VMLeaserClient.ReleaseVM(ctx, &api.ReleaseVMRequest{
			LeaseId:    vm.GetId(),
			GceProject: croscommon.GceProject,
			GceRegion:  vm.GetGceRegion(),
		}); err != nil {
			return nil, err
		}
		abandoned = append(abandoned, vm)
	}
	return abandoned, nil
}

// abandonFlags contains parameters for the "vm abandon" subcommand.
type abandonFlags struct {
	name string
}

// Registers abandon-specific flags.
func (c *abandonFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.name, "name", "", "Name of the instance to abandon")
}
