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
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/lsedeployment"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/machine"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/machineprototype"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/nic"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rack"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rackprototype"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/rpm"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/schedulingunit"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/switches"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/vlan"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/vm"
)

type update struct {
	subcommands.CommandRunBase
}

// UpdateCmd contains update command specification
var UpdateCmd = &subcommands.Command{
	UsageLine: "update <sub-command>",
	ShortDesc: "Update details of a resource/entity",
	LongDesc: `Update details for
	machine/rack/kvm/rpm/switch/drac/nic
	host/vm
	asset/dut/dut-batch/dut-state/labstation/cachingservice/schedulingunit
	machine-prototype/rack-prototype/chromeplatform/vlan/host-deployment
	attached-device-machine (aliased as adm/attached-device-machine)
	attached-device-host (aliased as adh/attached-device-host)
	defaultwifi`,
	CommandRun: func() subcommands.CommandRun {
		c := &update{}
		return c
	},
}

type updateApp struct {
	cli.Application
}

// Run implementing subcommands.CommandRun interface
func (c *update) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&updateApp{*d}, args)
}

// GetCommands lists all the subcommands under update
// Aliases:
//
//	attacheddevicemachine.UpdateAttachedDeviceMachineCmd = attacheddevicemachine.UpdateADMCmd
//	attacheddevicehost.UpdateAttachedDeviceHostCmd = attacheddevicehost.UpdateADHCmd
func (c updateApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		asset.UpdateAssetCmd,
		defaultwifi.UpdateDefaultWifiCmd,
		dut.UpdateDUTCmd,
		dut.UpdateDUTBatchCmd,
		dut.UpdateDUTStateCmd,
		labstation.UpdateLabstationCmd,
		cachingservice.UpdateCachingServiceCmd,
		schedulingunit.UpdateSchedulingUnitCmd,
		machine.UpdateMachineCmd,
		attacheddevicemachine.UpdateAttachedDeviceMachineCmd,
		attacheddevicemachine.UpdateADMCmd,
		devboard.UpdateDevboardMachineCmd,
		devboard.UpdateDevboardLSECmd,
		host.UpdateHostCmd,
		attacheddevicehost.UpdateAttachedDeviceHostCmd,
		attacheddevicehost.UpdateADHCmd,
		kvm.UpdateKVMCmd,
		rpm.UpdateRPMCmd,
		switches.UpdateSwitchCmd,
		drac.UpdateDracCmd,
		nic.UpdateNicCmd,
		vm.UpdateVMCmd,
		rack.UpdateRackCmd,
		machineprototype.UpdateMachineLSEPrototypeCmd,
		rackprototype.UpdateRackLSEPrototypeCmd,
		chromeplatform.UpdateChromePlatformCmd,
		vlan.UpdateVlanCmd,
		lsedeployment.UpdateMachineLSEDeploymentCmd,
	}
}

// GetName is cli.Application interface implementation
func (c updateApp) GetName() string {
	return "update"
}
