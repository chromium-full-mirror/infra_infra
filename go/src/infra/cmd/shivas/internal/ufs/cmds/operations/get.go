// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package operations

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/asset"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/attacheddevicehost"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/attacheddevicemachine"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/cachingservice"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/chromeplatform"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/defaultwifi"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/devboard"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/devicelabels"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/drac"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/dut"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/host"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/kvm"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/lsedeployment"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/machine"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/machineprototype"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/nic"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/ownership"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/peripherals"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rack"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rackprototype"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rpm"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/schedulingunit"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/stableversion"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/static"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/switches"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/vlan"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/vm"
)

type get struct {
	subcommands.CommandRunBase
}

// GetCmd contains get command specification
var GetCmd = &subcommands.Command{
	UsageLine: "get <sub-command>",
	ShortDesc: "Get details of a resource/entity",
	LongDesc: `Get details for
	machine/rack/kvm/rpm/switch/drac/nic
	host/vm/vm-slots
	asset/dut/cachingservice/schedulingunit
	machine-prototype/rack-prototype/platform/vlan/host-deployment
	attached-device-machine (aliased as adm/attached-device-machine)
	attached-device-host (aliased as adh/attached-device-host)
	defaultwifi`,
	CommandRun: func() subcommands.CommandRun {
		c := &get{}
		return c
	},
}

type getApp struct {
	cli.Application
}

// Run implementing subcommands.CommandRun interface
func (c *get) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&getApp{*d}, args)
}

// GetCommands lists all the subcommands under get
// Aliases:
//
//	attacheddevicemachine.GetAttachedDeviceMachineCmd = attacheddevicemachine.GetADMCmd
//	attacheddevicehost.GetAttachedDeviceHostCmd = attacheddevicehost.GetADHCmd
func (c getApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		asset.GetAssetCmd,
		dut.GetDutCmd,
		defaultwifi.GetDefaultWifiCmd,
		devboard.GetDevboardMachineCmd,
		devboard.GetDevboardLSECmd,
		cachingservice.GetCachingServiceCmd,
		schedulingunit.GetSchedulingUnitCmd,
		machine.GetMachineCmd,
		attacheddevicemachine.GetAttachedDeviceMachineCmd,
		attacheddevicemachine.GetADMCmd,
		host.GetHostCmd,
		attacheddevicehost.GetAttachedDeviceHostCmd,
		attacheddevicehost.GetADHCmd,
		kvm.GetKVMCmd,
		rpm.GetRPMCmd,
		switches.GetSwitchCmd,
		drac.GetDracCmd,
		nic.GetNicCmd,
		vm.GetVMCmd,
		vm.ListVMSlotCmd,
		rack.GetRackCmd,
		machineprototype.GetMachineLSEPrototypeCmd,
		rackprototype.GetRackLSEPrototypeCmd,
		chromeplatform.GetChromePlatformCmd,
		vlan.GetVlanCmd,
		stableversion.GetStableVersionCmd,
		static.GetStatesCmd,
		static.GetZonesCmd,
		lsedeployment.GetMachineLSEDeploymentCmd,
		ownership.GetOwnershipDataCmd,
		peripherals.GetPasitHostCmd,
		devicelabels.GetDeviceLabelsCmd,
	}
}

// GetName is cli.Application interface implementation
func (c getApp) GetName() string {
	return "get"
}
