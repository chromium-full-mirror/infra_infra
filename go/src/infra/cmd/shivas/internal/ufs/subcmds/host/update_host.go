// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package host

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

// UpdateHostCmd update a host on a machine.
var UpdateHostCmd = &subcommands.Command{
	UsageLine: "host [Options...]",
	ShortDesc: "Update a host(Dev Server, VM Server, Host OS...) on a machine",
	LongDesc:  cmdhelp.UpdateHostLongDesc,
	CommandRun: func() subcommands.CommandRun {
		c := &updateHost{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.newSpecsFile, "f", "", cmdhelp.MachineLSEFileText)
		c.Flags.BoolVar(&c.interactive, "i", false, "enable interactive mode for input")

		c.Flags.StringVar(&c.machineName, "machine", "", "name of the machine to associate the host")
		c.Flags.StringVar(&c.hostName, "name", "", "name of the host")
		c.Flags.StringVar(&c.prototype, "prototype", "", "name of the prototype to be used to deploy this host.")
		c.Flags.StringVar(&c.osVersion, "os", "", "name of the os version of the machine (browser lab only). "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.osImage, "os-image", "", "name of the os image of the machine (browser lab only). "+cmdhelp.ClearFieldHelpText)
		c.Flags.IntVar(&c.vmCapacity, "vm-capacity", 0, "the number of the vms that this machine supports (browser lab only). "+"To clear this field set it to -1.")
		c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of tag(s). Can be specified multiple times. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.description, "desc", "", "description for the vm. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.deploymentTicket, "ticket", "", "the deployment ticket for this host. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.vdc, "vdc", "", "the virtual datacenter a browser host belongs to. "+cmdhelp.ClearFieldHelpText)

		c.Flags.StringVar(&c.vlanName, "vlan", "", "name of the vlan to assign this host to")
		c.Flags.StringVar(&c.nicName, "nic", "", "name of the nic to associate the ip to")
		c.Flags.BoolVar(&c.deleteVlan, "delete-vlan", false, "if deleting the ip assignment for the host")
		c.Flags.StringVar(&c.ip, "ip", "", "the ip to assign the host to")
		c.Flags.StringVar(&c.switchName, "switch", "", "the name of the switch that this device is connected to. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.switchPort, "switch-port", "", "the port of the switch that this device is connected to. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.usbHub, "usb-hub", "", cmdhelp.UsbHubHelpText)
		c.Flags.StringVar(&c.biosVersion, "bios", "", "the bios version of the machine. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.kernelVersion, "kernel", "", "the kernel version of the machine. "+cmdhelp.ClearFieldHelpText)
		c.Flags.StringVar(&c.state, "state", "", cmdhelp.StateHelp)

		return c
	},
}

type updateHost struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	newSpecsFile string
	interactive  bool

	machineName      string
	hostName         string
	vlanName         string
	nicName          string
	deleteVlan       bool
	ip               string
	switchName       string
	switchPort       string
	state            string
	prototype        string
	osVersion        string
	osImage          string
	vmCapacity       int
	usbHub           string
	biosVersion      string
	kernelVersion    string
	tags             []string
	description      string
	deploymentTicket string
	vdc              string
}

func (c *updateHost) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateHost) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
	machinelse := &ufspb.MachineLSE{}
	if c.interactive {
		return errors.New("Interactive mode for this " +
			"command is not yet implemented yet. Use JSON input mode.")
		//TODO(eshwarn): add interactive input
		//utils.GetMachinelseInteractiveInput(ctx, ic, &machinelse, true)
	}
	if c.newSpecsFile != "" {
		if err = utils.ParseJSONFile(c.newSpecsFile, machinelse); err != nil {
			return err
		}
		if machinelse.GetMachines() == nil || len(machinelse.GetMachines()) <= 0 {
			return errors.New("machines field is empty in json. It is a required parameter for json input.")
		}
	}
	oldMachinelse, err := utils.PrintExistingHost(ctx, ic, c.hostName)
	if err != nil {
		return err
	}
	if c.newSpecsFile == "" {
		c.parseArgs(oldMachinelse, machinelse)
	}

	var networkOptions map[string]*ufsAPI.NetworkOption
	if c.deleteVlan || c.vlanName != "" || c.ip != "" || c.nicName != "" {
		networkOptions = map[string]*ufsAPI.NetworkOption{
			machinelse.Name: {
				Delete: c.deleteVlan,
				Vlan:   c.vlanName,
				Nic:    c.nicName,
				Ip:     c.ip,
			},
		}
	}

	machinelse.Name = ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, machinelse.Name)
	if !ufsUtil.ValidateTags(machinelse.Tags) {
		return fmt.Errorf(ufsAPI.InvalidTags)
	}
	paths := map[string]string{
		"machine":     ufsUtil.MachinesPath,
		"prototype":   ufsUtil.MachineLsePrototypePath,
		"vm-capacity": ufsUtil.ChromeBrowserMachineLseVmCapacityPath,
		"tag":         ufsUtil.TagsPath,
		"state":       ufsUtil.ResourceStatePath,
		"desc":        ufsUtil.DescriptionPath,
		"ticket":      ufsUtil.DeploymentTicketPath,
		"vdc":         ufsUtil.ChromeBrowserMachineLseVirtualDatacenterPath,
		"switch":      ufsUtil.AndroidHostLseSwitchInterfaceSwitchPath,
		"switch-port": ufsUtil.AndroidHostLseSwitchInterfacePortNamePath,
		"usb-hub":     ufsUtil.AndroidHostLseUsbHubPath,
		"bios":        ufsUtil.AndroidHostLseBiosVersionPath,
		"kernel":      ufsUtil.AndroidHostLseKernelVersionPath,
	}
	if oldMachinelse.GetChromeBrowserMachineLse() != nil {
		paths["os"] = ufsUtil.ChromeBrowserMachineLseOsVersionValuePath
		paths["os-image"] = ufsUtil.ChromeBrowserMachineLseOsVersionImagePath
	} else if oldMachinelse.GetAndroidHostLse() != nil {
		paths["os"] = ufsUtil.AndroidHostLseOsVersionValuePath
		paths["os-image"] = ufsUtil.AndroidHostLseOsVersionImagePath
	}
	res, err := ic.UpdateMachineLSE(ctx, &ufsAPI.UpdateMachineLSERequest{
		MachineLSE:     machinelse,
		NetworkOptions: networkOptions,
		UpdateMask:     utils.GetUpdateMask(&c.Flags, paths),
	})
	if err != nil {
		return err
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	c.printRes(ctx, ic, res)
	return nil
}

func (c *updateHost) printRes(ctx context.Context, ic ufsAPI.FleetClient, res *ufspb.MachineLSE) {
	fmt.Println("The host after update:")
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	if c.deleteVlan {
		fmt.Printf("Successfully deleted vlan & ip of host %s\nPlease run `shivas get host -full %s` to further check\n", res.Name, res.Name)
	}
	if c.vlanName != "" || c.ip != "" {
		// Log the assigned IP
		if dhcp, err := ic.GetDHCPConfig(ctx, &ufsAPI.GetDHCPConfigRequest{
			Hostname: res.Name,
		}); err == nil {
			fmt.Println("Newly added DHCP config:")
			utils.PrintProtoJSON(dhcp, false)
			fmt.Printf("Successfully added dhcp config %s to host %s\nPlease run `shivas get host -full %s` to further check\n", dhcp.GetIp(), res.Name, res.Name)
		}
	}
}

func (c *updateHost) parseArgs(oldLse, lse *ufspb.MachineLSE) {
	lse.Name = c.hostName
	lse.Hostname = c.hostName
	lse.MachineLsePrototype = c.prototype
	lse.ResourceState = ufsUtil.ToUFSState(c.state)
	lse.Machines = []string{c.machineName}
	if ufsUtil.ContainsAnyStrings(c.tags, utils.ClearFieldValue) {
		lse.Tags = nil
	} else {
		lse.Tags = c.tags
	}
	if c.deploymentTicket == utils.ClearFieldValue {
		lse.DeploymentTicket = ""
	} else {
		lse.DeploymentTicket = c.deploymentTicket
	}
	if c.description == utils.ClearFieldValue {
		lse.Description = ""
	} else {
		lse.Description = c.description
	}
	osVersion := &ufspb.OSVersion{}
	if c.osVersion == utils.ClearFieldValue {
		osVersion.Value = ""
	} else if c.osVersion != "" {
		osVersion.Value = c.osVersion
	}
	if c.osImage == utils.ClearFieldValue {
		osVersion.Image = ""
	} else if c.osImage != "" {
		osVersion.Image = c.osImage
	}
	if oldLse.GetChromeBrowserMachineLse() != nil {
		lse.Lse = &ufspb.MachineLSE_ChromeBrowserMachineLse{
			ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
				OsVersion: osVersion,
			},
		}
		if c.vmCapacity == -1 {
			lse.GetChromeBrowserMachineLse().VmCapacity = 0
		} else if c.vmCapacity != 0 {
			lse.GetChromeBrowserMachineLse().VmCapacity = int32(c.vmCapacity)
		}
		if c.vdc == utils.ClearFieldValue {
			lse.GetChromeBrowserMachineLse().VirtualDatacenter = ""
		} else if c.vdc != "" {
			lse.GetChromeBrowserMachineLse().VirtualDatacenter = c.vdc
		}
	} else if oldLse.GetAndroidHostLse() != nil {
		lse.Lse = &ufspb.MachineLSE_AndroidHostLse{
			AndroidHostLse: &ufspb.AndroidHostLSE{
				OsVersion:       osVersion,
				SwitchInterface: &ufspb.SwitchInterface{},
				UsbHub:          ufsUtil.ToUsbHub(c.usbHub),
				BiosVersion:     c.biosVersion,
				KernelVersion:   c.kernelVersion,
			},
		}
		if c.switchName == utils.ClearFieldValue {
			lse.GetAndroidHostLse().GetSwitchInterface().Switch = ""
		} else if c.switchName != "" {
			lse.GetAndroidHostLse().GetSwitchInterface().Switch = c.switchName
		}
		if c.switchPort == utils.ClearFieldValue {
			lse.GetAndroidHostLse().GetSwitchInterface().PortName = ""
		} else if c.switchPort != "" {
			lse.GetAndroidHostLse().GetSwitchInterface().PortName = c.switchPort
		}
		if c.biosVersion == utils.ClearFieldValue {
			lse.GetAndroidHostLse().BiosVersion = ""
		} else if c.biosVersion != "" {
			lse.GetAndroidHostLse().BiosVersion = c.biosVersion
		}
		if c.kernelVersion == utils.ClearFieldValue {
			lse.GetAndroidHostLse().KernelVersion = ""
		} else if c.kernelVersion != "" {
			lse.GetAndroidHostLse().KernelVersion = c.kernelVersion
		}
	}
}

func (c *updateHost) parseNetworkOpt(lseName string) map[string]*ufsAPI.NetworkOption {
	var networkOptions map[string]*ufsAPI.NetworkOption
	if c.deleteVlan || c.vlanName != "" || c.ip != "" {
		fmt.Println("Setting network option parameters")
		networkOptions = map[string]*ufsAPI.NetworkOption{
			lseName: {
				Delete: c.deleteVlan,
				Vlan:   c.vlanName,
				Nic:    c.nicName,
				Ip:     c.ip,
			},
		}
	}
	return networkOptions
}

func (c *updateHost) validateArgs() error {
	if c.newSpecsFile != "" && c.interactive {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive & JSON mode cannot be specified at the same time.")
	}
	if c.newSpecsFile != "" || c.interactive {
		if c.hostName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-name' cannot be specified at the same time.")
		}
		if c.machineName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-machine' cannot be specified at the same time.")
		}
		if c.prototype != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-prototype' cannot be specified at the same time.")
		}
		if len(c.tags) > 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-tag' cannot be specified at the same time.")
		}
		if c.vmCapacity != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe interactive/JSON mode is specified. '-vm-capacity' cannot be specified at the same time.")
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
		if c.vdc != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-vdc' cannot be specified at the same time.")
		}
		if c.switchName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-switch' cannot be specified at the same time.")
		}
		if c.switchPort != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-switch-port' cannot be specified at the same time.")
		}
		if c.usbHub != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-usb-hub' cannot be specified at the same time.")
		}
		if c.biosVersion != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-bios' cannot be specified at the same time.")
		}
		if c.kernelVersion != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-kernel' cannot be specified at the same time.")
		}
	}
	if c.newSpecsFile == "" && !c.interactive {
		if c.hostName == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required, no mode ('-f' or '-i') is specified.")
		}
		if c.nicName == "" && c.vlanName == "" && !c.deleteVlan && c.ip == "" &&
			c.state == "" && c.deploymentTicket == "" && c.osVersion == "" &&
			c.prototype == "" && c.tags == nil && c.vmCapacity == 0 &&
			c.description == "" && c.machineName == "" && c.osImage == "" &&
			c.vdc == "" && c.switchName == "" && c.switchPort == "" &&
			c.biosVersion == "" && c.kernelVersion == "" && c.usbHub == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nNothing to update. Please provide any field to update")
		}
		if c.state != "" && !ufsUtil.IsUFSState(ufsUtil.RemoveStatePrefix(c.state)) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid state, please check help info for '-state'.", c.state)
		}
		if c.usbHub != "" && !ufsUtil.IsUsbHub(c.usbHub) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid USB hub, please check help info for '-usb-hub'.", c.usbHub)
		}
	}
	return nil
}
