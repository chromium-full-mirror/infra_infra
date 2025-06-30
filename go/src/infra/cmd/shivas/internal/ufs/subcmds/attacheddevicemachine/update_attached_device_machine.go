// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package attacheddevicemachine

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
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

// UpdateAttachedDeviceMachineCmd updates the attached device machine for a given name.
var UpdateAttachedDeviceMachineCmd = &subcommands.Command{
	UsageLine:  "attached-device-machine ...",
	ShortDesc:  "Update attached device machine details by filters",
	LongDesc:   cmdhelp.UpdateADMText,
	CommandRun: updateADMCommandRun,
}

// UpdateADMCmd is an alias to UpdateAttachedDeviceMachineCmd
var UpdateADMCmd = &subcommands.Command{
	UsageLine:  "adm ...",
	ShortDesc:  "Update attached device machine details by filters",
	LongDesc:   cmdhelp.UpdateADMText,
	CommandRun: updateADMCommandRun,
}

func updateADMCommandRun() subcommands.CommandRun {
	c := &updateAttachedDeviceMachine{}
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.commonFlags.Register(&c.Flags)

	c.Flags.StringVar(&c.newSpecsFile, "f", "", cmdhelp.ADMFileText)

	c.Flags.StringVar(&c.machineName, "name", "", "The name of the attached device machine to add.")
	c.Flags.StringVar(&c.zoneName, "zone", "", cmdhelp.ZoneHelpText)
	c.Flags.StringVar(&c.rackName, "rack", "", "The rack to add the attached device machine to. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.serialNumber, "serial", "", "The serial number for this attached device machine.")
	c.Flags.StringVar(&c.manufacturer, "man", "", "The manufacturer for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.deviceType, "devicetype", "", "The device type for this attached device machine. "+cmdhelp.AttachedDeviceTypeHelpText)
	c.Flags.StringVar(&c.buildTarget, "build-target", "", "The build target for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.model, "model", "", "The model for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.phase, "phase", "", "The phase for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.revision, "revision", "", "The revision for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.chipId, "chip-id", "", "The chip id for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.imei1, "imei1", "", "IMEI 1 for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.imei2, "imei2", "", "IMEI 2 for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.batteryStatus, "battery-status", "", "The battery status for this attached device machine."+cmdhelp.BatteryStatusHelpText)
	c.Flags.StringVar(&c.storageMan, "storage-man", "", "The storage manufacturer for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.storageCap, "storage-cap", "", "The storage capacity for this attached device machine in bytes assigned. "+cmdhelp.ByteUnitsAcceptedText+" "+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(flag.StringSlice(&c.simTypes), "sim-type", "Sim type(s) for this attached device machine. Can be specified multiple times."+cmdhelp.SimTypeHelpText)
	c.Flags.StringVar(&c.simEid, "eid", "", "The sim EID for this attached device machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.state, "state", "", cmdhelp.StateHelp)
	c.Flags.Var(flag.StringSlice(&c.tags), "tag", "Name(s) of tag(s). Can be specified multiple times. "+cmdhelp.ClearFieldHelpText)
	return c
}

type updateAttachedDeviceMachine struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	newSpecsFile string

	machineName   string
	zoneName      string
	rackName      string
	tags          []string
	serialNumber  string
	manufacturer  string
	deviceType    string
	buildTarget   string
	model         string
	phase         string
	revision      string
	chipId        string
	imei1         string
	imei2         string
	batteryStatus string
	storageMan    string
	storageCap    string
	simTypes      []string
	simEid        string
	state         string
}

func (c *updateAttachedDeviceMachine) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateAttachedDeviceMachine) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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

	var machine ufspb.Machine
	if c.newSpecsFile != "" {
		if err = utils.ParseJSONFile(c.newSpecsFile, &machine); err != nil {
			return err
		}
		machine.Realm = ufsUtil.ToUFSRealm(machine.GetLocation().GetZone().String())
	} else {
		c.parseArgs(&machine)
	}
	_, err = utils.PrintExistingAttachedDeviceMachine(ctx, ic, machine.Name)
	if err != nil {
		return err
	}

	machine.Name = ufsUtil.AddPrefix(ufsUtil.MachineCollection, machine.Name)
	if !ufsUtil.ValidateTags(machine.Tags) {
		return fmt.Errorf(ufsAPI.InvalidTags)
	}
	res, err := ic.UpdateMachine(ctx, &ufsAPI.UpdateMachineRequest{
		Machine: &machine,
		UpdateMask: utils.GetUpdateMask(&c.Flags, map[string]string{
			"zone":           ufsUtil.ZonePath,
			"rack":           ufsUtil.LocationRackPath,
			"tag":            ufsUtil.TagsPath,
			"serial":         ufsUtil.SerialNumberPath,
			"state":          ufsUtil.ResourceStatePath,
			"man":            ufsUtil.AttachedDeviceManufacturerPath,
			"devicetype":     ufsUtil.AttachedDeviceDeviceTypePath,
			"build-target":   ufsUtil.AttachedDeviceBuildTargetPath,
			"model":          ufsUtil.AttachedDeviceModelPath,
			"phase":          ufsUtil.AttachedDevicePhasePath,
			"revision":       ufsUtil.AttachedDeviceRevisionPath,
			"chip-id":        ufsUtil.AttachedDeviceChipIdPath,
			"imei1":          ufsUtil.AttachedDeviceImei1Path,
			"imei2":          ufsUtil.AttachedDeviceImei2Path,
			"battery-status": ufsUtil.AttachedDeviceBatteryStatusPath,
			"storage-man":    ufsUtil.AttachedDeviceStorageManufacturerPath,
			"storage-cap":    ufsUtil.AttachedDeviceStorageCapacityPath,
			"sim-type":       ufsUtil.AttachedDeviceSimTypesPath,
			"eid":            ufsUtil.AttachedDeviceSimEidPath,
		}),
	})
	if err != nil {
		return err
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	fmt.Println("The attached device machine after update:")
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	fmt.Println("Successfully updated the attached device machine: ", res.Name)
	return nil
}

func (c *updateAttachedDeviceMachine) parseArgs(machine *ufspb.Machine) {
	machine.Device = &ufspb.Machine_AttachedDevice{
		AttachedDevice: &ufspb.AttachedDevice{
			Storage: &ufspb.AttachedDevice_Storage{},
			Sim:     &ufspb.AttachedDevice_SIM{},
		},
	}
	machine.Name = c.machineName
	machine.Location = &ufspb.Location{}
	if c.zoneName == utils.ClearFieldValue {
		machine.GetLocation().Zone = ufsUtil.ToUFSZone("")
	} else {
		machine.GetLocation().Zone = ufsUtil.ToUFSZone(c.zoneName)
	}
	if c.rackName == utils.ClearFieldValue {
		machine.GetLocation().Rack = ""
	} else {
		machine.GetLocation().Rack = c.rackName
	}
	if ufsUtil.ContainsAnyStrings(c.tags, utils.ClearFieldValue) {
		machine.Tags = nil
	} else {
		machine.Tags = c.tags
	}
	if c.serialNumber == utils.ClearFieldValue {
		machine.SerialNumber = ""
	} else {
		machine.SerialNumber = c.serialNumber
	}
	machine.ResourceState = ufsUtil.ToUFSState(c.state)
	machine.Realm = ufsUtil.ToUFSRealm(machine.GetLocation().GetZone().String())

	// Attached Device Machine specific masks
	machine.GetAttachedDevice().DeviceType = ufsUtil.ToUFSAttachedDeviceType(c.deviceType)
	machine.GetAttachedDevice().BatteryStatus = ufsUtil.ToBatteryStatus(c.batteryStatus)
	if c.manufacturer == utils.ClearFieldValue {
		machine.GetAttachedDevice().Manufacturer = ""
	} else {
		machine.GetAttachedDevice().Manufacturer = c.manufacturer
	}
	if c.buildTarget == utils.ClearFieldValue {
		machine.GetAttachedDevice().BuildTarget = ""
	} else {
		machine.GetAttachedDevice().BuildTarget = c.buildTarget
	}
	if c.model == utils.ClearFieldValue {
		machine.GetAttachedDevice().Model = ""
	} else {
		machine.GetAttachedDevice().Model = c.model
	}
	if c.phase == utils.ClearFieldValue {
		machine.GetAttachedDevice().Phase = ""
	} else {
		machine.GetAttachedDevice().Phase = c.phase
	}
	if c.revision == utils.ClearFieldValue {
		machine.GetAttachedDevice().Revision = ""
	} else {
		machine.GetAttachedDevice().Revision = c.revision
	}
	if c.chipId == utils.ClearFieldValue {
		machine.GetAttachedDevice().ChipId = ""
	} else {
		machine.GetAttachedDevice().ChipId = c.chipId
	}
	if c.imei1 == utils.ClearFieldValue {
		machine.GetAttachedDevice().Imei1 = ""
	} else {
		machine.GetAttachedDevice().Imei1 = c.imei1
	}
	if c.imei2 == utils.ClearFieldValue {
		machine.GetAttachedDevice().Imei2 = ""
	} else {
		machine.GetAttachedDevice().Imei2 = c.imei2
	}

	// AttachedDevice.Storage
	if c.storageMan == utils.ClearFieldValue {
		machine.GetAttachedDevice().GetStorage().Manufacturer = ""
	} else {
		machine.GetAttachedDevice().GetStorage().Manufacturer = c.storageMan
	}
	if c.storageCap == utils.ClearFieldValue {
		machine.GetAttachedDevice().GetStorage().Capacity = 0
	} else {
		machine.GetAttachedDevice().GetStorage().Capacity, _ = utils.ConvertToBytes(c.storageCap)
	}

	// AttachedDevice.Sim
	if ufsUtil.ContainsAnyStrings(c.simTypes, utils.ClearFieldValue) {
		machine.GetAttachedDevice().GetSim().Types = nil
	} else {
		types := make([]ufspb.AttachedDevice_SIM_SIMType, 0, len(c.simTypes))
		for _, t := range c.simTypes {
			types = append(types, ufsUtil.ToAttachedDeviceSimType(t))
		}
		machine.GetAttachedDevice().GetSim().Types = types
	}
	if c.simEid == utils.ClearFieldValue {
		machine.GetAttachedDevice().GetSim().Eid = ""
	} else {
		machine.GetAttachedDevice().GetSim().Eid = c.simEid
	}
}

func (c *updateAttachedDeviceMachine) validateArgs() error {
	if c.newSpecsFile != "" {
		if c.machineName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-name' cannot be specified at the same time.")
		}
		if c.rackName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-rack' cannot be specified at the same time.")
		}
		if c.zoneName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON input file is already specified. '-zone' cannot be specified at the same time.")
		}
		if c.serialNumber != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-serial' cannot be specified at the same time.")
		}
		if c.manufacturer != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-man' cannot be specified at the same time.")
		}
		if c.deviceType != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-devicetype' cannot be specified at the same time.")
		}
		if c.buildTarget != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-build-target' cannot be specified at the same time.")
		}
		if c.model != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-model' cannot be specified at the same time.")
		}
		if len(c.tags) > 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-tags' cannot be specified at the same time.")
		}
		if c.phase != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-phase' cannot be specified at the same time.")
		}
		if c.revision != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-revision' cannot be specified at the same time.")
		}
		if c.chipId != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-chip-id' cannot be specified at the same time.")
		}
		if c.imei1 != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-imei1' cannot be specified at the same time.")
		}
		if c.imei2 != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-imei2' cannot be specified at the same time.")
		}
		if c.batteryStatus != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-battery-status' cannot be specified at the same time.")
		}
		if c.storageMan != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-storage-man' cannot be specified at the same time.")
		}
		if c.storageCap != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-storage-cap' cannot be specified at the same time.")
		}
		if len(c.simTypes) > 0 {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-sim-type' cannot be specified at the same time.")
		}
		if c.simEid != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe JSON mode is specified. '-sim-eid' cannot be specified at the same time.")
		}
	} else {
		if c.machineName == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required, no mode ('-f') is setup.")
		}

		if c.zoneName == "" && c.manufacturer == "" && c.serialNumber == "" &&
			c.deviceType == "" && c.buildTarget == "" && c.model == "" &&
			c.rackName == "" && c.state == "" && len(c.tags) == 0 &&
			c.phase == "" && c.revision == "" && c.chipId == "" &&
			c.imei1 == "" && c.imei2 == "" && c.batteryStatus == "" &&
			c.storageMan == "" && c.storageCap == "" && len(c.simTypes) == 0 &&
			c.simEid == "" && c.state == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nNothing to update. Please provide any field to update")
		}
		if c.deviceType != "" && !ufsUtil.IsAttachedDeviceType(c.deviceType) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid attached device type, please check help info for '-devicetype'.", c.deviceType)
		}
		if _, err := utils.ConvertToBytes(c.storageCap); err != nil && c.storageCap != utils.ClearFieldValue {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe -storage-cap flag was used incorrectly: %w", err)
		}
		if c.batteryStatus != "" && !ufsUtil.IsBatteryStatus(c.batteryStatus) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid battery status, please check help info for '-battery-status'.", c.batteryStatus)
		}
		if len(c.simTypes) > 0 {
			for _, t := range c.simTypes {
				if !ufsUtil.IsAttachedDeviceSimType(t) {
					return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid sim type, please check help info for '-sim-type'.", t)
				}
			}
		}
		if c.zoneName != "" && !ufsUtil.IsUFSZone(ufsUtil.RemoveZonePrefix(c.zoneName)) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid zone name, please check help info for '-zone'.", c.zoneName)
		}
		if c.state != "" && !ufsUtil.IsUFSState(ufsUtil.RemoveStatePrefix(c.state)) {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n%s is not a valid state, please check help info for '-state'.", c.state)
		}
	}
	return nil
}
