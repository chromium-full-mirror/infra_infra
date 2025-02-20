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

type delete struct {
	subcommands.CommandRunBase
}

// DeleteCmd contains delete command specification
var DeleteCmd = &subcommands.Command{
	UsageLine: "delete <sub-command>",
	ShortDesc: "Delete a resource/entity",
	LongDesc: `Delete a
	machine/rack/kvm/rpm/switch/drac/nic
	host/vm
	asset/dut/cachingservice/schedulingunit
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
		c := &delete{}
		return c
	},
}

type deleteApp struct {
	cli.Application
}

// Run implementing subcommands.CommandRun interface
func (c *delete) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&deleteApp{*d}, args)
}

// GetCommands lists all the subcommands under delete
//
// Aliases:
//
//	attacheddevicemachine.DeleteAttachedDeviceMachineCmd = attacheddevicemachine.DeleteADMCmd
//	attacheddevicehost.DeleteAttachedDeviceHostCmd = attacheddevicehost.DeleteADHCmd
func (c deleteApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		asset.DeleteAssetCmd,
		dut.DeleteDUTCmd,
		schedulingunit.DeleteSchedulingUnitCmd,
		machine.DeleteMachineCmd,
		attacheddevicemachine.DeleteAttachedDeviceMachineCmd,
		attacheddevicemachine.DeleteADMCmd,
		devboard.DeleteDevboardMachineCmd,
		host.DeleteHostCmd,
		attacheddevicehost.DeleteAttachedDeviceHostCmd,
		attacheddevicehost.DeleteADHCmd,
		defaultwifi.DeleteDefaultWifiCmd,
		devboard.DeleteDevboardLSECmd,
		kvm.DeleteKVMCmd,
		rpm.DeleteRPMCmd,
		switches.DeleteSwitchCmd,
		drac.DeleteDracCmd,
		nic.DeleteNicCmd,
		vm.DeleteVMCmd,
		rack.DeleteRackCmd,
		machineprototype.DeleteMachineLSEPrototypeCmd,
		rackprototype.DeleteRackLSEPrototypeCmd,
		chromeplatform.DeleteChromePlatformCmd,
		cachingservice.DeleteCachingServiceCmd,
		vlan.DeleteVlanCmd,
		peripherals.DeleteBluetoothPeersCmd,
		peripherals.DeletePeripheralHMRCmd,
		peripherals.DeletePeripheralWifiCmd,
		peripherals.DeleteChameleonCmd,
		peripherals.DeletePeripheralAudioLatencyToolkitCmd,
		peripherals.DeletePeripheralALTCmd,
		peripherals.DeletePasitHostCmd,
		peripherals.DeletePeripheralAMTCmd,
	}
}

// GetName is cli.Application interface implementation
func (c deleteApp) GetName() string {
	return "delete"
}
