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
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/drac"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/dut"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/host"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/kvm"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/labstation"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/machine"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/machineprototype"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/nic"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/peripherals"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rack"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rackprototype"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rpm"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/schedulingunit"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/switches"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/vlan"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/vm"
)

type add struct {
	subcommands.CommandRunBase
}

// AddCmd contains add command specification
var AddCmd = &subcommands.Command{
	UsageLine: "add <sub-command>",
	ShortDesc: "Add details of a resource/entity",
	LongDesc: `Add details for
	machine/rack/kvm/rpm/switch/drac/nic
	host/vm
	asset/dut/labstation/cachingservice/schedulingunit
	machine-prototype/rack-prototype/chromeplatform/vlan
	attached-device-machine (aliased as adm/attached-device-machine)
	attached-device-host (aliased as adh/attached-device-host)
	defaultwifi
	peripheral-hmr
	peripheral-wifi
	bluetooth-peers
	peripheral-pasit-host
	peripheral-audio-latency-toolkit
	peripheral-amt`,
	CommandRun: func() subcommands.CommandRun {
		c := &add{}
		return c
	},
}

type addApp struct {
	cli.Application
}

// Run implementing subcommands.CommandRun interface
func (c *add) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&addApp{*d}, args)
}

// GetCommands lists all the subcommands under add
// Aliases:
//
//	attacheddevicemachine.AddAttachedDeviceMachineCmd = attacheddevicemachine.AddADMCmd
//	attacheddevicehost.AddAttachedDeviceHostCmd = attacheddevicehost.AddADHCmd
func (c addApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		asset.AddAssetCmd,
		dut.AddDUTCmd,
		defaultwifi.AddDefaultWifiCmd,
		devboard.AddDevboardLSECmd,
		devboard.AddDevboardMachineCmd,
		labstation.AddLabstationCmd,
		cachingservice.AddCachingServiceCmd,
		schedulingunit.AddSchedulingUnitCmd,
		machine.AddMachineCmd,
		attacheddevicemachine.AddAttachedDeviceMachineCmd,
		attacheddevicemachine.AddADMCmd,
		host.AddHostCmd,
		attacheddevicehost.AddAttachedDeviceHostCmd,
		attacheddevicehost.AddADHCmd,
		kvm.AddKVMCmd,
		rpm.AddRPMCmd,
		switches.AddSwitchCmd,
		drac.AddDracCmd,
		nic.AddNicCmd,
		vm.AddVMCmd,
		rack.AddRackCmd,
		machineprototype.AddMachineLSEPrototypeCmd,
		rackprototype.AddRackLSEPrototypeCmd,
		chromeplatform.AddChromePlatformCmd,
		vlan.AddVlanCmd,
		peripherals.AddBluetoothPeersCmd,
		peripherals.AddPeripheralHMRCmd,
		peripherals.AddPeripheralWifiCmd,
		peripherals.AddChameleonCmd,
		peripherals.AddPeripheralAudioLatencyToolkitCmd,
		peripherals.AddPeripheralALTCmd,
		peripherals.AddPasitHostCmd,
		peripherals.AddPeripheralAMTCmd,
	}
}

// GetName is cli.Application interface implementation
func (c addApp) GetName() string {
	return "add"
}
