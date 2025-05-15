// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/maruel/subcommands"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/libs/fleet/device/attacheddevice"
	"go.chromium.org/infra/libs/fleet/device/dut"
	"go.chromium.org/infra/libs/fleet/device/schedulingunit"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// SyncDUTInfo subcommand: One round data sync between bot and UFS.
var SyncDUTInfo = &subcommands.Command{
	UsageLine: "internal-sync-dut-info",
	ShortDesc: "sync DUT info between the client and UFS",
	LongDesc: `Sync DUT info between the client and UFS.

	For internal use only.
	The client can upload/download specific data to/from UFS. Either direction is
  optional. If both direction are specified, we do upload first, then download.`,
	CommandRun: func() subcommands.CommandRun {
		c := &syncDUTInfoRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.BoolVar(&c.byHostname, "by-hostname", false, "Lookup by hostname instead of ID/Asset tag.")

		c.Flags.StringVar(&c.uploadHealthStatus, "upload-health-status", "", "Upload the health status of the DUT to UFS.")
		c.Flags.BoolVar(&c.downloadDUTInfo, "download-dut-info", false, "Download DUT info from UFS.")

		return c
	},
}

type syncDUTInfoRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	byHostname bool

	uploadHealthStatus string
	downloadDUTInfo    bool

	ufs ufsAPI.FleetClient
}

func (c *syncDUTInfoRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *syncDUTInfoRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 1 {
		return cmdlib.NewUsageError(c.Flags, "exactly one DUT hostname must be provided")
	}
	if h := c.uploadHealthStatus; h != "" && h != "healthy" && h != "unhealthy" {
		return cmdlib.NewUsageError(c.Flags, "health status can only be [healthy|unhealthy]")
	}

	ctx := cli.GetContext(a, c, env)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()

	if c.commonFlags.Verbose() {
		fmt.Printf("Using UnifiedFleet service %s (namespace %s)\n", e.UnifiedFleetService, c.getNamespace())
	}
	ctx = utils.SetupContext(ctx, c.getNamespace())

	c.ufs = ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	if err := c.uploadHealth(ctx, a, args[0]); err != nil {
		return err
	}

	if err := c.downloadInfo(ctx, a, args[0]); err != nil {
		return err
	}
	return nil
}

func (c *syncDUTInfoRun) uploadHealth(ctx context.Context, a subcommands.Application, dutID string) error {
	if c.uploadHealthStatus == "" {
		return nil
	}
	state := dutstate.Ready
	if c.uploadHealthStatus == "unhealthy" {
		state = dutstate.NeedsManualRepair
	}
	if err := dutstate.Update(ctx, c.ufs, dutID, state); err != nil {
		return errors.Annotate(err, "upload health status %q of %q to UFS", c.uploadHealthStatus, dutID).Err()
	}
	return nil
}

// downloadInfo downloads DUT info from UFS and prints out.
func (c *syncDUTInfoRun) downloadInfo(ctx context.Context, a subcommands.Application, dutID string) error {
	if !c.downloadDUTInfo {
		return nil
	}
	stderr := a.GetErr()
	r := func(err error) { fmt.Fprintf(stderr, "sanitize dimensions: %s\n", err) }
	var bi *botInfo
	var err error

	if ns := c.getNamespace(); ns == ufsUtil.BrowserNamespace {
		if bi, err = getBrowserBotInfo(ctx, c.ufs, dutID); err != nil {
			return fmt.Errorf("download info (%q): %w", ns, err)
		}
	} else {
		if bi, err = getOSBotInfo(ctx, c.ufs, dutID, c.byHostname, r); err != nil {
			return fmt.Errorf("download info (%q): %w", ns, err)
		}
	}

	// Post-processing
	enc, err := json.Marshal(bi)
	if err != nil {
		return err
	}
	a.GetOut().Write(enc)
	return nil
}

type botInfo struct {
	Dimensions swarming.Dimensions
	State      botState
}

type botState map[string][]string

// getNamespace returns the namespace we will be using to query UFS given user
// input. It is guaranteed to be a valid namespace (so we can make assumptions
// downstream using that fact).
// Note that this function specifically swallows invalid input and sets as `os`
func (c *syncDUTInfoRun) getNamespace() string {
	ns, err := c.envFlags.Namespace(nil, "")
	if err != nil {
		// Set namespace to OS namespace for whatever errors.
		ns = ufsUtil.OSNamespace
	}
	return ns
}

func getBrowserBotInfo(ctx context.Context, client ufsAPI.FleetClient, id string) (*botInfo, error) {
	// id is the hostname by default for browser bots
	resp, err := getDeviceData(ctx, client, id, true)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("no browser device data for host %q", id)
		}
		return nil, err
	}
	var state, zone, maintenanceConfig string
	if resp.GetBrowserDeviceData().GetHost() != nil {
		state = dutstate.ConvertFromUFSState(resp.GetBrowserDeviceData().GetHost().GetResourceState()).String()
		zone = resp.GetBrowserDeviceData().GetHost().GetZone()
		maintenanceConfig = resp.GetBrowserDeviceData().GetHost().GetMaintenanceConfigName()
	} else {
		state = dutstate.ConvertFromUFSState(resp.GetBrowserDeviceData().GetVm().GetResourceState()).String()
		zone = resp.GetBrowserDeviceData().GetVm().GetZone()
	}
	dims := map[string][]string{
		"ufs_state": {state},
		// Duplicate state to dut_state to reuse analytics logic built for ChromeOS lab
		"dut_state": {state},
		"ufs_zone":  {zone},
	}
	if maintenanceConfig != "" {
		dims["maintenance_config"] = []string{maintenanceConfig}
	}
	return &botInfo{Dimensions: dims}, nil
}

func getOSBotInfo(ctx context.Context, client ufsAPI.FleetClient, id string, byHostname bool, r swarming.ReportFunc) (*botInfo, error) {
	resp, err := getDeviceData(ctx, client, id, byHostname)
	if err != nil {
		return nil, err
	}
	if resp.GetResourceType() == ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT {
		return getSUBotInfo(ctx, client, resp.GetSchedulingUnit(), r)
	}
	botDimensions, err := getBotDimensions(ctx, client, resp, r)
	if err != nil {
		return nil, err
	}
	botState, err := getBotState(resp)
	if err != nil {
		return nil, err
	}
	return &botInfo{
		Dimensions: botDimensions,
		State:      botState,
	}, nil
}

func getSUBotInfo(ctx context.Context, client ufsAPI.FleetClient, su *ufspb.SchedulingUnit, r swarming.ReportFunc) (*botInfo, error) {
	var dutsDims []swarming.Dimensions
	for _, hostname := range su.GetMachineLSEs() {
		resp, err := getDeviceData(ctx, client, hostname, true)
		if err != nil {
			return nil, err
		}
		botDimensions, err := getBotDimensions(ctx, client, resp, r)
		if err != nil {
			return nil, err
		}
		dutsDims = append(dutsDims, botDimensions)
	}
	return &botInfo{
		Dimensions: schedulingunit.GetSchedulingUnitDimensions(su, dutsDims),
		State:      schedulingunit.GetSchedulingUnitBotState(su),
	}, nil
}

func getDeviceData(ctx context.Context, client ufsAPI.FleetClient, id string, byHostname bool) (*ufsAPI.GetDeviceDataResponse, error) {
	req := &ufsAPI.GetDeviceDataRequest{}
	if byHostname {
		req.Hostname = id
	} else {
		req.DeviceId = id
	}
	return client.GetDeviceData(ctx, req)
}

func getBotState(deviceData *ufsAPI.GetDeviceDataResponse) (botState, error) {
	switch deviceData.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		return getDUTBotState(deviceData.GetChromeOsDeviceData()), nil
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		return getAttachedDeviceBotState(deviceData.GetAttachedDeviceData()), nil
	}
	return nil, fmt.Errorf("get bot state: invalid device type (%s)", deviceData.GetResourceType())
}

func getDUTBotState(deviceData *ufspb.ChromeOSDeviceData) botState {
	d := deviceData.GetDutV1()
	s := make(botState)
	for _, kv := range d.GetCommon().GetAttributes() {
		k, v := kv.GetKey(), kv.GetValue()
		s[k] = append(s[k], v)
	}
	s["storage_state"] = []string{d.GetCommon().GetLabels().GetPeripherals().GetStorageState().String()[len("HARDWARE_"):]}
	s["servo_usb_state"] = []string{d.GetCommon().GetLabels().GetPeripherals().GetServoUsbState().String()[len("HARDWARE_"):]}
	s["battery_state"] = []string{d.GetCommon().GetLabels().GetPeripherals().GetBatteryState().String()[len("HARDWARE_"):]}
	s["wifi_state"] = []string{d.GetCommon().GetLabels().GetPeripherals().GetWifiState().String()[len("HARDWARE_"):]}
	s["bluetooth_state"] = []string{d.GetCommon().GetLabels().GetPeripherals().GetBluetoothState().String()[len("HARDWARE_"):]}
	s["rpm_state"] = []string{d.GetCommon().GetLabels().GetPeripherals().GetRpmState().String()}
	s["lab_config_version_index"] = []string{deviceData.GetLabConfig().GetUpdateTime().AsTime().Format(ufsUtil.TimestampBasedVersionKeyFormat)}
	s["dut_state_version_index"] = []string{deviceData.GetDutState().GetUpdateTime().AsTime().Format(ufsUtil.TimestampBasedVersionKeyFormat)}
	s["dut_state_reason"] = []string{deviceData.GetDutState().GetDutStateReason()}
	return s
}

func getAttachedDeviceBotState(deviceData *ufsAPI.AttachedDeviceData) botState {
	s := make(botState)
	s["lab_config_version_index"] = []string{deviceData.GetLabConfig().GetUpdateTime().AsTime().Format(ufsUtil.TimestampBasedVersionKeyFormat)}
	s["dut_state_version_index"] = []string{deviceData.GetDutState().GetUpdateTime().AsTime().Format(ufsUtil.TimestampBasedVersionKeyFormat)}
	return s
}

func getBotDimensions(ctx context.Context, client ufsAPI.FleetClient, deviceData *ufsAPI.GetDeviceDataResponse, r swarming.ReportFunc) (swarming.Dimensions, error) {
	switch deviceData.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		dutState := dutstate.Read(ctx, client, deviceData.GetChromeOsDeviceData().GetLabConfig().GetName())
		return dut.GetDUTBotDims(ctx, r, dutState, deviceData.GetChromeOsDeviceData()), nil
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		dutState := dutstate.Read(ctx, client, deviceData.GetAttachedDeviceData().GetLabConfig().GetName())
		return attacheddevice.GetAttachedDeviceBotDims(ctx, r, dutState, deviceData.GetAttachedDeviceData()), nil
	}
	return nil, fmt.Errorf("append bot dimensions: invalid device type (%s)", deviceData.GetResourceType())
}
