// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// UpdateVMCmd update VM on a host.
var UpdateVMCmd = &subcommands.Command{
	UsageLine: "vm [Options...]",
	ShortDesc: "Update a VM on a host",
	LongDesc: `Update a VM on a host

Examples:
shivas update vm -f vm.json
Update a VM on a host by reading a JSON file input.
[WARNING]: machineLseId is a required field in json, all other output only fields will be ignored.
Specify additional settings, e.g. vlan, ip, state via command line parameters along with JSON input

shivas update vm -name cr22 -os windows
Partial update a vm by parameters. Only specified parameters will be updated in the vm.`,
	CommandRun: func() subcommands.CommandRun {
		c := &updateVM{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.newSpecsFile, "f", "", cmdhelp.VMFileText)

		c.Flags.StringVar(&c.hostName, "host", "", "hostname of the host to add the VM")
		c.Flags.StringVar(&c.vmName, "name", "", "hostname/name of the VM")
		c.Flags.StringVar(&c.macAddress, "mac", "", "mac address of the VM. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.osVersion, "os", "", "os version of the VM. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.osImage, "os-image", "", "the os image of the VM. "+cmdhelp.ClearFieldHelpText)
		c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of tag(s). Can be specified multiple times. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.description, "desc", "", "description for the vm. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.deploymentTicket, "ticket", "", "the deployment ticket for this vm. "+cmdhelp.ClearFieldHelpText)

		c.Flags.StringVar(&c.vlanName, "vlan", "", "name of the vlan to assign this vm to")
		c.Flags.BoolVar(&c.deleteVlan, "delete-vlan", false, "if deleting the ip assignment for the vm")
		c.Flags.StringVar(&c.ip, "ip", "", "the ip to assign the vm to")
		c.Flags.StringVar(&c.state, "state", "", cmdhelp.StateHelp)
		c.Flags.IntVar(&c.cpuCores, "cpu-cores", 0, "number of CPU cores. To clear this field set it to -1.")
		c.Flags.StringVar(&c.memory, "memory", "", "amount of memory in bytes assigned. "+cmdhelp.ByteUnitsAcceptedText+" "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.storage, "storage", "", "disk storage capacity in bytes assigned. "+cmdhelp.ByteUnitsAcceptedText+" "+cmdhelp.ClearFieldHelpText)
		c.Flags.Int64Var(&c.vmid, "vmid", -1, "Proxmox unique ID of the VM (100 - 999999999).")
		return c
	},
}

type updateVM struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	newSpecsFile string

	hostName         string
	vmName           string
	vlanName         string
	deleteVlan       bool
	ip               string
	state            string
	macAddress       string
	osVersion        string
	osImage          string
	tags             []string
	description      string
	deploymentTicket string
	cpuCores         int
	memory           string
	storage          string
	vmid             int64
}

func (c *updateVM) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateVM) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ns, err := c.envFlags.Namespace(nil, "")
	if err != nil {
		return err
	}
	ctx = utils.SetupContext(ctx, ns)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	// Parse the json input
	var vm ufspb.VM
	if c.newSpecsFile != "" {
		if err = utils.ParseJSONFile(c.newSpecsFile, &vm); err != nil {
			return err
		}
		if vm.GetMachineLseId() == "" {
			return errors.New("machineLseId field is empty in json. It is a required parameter for json input.")
		}
	} else {
		c.parseArgs(&vm)
	}
	if err := utils.PrintExistingVM(ctx, ic, vm.Name); err != nil {
		return err
	}
	var nwOpt *ufsAPI.NetworkOption
	if c.deleteVlan || c.vlanName != "" || c.ip != "" {
		nwOpt = &ufsAPI.NetworkOption{
			Delete: c.deleteVlan,
			Vlan:   c.vlanName,
			Ip:     c.ip,
		}
	}

	vm.Name = ufsUtil.AddPrefix(ufsUtil.VMCollection, vm.Name)
	if !ufsUtil.ValidateTags(vm.Tags) {
		return fmt.Errorf(ufsAPI.InvalidTags)
	}
	res, err := ic.UpdateVM(ctx, &ufsAPI.UpdateVMRequest{
		Vm:            &vm,
		NetworkOption: nwOpt,
		UpdateMask: utils.GetUpdateMask(&c.Flags, map[string]string{
			"host":      ufsUtil.MachineLseIdPath,
			"state":     ufsUtil.ResourceStatePath,
			"mac":       ufsUtil.MacAddressPath,
			"os":        ufsUtil.OsVersionValuePath,
			"os-image":  ufsUtil.OsVersionImagePath,
			"tag":       ufsUtil.TagsPath,
			"desc":      ufsUtil.DescriptionPath,
			"ticket":    ufsUtil.DeploymentTicketPath,
			"cpu-cores": ufsUtil.CpuCoresPath,
			"memory":    ufsUtil.MemoryPath,
			"storage":   ufsUtil.StoragePath,
			"vmid":      ufsUtil.VmidPath,
		}),
	})
	if err != nil {
		return errors.Annotate(err, "Unable to update the VM on the host").Err()
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	c.printRes(ctx, ic, res)
	return nil
}

func (c *updateVM) printRes(ctx context.Context, ic ufsAPI.FleetClient, res *ufspb.VM) {
	fmt.Println("The vm after update:")
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	if c.deleteVlan {
		fmt.Printf("Successfully deleted vlan & ip of vm %s\nPlease run `shivas get vm -full %s` to further check\n", res.Name, res.Name)
	}
	if c.vlanName != "" || c.ip != "" {
		// Log the assigned IP
		if dhcp, err := ic.GetDHCPConfig(ctx, &ufsAPI.GetDHCPConfigRequest{
			Hostname: res.Name,
		}); err == nil {
			fmt.Println("Newly added DHCP config:")
			utils.PrintProtoJSON(dhcp, false)
			fmt.Printf("Successfully added dhcp config %s to vm %s\nPlease run `shivas get vm -full %s` to further check\n", dhcp.GetIp(), res.Name, res.Name)
		}
	}
}

func (c *updateVM) parseArgs(vm *ufspb.VM) {
	vm.Name = c.vmName
	if c.macAddress == utils.ClearFieldValue {
		vm.MacAddress = ""
	} else {
		vm.MacAddress = c.macAddress
	}
	vm.OsVersion = &ufspb.OSVersion{}
	vm.ResourceState = ufsUtil.ToUFSState(c.state)
	vm.MachineLseId = c.hostName
	if c.osVersion == utils.ClearFieldValue {
		vm.GetOsVersion().Value = ""
	} else {
		vm.GetOsVersion().Value = c.osVersion
	}
	if c.osImage == utils.ClearFieldValue {
		vm.GetOsVersion().Image = ""
	} else {
		vm.GetOsVersion().Image = c.osImage
	}
	if ufsUtil.ContainsAnyStrings(c.tags, utils.ClearFieldValue) {
		vm.Tags = nil
	} else {
		vm.Tags = c.tags
	}
	if c.description == utils.ClearFieldValue {
		vm.Description = ""
	} else {
		vm.Description = c.description
	}
	if c.deploymentTicket == utils.ClearFieldValue {
		vm.DeploymentTicket = ""
	} else {
		vm.DeploymentTicket = c.deploymentTicket
	}
	if c.cpuCores == -1 {
		vm.CpuCores = 0
	} else {
		vm.CpuCores = int32(c.cpuCores)
	}
	if c.memory == utils.ClearFieldValue {
		vm.Memory = 0
	} else {
		vm.Memory, _ = utils.ConvertToBytes(c.memory)
	}
	if c.storage == utils.ClearFieldValue {
		vm.Storage = 0
	} else {
		vm.Storage, _ = utils.ConvertToBytes(c.storage)
	}
	if c.vmid == -1 {
		vm.Vmid = 0
	} else {
		vm.Vmid = int64(c.vmid)
	}
}

func (c *updateVM) validateArgs() error {
	if c.newSpecsFile == "" {
		if c.vmName == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required, no mode ('-f') is specified.")
		}
		if c.vlanName == "" && !c.deleteVlan && c.ip == "" && c.state == "" && c.deploymentTicket == "" &&
			c.hostName == "" && c.osVersion == "" && c.osImage == "" && c.macAddress == "" && len(c.tags) == 0 && c.description == "" &&
			c.cpuCores == 0 && c.memory == "" && c.storage == "" && c.vmid == 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nNothing to update. Please provide any field to update")
		}
		if c.state != "" && !ufsUtil.IsUFSState(ufsUtil.RemoveStatePrefix(c.state)) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid state, please check help info for '-state'.", c.state)
		}
		if _, err := utils.ConvertToBytes(c.memory); err != nil && c.memory != utils.ClearFieldValue {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe -memory flag was used incorrectly: %w", err)
		}
		if _, err := utils.ConvertToBytes(c.storage); err != nil && c.storage != utils.ClearFieldValue {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe -storage flag was used incorrectly: %w", err)
		}
	} else {
		if c.vmName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-name' cannot be specified at the same time.")
		}
		if c.hostName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-host' cannot be specified at the same time.")
		}
		if c.macAddress != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-mac' cannot be specified at the same time.")
		}
		if len(c.tags) > 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-tag' cannot be specified at the same time.")
		}
		if c.osVersion != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-os' cannot be specified at the same time.")
		}
		if c.osImage != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-os-image' cannot be specified at the same time.")
		}
		if c.description != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-desc' cannot be specified at the same time.")
		}
		if c.state != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-state' cannot be specified at the same time.")
		}
		if c.deploymentTicket != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-ticket' cannot be specified at the same time.")
		}
		if c.cpuCores != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-cpu-cores' cannot be specified at the same time.")
		}
		if c.memory != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-memory' cannot be specified at the same time.")
		}
		if c.storage != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-storage' cannot be specified at the same time.")
		}
	}
	return nil
}
